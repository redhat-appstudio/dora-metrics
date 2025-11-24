// Package processor provides ArgoCD event processing functionality.
// It handles the core business logic for processing ArgoCD application events,
// including deployment tracking, commit validation, and DevLake payload generation.
package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redhat-appstudio/dora-metrics/pkg/integrations"
	"github.com/redhat-appstudio/dora-metrics/pkg/logger"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/api"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/github"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/parser"
	"github.com/redhat-appstudio/dora-metrics/pkg/storage"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// EventProcessor handles ArgoCD application event processing.
// It coordinates between different components to process events, validate commits,
// track deployments, and generate DevLake payloads.
type EventProcessor struct {
	config          *api.Config
	storage         *storage.RedisClient
	githubClient    github.Client
	argocdClient    api.Client
	parser          api.ApplicationParser
	formatter       *parser.Formatter
	commitProcessor *CommitProcessor
	validator       *AppValidator
}

// NewEventProcessor creates a new event processor instance.
// It takes configuration, storage client, GitHub client, and ArgoCD client as dependencies.
func NewEventProcessor(config *api.Config, storage *storage.RedisClient, githubClient github.Client, argocdClient api.Client) api.EventHandler {
	return &EventProcessor{
		config:          config,
		storage:         storage,
		githubClient:    githubClient,
		argocdClient:    argocdClient,
		parser:          parser.NewApplicationParser(config),
		formatter:       parser.NewFormatter(githubClient, storage),
		commitProcessor: NewCommitProcessor(githubClient, storage, config.RepositoryBlacklist),
		validator:       NewAppValidator(),
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
	syncRevision := app.Status.Sync.Revision

	// Early validation checks - MUST pass all checks to continue
	if !ep.validator.ShouldProcess(app, syncRevision) {
		if !ep.validator.isHealthy(app) {
			logger.Debugf("Application %s health status is not acceptable (status: %s), skipping", app.Name, app.Status.Health.Status)
		}
		if !ep.validator.isSynced(app) {
			logger.Debugf("Application %s sync status is not acceptable (status: %s), skipping - not actually deployed yet",
				app.Name, app.Status.Sync.Status)
		}
		return nil
	}

	// Fetch fresh application state for accurate history validation
	freshApp, err := ep.fetchApplicationFromArgoCD(ctx, app.Name, app.Namespace)
	if err != nil {
		logger.Warnf("Failed to fetch fresh application state for %s: %v, using event state", app.Name, err)
		freshApp = app
		// CRITICAL: Even when falling back to original app, verify it's still healthy AND synced
		// The app might have become unhealthy or out of sync between the early check and now
		if !ep.validator.isHealthy(freshApp) {
			logger.Debugf("Application %s (fallback) health status is not acceptable (status: %s), skipping - will not process",
				freshApp.Name, freshApp.Status.Health.Status)
			return nil
		}
		if !ep.validator.isSynced(freshApp) {
			logger.Debugf("Application %s (fallback) sync status is not acceptable (status: %s), skipping - not actually deployed yet",
				freshApp.Name, freshApp.Status.Sync.Status)
			return nil
		}
	} else {
		logger.Debugf("Fetched fresh application state for %s from ArgoCD API (health: %s, sync: %s)",
			app.Name, freshApp.Status.Health.Status, freshApp.Status.Sync.Status)

		// CRITICAL: If we successfully fetched fresh app, it MUST be healthy AND synced to continue
		// Never process if fresh app is not healthy or not synced
		if !ep.validator.isHealthy(freshApp) {
			logger.Debugf("Fresh application %s health status is not acceptable (status: %s), skipping - will not process",
				freshApp.Name, freshApp.Status.Health.Status)
			return nil
		}
		if !ep.validator.isSynced(freshApp) {
			logger.Debugf("Fresh application %s sync status is not acceptable (status: %s), skipping - not actually deployed yet",
				freshApp.Name, freshApp.Status.Sync.Status)
			return nil
		}
	}

	// Validate revision exists in deployment history
	if !ep.validator.IsRevisionInHistory(freshApp, syncRevision) {
		if len(freshApp.Status.History) > 0 {
			logger.Debugf("Sync revision %s is not found in deployment history for %s, skipping",
				syncRevision, app.Name)
		} else {
			logger.Debugf("Application %s has no deployment history, skipping", app.Name)
		}
		return nil
	}

	app = freshApp

	// Try to acquire a processing lock to prevent concurrent processing of the same deployment
	lockAcquired, err := ep.storage.AcquireProcessingLock(ctx, app.Name, appInfo.Cluster, syncRevision)
	if err != nil {
		logger.Warnf("Failed to acquire processing lock for %s/%s: %v, proceeding", app.Name, appInfo.Cluster, err)
		// Continue without lock if we can't acquire it (fail open)
	} else if !lockAcquired {
		// Another process is already processing this deployment
		logger.Debugf("Deployment %s/%s (revision: %s) is already being processed by another worker, skipping", app.Name, appInfo.Cluster, syncRevision)
		return nil
	}

	// Ensure we release the lock when done
	defer func() {
		if lockAcquired {
			if err := ep.storage.ReleaseProcessingLock(ctx, app.Name, appInfo.Cluster, syncRevision); err != nil {
				logger.Warnf("Failed to release processing lock for %s/%s: %v", app.Name, appInfo.Cluster, err)
			}
		}
	}()

	// Check if this is a new or fresh deployment
	if !ep.isNewOrFreshDeployment(ctx, app, appInfo, syncRevision) {
		return nil
	}

	logger.Infof("Processing new deployment for application %s (revision: %s)", app.Name, syncRevision)
	return ep.processNewDeployment(ctx, app, appInfo)
}

// isNewOrFreshDeployment checks if this is a new deployment or a fresh event for the same revision.
func (ep *EventProcessor) isNewOrFreshDeployment(ctx context.Context, app *v1alpha1.Application, appInfo *api.ApplicationInfo, syncRevision string) bool {
	isNew, err := ep.storage.IsNewDeployment(ctx, app.Name, appInfo.Cluster, syncRevision)
	if err != nil {
		logger.Warnf("Failed to check if deployment is new: %v, proceeding", err)
		return true
	}

	if isNew {
		return true
	}

	// Check if this is a fresh deployment event (same revision, later timestamp)
	lastDeployment, err := ep.storage.GetDeployment(ctx, app.Name, appInfo.Cluster)
	if err != nil || lastDeployment == nil {
		logger.Debugf("Revision %s already processed for %s, skipping", syncRevision, app.Name)
		return false
	}

	deployedAt := ep.validator.GetDeployedTimestamp(app, syncRevision)
	if deployedAt.IsZero() || !deployedAt.After(lastDeployment.DeployedAt) {
		logger.Debugf("Revision %s already processed for %s (same deployment), skipping", syncRevision, app.Name)
		return false
	}

	logger.Infof("Same revision %s but new deployment event (deployed at %v vs last %v), processing",
		syncRevision, deployedAt, lastDeployment.DeployedAt)
	return true
}

// fetchApplicationFromArgoCD fetches the latest application state from ArgoCD API.
// This ensures we have the most up-to-date history to verify deployments.
func (ep *EventProcessor) fetchApplicationFromArgoCD(ctx context.Context, appName, namespace string) (*v1alpha1.Application, error) {
	if ep.argocdClient == nil {
		return nil, fmt.Errorf("ArgoCD client not available")
	}

	argocdClientset := ep.argocdClient.GetArgoCDClient()
	if argocdClientset == nil {
		return nil, fmt.Errorf("ArgoCD clientset not available")
	}

	// Fetch the application from ArgoCD API
	freshApp, err := argocdClientset.ArgoprojV1alpha1().Applications(namespace).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get application from ArgoCD API: %w", err)
	}

	return freshApp, nil
}

// handleDeletedEvent processes a DELETED event.
func (ep *EventProcessor) handleDeletedEvent(ctx context.Context, app *v1alpha1.Application, _ *api.ApplicationInfo) error {
	logger.Infof("Application %s deleted from namespace %s", app.Name, app.Namespace)
	return nil
}

// processNewDeployment processes a new deployment.
func (ep *EventProcessor) processNewDeployment(ctx context.Context, app *v1alpha1.Application, appInfo *api.ApplicationInfo) error {
	// Get deployed timestamp from history
	deployedAt := ep.validator.GetDeployedTimestamp(app, appInfo.Revision)
	if deployedAt.IsZero() {
		deployedAt = time.Now()
		logger.Warnf("No deployed timestamp in history for revision %s, using current time", appInfo.Revision)
	}
	appInfo.DeployedAt = deployedAt

	// Get commit history (already filtered to exclude blacklisted repositories)
	commitHistory := ep.commitProcessor.GetCommitHistoryForDeployment(app, appInfo)
	if len(commitHistory) == 0 {
		if appInfo.Revision != "" {
			logger.Debugf("Skipping DevLake payload for %s (revision: %s) - no commits remaining after filtering", app.Name, appInfo.Revision)
		}
		ep.storeDeploymentRecord(ctx, app, appInfo, commitHistory)
		return nil
	}

	logger.Debugf("Proceeding with DevLake payload for %s - %d commit(s) remaining after blacklist filtering", app.Name, len(commitHistory))

	// Format and send deployment
	deployment, hasCommits := ep.formatter.FormatDeployment(app, appInfo, deployedAt, commitHistory)
	if !hasCommits {
		if appInfo.Revision != "" {
			logger.Debugf("Skipping DevLake payload for %s (revision: %s) - no commits", app.Name, appInfo.Revision)
		}
		ep.storeDeploymentRecord(ctx, app, appInfo, commitHistory)
		return nil
	}

	ep.logDevLakePayload(deployment)
	if err := ep.sendDeploymentToDevLake(ctx, deployment); err != nil {
		logger.Errorf("Failed to send deployment to DevLake: %v", err)
	}

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
