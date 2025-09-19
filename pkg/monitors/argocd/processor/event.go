// Package processor provides ArgoCD event processing functionality.
// It handles the core business logic for processing ArgoCD application events,
// including deployment tracking, commit validation, and DevLake payload generation.
package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	config       *api.Config
	storage      *storage.RedisClient
	githubClient github.Client
	parser       api.ApplicationParser
	formatter    *parser.Formatter
}

// NewEventProcessor creates a new event processor instance.
// It takes configuration, storage client, and GitHub client as dependencies.
func NewEventProcessor(
	config *api.Config,
	storage *storage.RedisClient,
	githubClient github.Client,
) api.EventHandler {
	return &EventProcessor{
		config:       config,
		storage:      storage,
		githubClient: githubClient,
		parser:       parser.NewApplicationParser(config),
		formatter:    parser.NewFormatter(githubClient, storage),
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
	logger.Infof("MODIFIED event received for application %s (sync revision: %s, last history: %s, health: %s)", app.Name, syncRevision, lastHistoryRevision, app.Status.Health.Status)

	// Check if this is a new deployment by comparing sync revision with last ArgoCD history
	if len(app.Status.History) > 0 {
		if lastHistoryRevision == syncRevision {
			logger.Infof("Sync revision %s matches last history entry %s, this is the deployed revision", syncRevision, lastHistoryRevision)
			// Continue to process this deployed revision
		} else {
			logger.Infof("Sync revision %s does NOT match last history entry %s, skipping deployment", syncRevision, lastHistoryRevision)
			return nil
		}
	} else {
		logger.Infof("No history found for application %s, skipping", app.Name)
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
	validImages := ep.extractValidImages(appInfo.Images)
	commitHistory, err := ep.getCommitHistoryForChangedImages(app, appInfo, validImages)
	if err != nil {
		logger.Warnf("Failed to get commit history for failed deployment: %v", err)
		commitHistory = []storage.CommitInfo{}
	}

	// If no commit history from changes, at least include current commits from images
	if len(commitHistory) == 0 {
		logger.Debugf("No commit history from changes for failed deployment, trying createCommitsFromImages")
		commitHistory = ep.createCommitsFromImages(app, validImages)
		logger.Debugf("createCommitsFromImages returned %d commits for failed deployment", len(commitHistory))
	}

	// Create a failed DevLake payload using the formatter to get real commit data
	displayTitle := fmt.Sprintf("Failed Deployment app: %s, component: %s, revision %s (%s)",
		app.Name, appInfo.Component, appInfo.Revision, appInfo.DeployedAt.Format("2006-01-02 15:04:05 MST"))
	name := fmt.Sprintf("deploy to production %s", appInfo.Revision)

	// Use the formatter to create a proper DevLake deployment with real commit data
	deployment, hasCommits := ep.formatter.FormatDeployment(app, appInfo, appInfo.DeployedAt, commitHistory)

	// Only process if we have commits
	if !hasCommits {
		logger.Infof("No commits found for failed deployment %s, skipping DevLake payload", app.Name)
		return nil
	}

	// Override the result to FAILED and update display info
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

	// Extract and validate images
	validImages := ep.extractValidImages(appInfo.Images)
	if len(validImages) == 0 {
		logger.Warnf("No valid commit images found for application %s, will only include infra-deployments commit", app.Name)
	}

	// Get commit history for changed images
	commitHistory, err := ep.getCommitHistoryForChangedImages(app, appInfo, validImages)
	if err != nil {
		logger.Warnf("Failed to get commit history: %v", err)
		commitHistory = []storage.CommitInfo{}
	}

	// If no commit history from changes, at least include current commits from images
	if len(commitHistory) == 0 {
		logger.Debugf("No commit history from changes, trying createCommitsFromImages")
		commitHistory = ep.createCommitsFromImages(app, validImages)
		logger.Debugf("createCommitsFromImages returned %d commits", len(commitHistory))
	}

	logger.Debugf("Final commit history has %d commits for application %s", len(commitHistory), app.Name)

	// Create and log DevLake payload
	deployment, hasCommits := ep.formatter.FormatDeployment(app, appInfo, appInfo.DeployedAt, commitHistory)

	// Only process if there are commits to include
	if !hasCommits {
		logger.Infof("Skipping DevLake payload for application %s (revision: %s) - no new commits to include", app.Name, appInfo.Revision)
		return nil
	}

	ep.logDevLakePayload(deployment)

	// Store deployment record
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

	if err := ep.storage.StoreDeployment(context.Background(), record); err != nil {
		logger.Errorf("Failed to store deployment record: %v", err)
	}

	return nil
}

// createCommitsFromImages creates commit info from current image tags and revision.
func (ep *EventProcessor) createCommitsFromImages(app *v1alpha1.Application, validImages []string) []storage.CommitInfo {
	logger.Infof("createCommitsFromImages called with %d valid images", len(validImages))
	var commits []storage.CommitInfo
	seen := make(map[string]bool)

	// Always include the deployment revision commit - find its repository first
	revisionRepoURL, err := ep.githubClient.FindRepositoryForCommit(app.Status.Sync.Revision)
	if err != nil {
		logger.Warnf("Failed to find repository for revision %s: %v", app.Status.Sync.Revision, err)
		// Try to get from history as fallback
		revisionRepoURL = ep.getRepoURLFromHistory(app, app.Status.Sync.Revision)
		if revisionRepoURL == "" {
			// Last resort fallback to infra-deployments
			revisionRepoURL = "https://github.com/redhat-appstudio/infra-deployments.git"
			logger.Warnf("Using fallback infra-deployments repo for revision %s", app.Status.Sync.Revision)
		} else {
			logger.Infof("Found revision %s repository from history: %s", app.Status.Sync.Revision, revisionRepoURL)
		}
	} else {
		logger.Infof("Found revision %s repository via GitHub search: %s", app.Status.Sync.Revision, revisionRepoURL)
	}

	commitMsg := ep.githubClient.GetCommitMessage(app.Status.Sync.Revision, revisionRepoURL)
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("Commit %s", app.Status.Sync.Revision[:8])
	}

	// Normalize the repository URL
	normalizedRepoURL := ep.normalizeRepoURL(revisionRepoURL)

	// Get commit creation date - this is REQUIRED for DevLake compliance
	commitDate := ep.githubClient.GetCommitDate(app.Status.Sync.Revision, revisionRepoURL)
	if commitDate.IsZero() {
		logger.Errorf("CRITICAL: Could not get commit date for %s from %s - this violates DevLake requirements", app.Status.Sync.Revision, revisionRepoURL)
		// Don't use fallback - we need the real commit date
		// Return empty slice since we can't process without commit date
		return []storage.CommitInfo{}
	}

	commits = append(commits, storage.CommitInfo{
		SHA:       app.Status.Sync.Revision,
		Message:   commitMsg,
		RepoURL:   normalizedRepoURL,
		CreatedAt: commitDate,
	})
	seen[app.Status.Sync.Revision] = true

	// Add commits from valid image tags (only if different from revision)
	for _, image := range validImages {
		tag := ep.extractTagFromImage(image)
		if tag == "" {
			continue // Skip if no tag
		}

		// Check if this commit is already added (by SHA only, since same commit can be in different repos)
		alreadyAdded := false
		for _, existingCommit := range commits {
			if existingCommit.SHA == tag {
				alreadyAdded = true
				break
			}
		}
		if alreadyAdded {
			continue // Skip if already added
		}

		// Find repository for this commit
		imageRepoURL, err := ep.githubClient.FindRepositoryForCommit(tag)
		if err != nil {
			logger.Warnf("Failed to find repository for commit %s: %v", tag, err)
			// Try to get from history as fallback
			imageRepoURL = ep.getRepoURLFromHistory(app, tag)
			if imageRepoURL == "" {
				logger.Warnf("Skipping commit %s - no repository found", tag)
				continue // Skip if we can't find the repository
			} else {
				logger.Infof("Found commit %s repository from history: %s", tag, imageRepoURL)
			}
		} else {
			logger.Infof("Found commit %s repository via GitHub search: %s", tag, imageRepoURL)
		}

		// Get commit message
		imageCommitMsg := ep.githubClient.GetCommitMessage(tag, imageRepoURL)
		if imageCommitMsg == "" {
			imageCommitMsg = fmt.Sprintf("Commit %s", tag[:8])
		}

		// Get commit creation date - this is REQUIRED for DevLake compliance
		imageCommitDate := ep.githubClient.GetCommitDate(tag, imageRepoURL)
		if imageCommitDate.IsZero() {
			logger.Errorf("CRITICAL: Could not get commit date for image %s from %s - this violates DevLake requirements", tag, imageRepoURL)
			// Don't use fallback - we need the real commit date
			continue // Skip this image if we can't get its commit date
		}

		// Normalize the repository URL
		normalizedImageRepoURL := ep.normalizeRepoURL(imageRepoURL)

		commits = append(commits, storage.CommitInfo{
			SHA:       tag,
			Message:   imageCommitMsg,
			RepoURL:   normalizedImageRepoURL,
			CreatedAt: imageCommitDate,
		})
		seen[tag] = true
	}

	return commits
}

// commitHistoryToStrings converts commit history to string slice.
func (ep *EventProcessor) commitHistoryToStrings(commits []storage.CommitInfo) []string {
	var result []string
	for _, commit := range commits {
		result = append(result, commit.SHA)
	}
	return result
}

// extractValidImages extracts and validates image tags that are commit hashes.
func (ep *EventProcessor) extractValidImages(images []string) []string {
	var validImages []string

	for _, image := range images {
		tag := ep.extractTagFromImage(image)
		if tag != "" {
			if valid, err := ep.githubClient.IsValidCommit(tag); err == nil && valid {
				validImages = append(validImages, image)
			}
		}
	}

	return validImages
}

// extractTagFromImage extracts the tag from a Docker image reference.
func (ep *EventProcessor) extractTagFromImage(image string) string {
	parts := strings.Split(image, ":")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
}

// getCommitHistoryForChangedImages gets commit history for images that have changed.
func (ep *EventProcessor) getCommitHistoryForChangedImages(
	app *v1alpha1.Application,
	appInfo *api.ApplicationInfo,
	validImages []string,
) ([]storage.CommitInfo, error) {
	var allCommits []storage.CommitInfo
	seen := make(map[string]bool) // Track by SHA only to prevent duplicates

	// Always include the deployment revision commit - find its repository first
	revisionRepoURL, err := ep.githubClient.FindRepositoryForCommit(app.Status.Sync.Revision)
	if err != nil {
		logger.Warnf("Failed to find repository for revision %s: %v", app.Status.Sync.Revision, err)
		// Try to get from history as fallback
		revisionRepoURL = ep.getRepoURLFromHistory(app, app.Status.Sync.Revision)
		if revisionRepoURL == "" {
			// Last resort fallback to infra-deployments
			revisionRepoURL = "https://github.com/redhat-appstudio/infra-deployments.git"
			logger.Warnf("Using fallback infra-deployments repo for revision %s", app.Status.Sync.Revision)
		} else {
			logger.Infof("Found revision %s repository from history: %s", app.Status.Sync.Revision, revisionRepoURL)
		}
	} else {
		logger.Infof("Found revision %s repository via GitHub search: %s", app.Status.Sync.Revision, revisionRepoURL)
	}

	commitMsg := ep.githubClient.GetCommitMessage(app.Status.Sync.Revision, revisionRepoURL)
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("Commit %s", app.Status.Sync.Revision[:8])
	}

	// Normalize the repository URL
	normalizedRepoURL := ep.normalizeRepoURL(revisionRepoURL)

	// Get commit creation date - this is REQUIRED for DevLake compliance
	commitDate := ep.githubClient.GetCommitDate(app.Status.Sync.Revision, revisionRepoURL)
	if commitDate.IsZero() {
		logger.Errorf("CRITICAL: Could not get commit date for %s from %s - this violates DevLake requirements", app.Status.Sync.Revision, revisionRepoURL)
		// Don't use fallback - we need the real commit date
		return []storage.CommitInfo{}, fmt.Errorf("failed to get commit date for %s", app.Status.Sync.Revision)
	}

	allCommits = append(allCommits, storage.CommitInfo{
		SHA:       app.Status.Sync.Revision,
		Message:   commitMsg,
		RepoURL:   normalizedRepoURL,
		CreatedAt: commitDate,
	})
	seen[app.Status.Sync.Revision] = true

	// Get previous deployment
	prevDeployment, err := ep.storage.GetDeployment(context.Background(), appInfo.Component, appInfo.Cluster)
	if err != nil {
		logger.Debugf("No previous deployment found for cluster %s, will add current image commits", appInfo.Cluster)
		// If no previous deployment, add current image commits only if there are valid images
		if len(validImages) == 0 {
			logger.Debugf("No valid image commits found, only returning infra-deployments commit (count: %d)", len(allCommits))
			return allCommits, nil
		}

		for _, image := range validImages {
			tag := ep.extractTagFromImage(image)
			if tag == "" {
				continue // Skip if no tag
			}

			// Check if this commit is already added (by SHA only, since same commit can be in different repos)
			alreadyAdded := false
			for _, existingCommit := range allCommits {
				if existingCommit.SHA == tag {
					alreadyAdded = true
					break
				}
			}
			if alreadyAdded {
				continue // Skip if already added
			}

			// Find repository for this commit
			imageRepoURL, err := ep.githubClient.FindRepositoryForCommit(tag)
			if err != nil {
				logger.Warnf("Failed to find repository for commit %s: %v", tag, err)
				// Try to get from history as fallback
				imageRepoURL = ep.getRepoURLFromHistory(app, tag)
				if imageRepoURL == "" {
					logger.Warnf("Skipping commit %s - no repository found", tag)
					continue // Skip if we can't find the repository
				} else {
					logger.Infof("Found commit %s repository from history: %s", tag, imageRepoURL)
				}
			} else {
				logger.Infof("Found commit %s repository via GitHub search: %s", tag, imageRepoURL)
			}

			imageCommitMsg := ep.githubClient.GetCommitMessage(tag, imageRepoURL)
			if imageCommitMsg == "" {
				imageCommitMsg = fmt.Sprintf("Commit %s", tag[:8])
			}

			// Get commit creation date - this is REQUIRED for DevLake compliance
			imageCommitDate := ep.githubClient.GetCommitDate(tag, imageRepoURL)
			if imageCommitDate.IsZero() {
				logger.Errorf("CRITICAL: Could not get commit date for image %s from %s - this violates DevLake requirements", tag, imageRepoURL)
				// Don't use fallback - we need the real commit date
				continue // Skip this image if we can't get its commit date
			}

			// Normalize the repository URL
			normalizedImageRepoURL := ep.normalizeRepoURL(imageRepoURL)

			allCommits = append(allCommits, storage.CommitInfo{
				SHA:       tag,
				Message:   imageCommitMsg,
				RepoURL:   normalizedImageRepoURL,
				CreatedAt: imageCommitDate,
			})
			seen[tag] = true
		}
		return allCommits, nil
	}

	// There is a previous deployment - compare old and new image tags and get commit history
	logger.Debugf("Previous deployment found for cluster %s, will compare image tags and get commit history", appInfo.Cluster)

	// Find changed images and get commit history between old and new tags
	changedImages := ep.findChangedImages(validImages, prevDeployment.Images)
	if len(changedImages) == 0 {
		logger.Debugf("No changed images found for application %s", app.Name)
		return allCommits, nil
	}

	// Get commit history for each changed image
	for _, image := range changedImages {
		commits, err := ep.getCommitHistoryForImage(app, image, prevDeployment.Images)
		if err != nil {
			logger.Warnf("Failed to get commit history for image %s: %v", image, err)
			continue
		}

		// Add unique commits from history
		for _, commit := range commits {
			if !seen[commit.SHA] {
				// Normalize the repository URL
				normalizedCommitRepoURL := ep.normalizeRepoURL(commit.RepoURL)
				commit.RepoURL = normalizedCommitRepoURL

				allCommits = append(allCommits, commit)
				seen[commit.SHA] = true
			}
		}
	}

	return allCommits, nil
}

// findChangedImages finds images that have changed between deployments.
func (ep *EventProcessor) findChangedImages(currentImages, previousImages []string) []string {
	var changedImages []string

	for _, currentImage := range currentImages {
		if !ep.imageExistsInPrevious(currentImage, previousImages) {
			changedImages = append(changedImages, currentImage)
		}
	}

	return changedImages
}

// imageExistsInPrevious checks if an image exists in the previous deployment.
func (ep *EventProcessor) imageExistsInPrevious(image string, previousImages []string) bool {
	baseImage := ep.getBaseImage(image)
	for _, prevImage := range previousImages {
		if ep.getBaseImage(prevImage) == baseImage {
			return true
		}
	}
	return false
}

// getBaseImage extracts the base image name without tag.
func (ep *EventProcessor) getBaseImage(image string) string {
	lastColon := strings.LastIndex(image, ":")
	if lastColon == -1 {
		return image
	}
	return image[:lastColon]
}

// getCommitHistoryForImage gets commit history for a specific image.
func (ep *EventProcessor) getCommitHistoryForImage(
	app *v1alpha1.Application,
	image string,
	previousImages []string,
) ([]storage.CommitInfo, error) {
	currentTag := ep.extractTagFromImage(image)
	baseImage := ep.getBaseImage(image)

	// Find the previous tag for the same base image
	var previousTag string
	for _, prevImage := range previousImages {
		if ep.getBaseImage(prevImage) == baseImage {
			previousTag = ep.extractTagFromImage(prevImage)
			break
		}
	}

	if previousTag == "" {
		logger.Debugf("No previous tag found for base image %s", baseImage)
		return []storage.CommitInfo{}, nil
	}

	// Find repository URL for the current tag
	repoURL, err := ep.githubClient.FindRepositoryForCommit(currentTag)
	if err != nil {
		logger.Warnf("Failed to find repository for current tag %s: %v", currentTag, err)
		// Try to get from history as fallback
		repoURL = ep.getRepoURLFromHistory(app, currentTag)
		if repoURL == "" {
			return []storage.CommitInfo{}, nil
		}
	}

	// Get commit history between tags
	return ep.githubClient.GetCommitHistoryBetween(previousTag, currentTag, repoURL)
}

// getRepoURLFromHistory extracts repository URL from ArgoCD application history.
func (ep *EventProcessor) getRepoURLFromHistory(app *v1alpha1.Application, commitSHA string) string {
	for _, historyItem := range app.Status.History {
		if historyItem.Revision == commitSHA && historyItem.Source.RepoURL != "" {
			return historyItem.Source.RepoURL
		}
	}
	return ""
}

// normalizeRepoURL normalizes repository URLs for comparison.
func (ep *EventProcessor) normalizeRepoURL(repoURL string) string {
	// Remove .git suffix and normalize
	normalized := strings.TrimSuffix(repoURL, ".git")
	normalized = strings.TrimSuffix(normalized, "/")
	return normalized
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
