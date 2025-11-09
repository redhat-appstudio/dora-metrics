// Package processor provides ArgoCD event processing functionality.
// It handles the core business logic for processing ArgoCD application events,
// including deployment tracking, commit validation, and DevLake payload generation.
package processor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redhat-appstudio/dora-metrics/pkg/integrations"
	"github.com/redhat-appstudio/dora-metrics/pkg/logger"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/api"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/github"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/parser"
	"github.com/redhat-appstudio/dora-metrics/pkg/storage"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"k8s.io/apimachinery/pkg/watch"
)

// EventProcessor handles ArgoCD application event processing.
// It coordinates between different components to process events, validate commits,
// track deployments, and generate DevLake payloads.
type EventProcessor struct {
	config          *api.Config
	storage         *storage.RedisClient
	githubClient    github.Client
	parser          api.ApplicationParser
	formatter       *parser.Formatter
	commitProcessor *CommitProcessor
}

// NewEventProcessor creates a new event processor instance.
// It takes configuration, storage client, and GitHub client as dependencies.
func NewEventProcessor(config *api.Config, storage *storage.RedisClient, githubClient github.Client) api.EventHandler {
	return &EventProcessor{
		config:          config,
		storage:         storage,
		githubClient:    githubClient,
		parser:          parser.NewApplicationParser(config),
		formatter:       parser.NewFormatter(githubClient, storage),
		commitProcessor: NewCommitProcessor(githubClient, storage),
	}
}

// HandleEvent processes an ArgoCD application event.
func (ep *EventProcessor) HandleEvent(ctx context.Context, event watch.Event, app *v1alpha1.Application) error {
	// Skip ADDED events
	if event.Type == watch.Added {
		return nil
	}

	// Parse application information
	appInfo, err := ep.parser.ParseApplication(app)
	if err != nil {
		return fmt.Errorf("failed to parse application: %w", err)
	}

	// Check if we should monitor this application
	if !ep.parser.ShouldMonitor(app) {
		return nil
	}

	// Process the event based on type
	switch event.Type {
	case watch.Modified:
		return ep.handleModifiedEvent(ctx, app, appInfo)
	case watch.Deleted:
		return ep.handleDeletedEvent(ctx, app, appInfo)
	default:
		logger.Debugf("Unhandled event type %s for application %s", event.Type, app.Name)
		return nil
	}
}

// handleModifiedEvent processes a MODIFIED event.
func (ep *EventProcessor) handleModifiedEvent(ctx context.Context, app *v1alpha1.Application, appInfo *api.ApplicationInfo) error {
	// Handle applications that are OutOfSync with Missing health - these are failed deployments
	if app.Status.Sync.Status == "OutOfSync" && app.Status.Health.Status == "Missing" {
		logger.Infof("Processing failed deployment for application %s - OutOfSync with Missing health", app.Name)
		return ep.processFailedDeployment(ctx, app, appInfo)
	}

	// Read sync revision directly from the app object
	syncRevision := app.Status.Sync.Revision
	lastHistoryRevision := ""
	if len(app.Status.History) > 0 {
		lastHistoryRevision = app.Status.History[len(app.Status.History)-1].Revision
	}
	logger.Debugf("MODIFIED event received for application %s (sync revision: %s, last history: %s, health: %s)", app.Name, syncRevision, lastHistoryRevision, app.Status.Health.Status)

	// Check if this is a new deployment by comparing sync revision with last ArgoCD history
	if len(app.Status.History) > 0 {
		if lastHistoryRevision == syncRevision {
			logger.Debugf("Sync revision %s matches last history entry %s, this is the deployed revision", syncRevision, lastHistoryRevision)
			// Continue to process this deployed revision
		} else {
			logger.Debugf("Sync revision %s does NOT match last history entry %s, skipping deployment", syncRevision, lastHistoryRevision)
			return nil
		}
	} else {
		logger.Debugf("No history found for application %s, skipping", app.Name)
		return nil
	}

	// This is a new deployment, check if we already processed this commit for this specific application
	processed, err := ep.storage.IsCommitProcessed(ctx, syncRevision, app.Name, appInfo.Cluster)
	if err != nil {
		logger.Warnf("Failed to check if commit was processed: %v", err)
	} else if processed {
		logger.Debugf("Commit %s already processed for application %s in cluster %s, skipping", syncRevision, app.Name, appInfo.Cluster)
		return nil
	}

	// Mark commit as processed for this specific application to prevent duplicate processing
	if err := ep.storage.MarkCommitAsProcessed(ctx, syncRevision, app.Name, appInfo.Cluster); err != nil {
		logger.Errorf("Failed to mark commit as processed: %v", err)
	}

	logger.Infof("Processing new deployment for application %s (revision: %s)", app.Name, syncRevision)
	return ep.processNewDeployment(ctx, app, appInfo)
}

// processFailedDeployment processes a failed deployment (OutOfSync + Missing health).
func (ep *EventProcessor) processFailedDeployment(ctx context.Context, app *v1alpha1.Application, appInfo *api.ApplicationInfo) error {
	logger.Infof("Processing failed deployment for application %s", app.Name)

	// Check if we already processed this failed deployment
	processed, err := ep.storage.IsCommitProcessed(ctx, appInfo.Revision, app.Name, appInfo.Cluster)
	if err != nil {
		logger.Warnf("Failed to check if failed deployment was processed: %v", err)
	} else if processed {
		logger.Infof("Failed deployment %s already processed for application %s in cluster %s, skipping until recovery", appInfo.Revision, app.Name, appInfo.Cluster)
		return nil
	}

	// Mark as processed to avoid duplicate processing until recovery
	if err := ep.storage.MarkCommitAsProcessed(ctx, appInfo.Revision, app.Name, appInfo.Cluster); err != nil {
		logger.Errorf("Failed to mark failed deployment as processed: %v", err)
	}

	// Get commit history for the failed deployment (same as successful deployment)
	commitHistory := ep.commitProcessor.GetCommitHistoryForDeployment(app, appInfo)

	// Use the formatter to create a proper DevLake deployment with real commit data
	deployment, hasCommits := ep.formatter.FormatDeployment(app, appInfo, appInfo.DeployedAt, commitHistory)

	// Only process if we have commits
	if !hasCommits {
		logger.Infof("No commits found for failed deployment %s, skipping DevLake payload", app.Name)
		return nil
	}

	// Override the result to FAILED and update display info with meaningful format
	// The formatter already creates a good DisplayTitle, we just need to update it to show FAILED status
	componentName := appInfo.Component
	if componentName == "" {
		componentName = app.Name
	}
	namespace := appInfo.Namespace
	if namespace == "" {
		namespace = app.Namespace
	}
	
	displayTitle := fmt.Sprintf("ArgoCD Deployment | Component: %s | Namespace: %s | Revision: %s | Status: FAILED | Deployed: %s",
		componentName,
		namespace,
		appInfo.Revision,
		appInfo.DeployedAt.Format("2006-01-02 15:04:05 MST"))
	name := fmt.Sprintf("deploy to production %s", appInfo.Revision)

	deployment.Result = "FAILED"
	deployment.DisplayTitle = &displayTitle
	deployment.Name = &name

	// Update commit results to FAILED
	for i := range deployment.DeploymentCommits {
		deployment.DeploymentCommits[i].Result = "FAILED"
		failedMsg := fmt.Sprintf("Deployment failed for %s - OutOfSync with Missing health", app.Name)
		deployment.DeploymentCommits[i].CommitMsg = failedMsg
		deployment.DeploymentCommits[i].DisplayTitle = &failedMsg
		deployment.DeploymentCommits[i].Name = &failedMsg
	}

	// Log the failed deployment payload
	ep.logDevLakePayload(deployment)

	// Send failed deployment to DevLake
	if err := ep.sendDeploymentToDevLake(ctx, deployment); err != nil {
		logger.Errorf("Failed to send failed deployment to DevLake: %v", err)
		// Don't return error - continue with storing the record
	}

	// Store the failed deployment record
	deploymentRecord := &storage.DeploymentRecord{
		ApplicationName: app.Name,
		Namespace:       appInfo.Namespace,
		ComponentName:   appInfo.Component,
		ClusterName:     appInfo.Cluster,
		Revision:        appInfo.Revision,
		Images:          appInfo.Images,
		DeployedAt:      appInfo.DeployedAt,
		Environment:     appInfo.Environment,
		CommitHistory:   []string{appInfo.Revision},
	}

	if err := ep.storage.StoreDeployment(ctx, deploymentRecord); err != nil {
		logger.Errorf("Failed to store failed deployment record: %v", err)
	}

	logger.Infof("Processed failed deployment for application %s (revision: %s)", app.Name, appInfo.Revision)
	return nil
}

// handleDeletedEvent processes a DELETED event.
func (ep *EventProcessor) handleDeletedEvent(ctx context.Context, app *v1alpha1.Application, appInfo *api.ApplicationInfo) error {
	logger.Infof("Application %s deleted from namespace %s", app.Name, app.Namespace)
	return nil
}

// processNewDeployment processes a new deployment.
func (ep *EventProcessor) processNewDeployment(ctx context.Context, app *v1alpha1.Application, appInfo *api.ApplicationInfo) error {
	logger.Infof("Processing new deployment for application %s (revision: %s)", app.Name, appInfo.Revision)

	// Get commit history for this deployment
	commitHistory := ep.commitProcessor.GetCommitHistoryForDeployment(app, appInfo)
	if len(commitHistory) == 0 {
		logger.Infof("Skipping DevLake payload for application %s (revision: %s) - no new commits to include", app.Name, appInfo.Revision)
		return nil
	}

	// Create and log DevLake payload
	deployment, hasCommits := ep.formatter.FormatDeployment(app, appInfo, appInfo.DeployedAt, commitHistory)
	if !hasCommits {
		logger.Infof("Skipping DevLake payload for application %s (revision: %s) - no new commits to include", app.Name, appInfo.Revision)
		return nil
	}

	ep.logDevLakePayload(deployment)

	// Send deployment to DevLake
	if err := ep.sendDeploymentToDevLake(ctx, deployment); err != nil {
		logger.Errorf("Failed to send deployment to DevLake: %v", err)
		// Don't return error - continue with storing the record
	}

	// Store deployment record
	ep.storeDeploymentRecord(ctx, app, appInfo, commitHistory)

	return nil
}

// commitHistoryToStrings converts commit history to string slice.
func (ep *EventProcessor) commitHistoryToStrings(commits []storage.CommitInfo) []string {
	var result []string
	for _, commit := range commits {
		result = append(result, commit.SHA)
	}
	return result
}

// logDevLakePayload logs the DevLake payload as a single JSON entry.
func (ep *EventProcessor) logDevLakePayload(deployment integrations.DevLakeCICDDeployment) {
	// Marshal the entire deployment to JSON
	jsonData, err := json.MarshalIndent(deployment, "", "  ")
	if err != nil {
		logger.Errorf("Failed to marshal DevLake payload to JSON: %v", err)
		return
	}

	logger.Infof("DEVLAKE_PAYLOAD: %s", string(jsonData))
}

// storeDeploymentRecord stores the deployment record in Redis.
func (ep *EventProcessor) storeDeploymentRecord(ctx context.Context, app *v1alpha1.Application, appInfo *api.ApplicationInfo, commitHistory []storage.CommitInfo) {
	// Use the image processor to get valid images
	imageProcessor := NewImageProcessor(ep.githubClient)
	validImages := imageProcessor.ExtractValidImages(appInfo.Images)

	record := &storage.DeploymentRecord{
		ApplicationName: app.Name,
		Namespace:       appInfo.Namespace,
		ComponentName:   appInfo.Component,
		ClusterName:     appInfo.Cluster,
		Revision:        appInfo.Revision,
		Images:          validImages,
		DeployedAt:      appInfo.DeployedAt,
		Environment:     appInfo.Environment,
		CommitHistory:   ep.commitHistoryToStrings(commitHistory),
	}

	if err := ep.storage.StoreDeployment(ctx, record); err != nil {
		logger.Errorf("Failed to store deployment record: %v", err)
	}
}

// sendDeploymentToDevLake sends a deployment to DevLake via the integration manager
func (ep *EventProcessor) sendDeploymentToDevLake(ctx context.Context, deployment integrations.DevLakeCICDDeployment) error {
	// Get the integration manager
	manager := integrations.GetManager()

	// Send deployment to DevLake
	return manager.SendDeploymentEventToDevLake(ctx, deployment)
}
