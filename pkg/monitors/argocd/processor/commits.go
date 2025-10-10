// Package processor provides commit processing functionality for ArgoCD deployments.
// It handles commit history retrieval, validation, and DevLake formatting.
package processor

import (
	"context"
	"fmt"
	"strings"

	"github.com/redhat-appstudio/dora-metrics/pkg/logger"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/api"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/github"
	"github.com/redhat-appstudio/dora-metrics/pkg/storage"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// CommitProcessor handles commit-related operations for ArgoCD deployments.
type CommitProcessor struct {
	githubClient   github.Client
	storage        *storage.RedisClient
	imageProcessor *ImageProcessor
}

// NewCommitProcessor creates a new commit processor instance.
func NewCommitProcessor(githubClient github.Client, storage *storage.RedisClient) *CommitProcessor {
	return &CommitProcessor{
		githubClient:   githubClient,
		storage:        storage,
		imageProcessor: NewImageProcessor(githubClient),
	}
}

// GetCommitHistoryForDeployment gets the complete commit history for a deployment.
func (cp *CommitProcessor) GetCommitHistoryForDeployment(app *v1alpha1.Application, appInfo *api.ApplicationInfo) []storage.CommitInfo {
	// Extract and validate images
	validImages := cp.imageProcessor.ExtractValidImages(appInfo.Images)
	if len(validImages) == 0 {
		logger.Warnf("No valid commit images found for application %s, will only include infra-deployments commit", app.Name)
	}

	// Get commit history for changed images
	commitHistory, err := cp.getCommitHistoryForChangedImages(app, appInfo, validImages)
	if err != nil {
		logger.Warnf("Failed to get commit history: %v", err)
		commitHistory = []storage.CommitInfo{}
	}

	// If no commit history from changes, at least include current commits from images
	if len(commitHistory) == 0 {
		logger.Debugf("No commit history from changes, trying createCommitsFromImages")
		commitHistory = cp.createCommitsFromImages(app, validImages)
		logger.Debugf("createCommitsFromImages returned %d commits", len(commitHistory))
	}

	logger.Debugf("Final commit history has %d commits for application %s", len(commitHistory), app.Name)
	return commitHistory
}

// createCommitsFromImages creates commit info from current image tags and revision.
func (cp *CommitProcessor) createCommitsFromImages(app *v1alpha1.Application, validImages []string) []storage.CommitInfo {
	logger.Infof("createCommitsFromImages called with %d valid images", len(validImages))
	var commits []storage.CommitInfo
	seen := make(map[string]bool)

	// Always include the deployment revision commit - find its repository first
	revisionRepoURL, err := cp.githubClient.FindRepositoryForCommit(app.Status.Sync.Revision)
	if err != nil {
		logger.Warnf("Failed to find repository for revision %s: %v", app.Status.Sync.Revision, err)
		// Try to get from history as fallback
		revisionRepoURL = cp.getRepoURLFromHistory(app, app.Status.Sync.Revision)
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

	commitMsg := cp.githubClient.GetCommitMessage(app.Status.Sync.Revision, revisionRepoURL)
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("Commit %s", app.Status.Sync.Revision[:8])
	}

	// Normalize the repository URL
	normalizedRepoURL := cp.normalizeRepoURL(revisionRepoURL)

	// Get commit creation date - this is REQUIRED for DevLake compliance
	commitDate := cp.githubClient.GetCommitDate(app.Status.Sync.Revision, revisionRepoURL)
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
		tag := cp.imageProcessor.extractTagFromImage(image)
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
		imageRepoURL, err := cp.githubClient.FindRepositoryForCommit(tag)
		if err != nil {
			logger.Warnf("Failed to find repository for commit %s: %v", tag, err)
			// Try to get from history as fallback
			imageRepoURL = cp.getRepoURLFromHistory(app, tag)
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
		imageCommitMsg := cp.githubClient.GetCommitMessage(tag, imageRepoURL)
		if imageCommitMsg == "" {
			imageCommitMsg = fmt.Sprintf("Commit %s", tag[:8])
		}

		// Get commit creation date - this is REQUIRED for DevLake compliance
		imageCommitDate := cp.githubClient.GetCommitDate(tag, imageRepoURL)
		if imageCommitDate.IsZero() {
			logger.Errorf("CRITICAL: Could not get commit date for image %s from %s - this violates DevLake requirements", tag, imageRepoURL)
			// Don't use fallback - we need the real commit date
			continue // Skip this image if we can't get its commit date
		}

		// Normalize the repository URL
		normalizedImageRepoURL := cp.normalizeRepoURL(imageRepoURL)

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

// getCommitHistoryForChangedImages gets commit history for images that have changed.
func (cp *CommitProcessor) getCommitHistoryForChangedImages(
	app *v1alpha1.Application,
	appInfo *api.ApplicationInfo,
	validImages []string,
) ([]storage.CommitInfo, error) {
	var allCommits []storage.CommitInfo
	seen := make(map[string]bool) // Track by SHA only to prevent duplicates

	// Always include the deployment revision commit - find its repository first
	revisionRepoURL, err := cp.githubClient.FindRepositoryForCommit(app.Status.Sync.Revision)
	if err != nil {
		logger.Warnf("Failed to find repository for revision %s: %v", app.Status.Sync.Revision, err)
		// Try to get from history as fallback
		revisionRepoURL = cp.getRepoURLFromHistory(app, app.Status.Sync.Revision)
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

	commitMsg := cp.githubClient.GetCommitMessage(app.Status.Sync.Revision, revisionRepoURL)
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("Commit %s", app.Status.Sync.Revision[:8])
	}

	// Normalize the repository URL
	normalizedRepoURL := cp.normalizeRepoURL(revisionRepoURL)

	// Get commit creation date - this is REQUIRED for DevLake compliance
	commitDate := cp.githubClient.GetCommitDate(app.Status.Sync.Revision, revisionRepoURL)
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
	prevDeployment, err := cp.storage.GetDeployment(context.Background(), appInfo.Component, appInfo.Cluster)
	if err != nil {
		logger.Debugf("No previous deployment found for cluster %s, will add current image commits", appInfo.Cluster)
		// If no previous deployment, add current image commits only if there are valid images
		if len(validImages) == 0 {
			logger.Debugf("No valid image commits found, only returning infra-deployments commit (count: %d)", len(allCommits))
			return allCommits, nil
		}

		for _, image := range validImages {
			tag := cp.imageProcessor.extractTagFromImage(image)
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
			imageRepoURL, err := cp.githubClient.FindRepositoryForCommit(tag)
			if err != nil {
				logger.Warnf("Failed to find repository for commit %s: %v", tag, err)
				// Try to get from history as fallback
				imageRepoURL = cp.getRepoURLFromHistory(app, tag)
				if imageRepoURL == "" {
					logger.Warnf("Skipping commit %s - no repository found", tag)
					continue // Skip if we can't find the repository
				} else {
					logger.Infof("Found commit %s repository from history: %s", tag, imageRepoURL)
				}
			} else {
				logger.Infof("Found commit %s repository via GitHub search: %s", tag, imageRepoURL)
			}

			imageCommitMsg := cp.githubClient.GetCommitMessage(tag, imageRepoURL)
			if imageCommitMsg == "" {
				imageCommitMsg = fmt.Sprintf("Commit %s", tag[:8])
			}

			// Get commit creation date - this is REQUIRED for DevLake compliance
			imageCommitDate := cp.githubClient.GetCommitDate(tag, imageRepoURL)
			if imageCommitDate.IsZero() {
				logger.Errorf("CRITICAL: Could not get commit date for image %s from %s - this violates DevLake requirements", tag, imageRepoURL)
				// Don't use fallback - we need the real commit date
				continue // Skip this image if we can't get its commit date
			}

			// Normalize the repository URL
			normalizedImageRepoURL := cp.normalizeRepoURL(imageRepoURL)

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
	changedImages := cp.imageProcessor.FindChangedImages(validImages, prevDeployment.Images)
	if len(changedImages) == 0 {
		logger.Debugf("No changed images found for application %s", app.Name)
		return allCommits, nil
	}

	// Get commit history for each changed image
	for _, image := range changedImages {
		commits, err := cp.imageProcessor.GetCommitHistoryForImage(app, image, prevDeployment.Images)
		if err != nil {
			logger.Warnf("Failed to get commit history for image %s: %v", image, err)
			continue
		}

		// Add unique commits from history
		for _, commit := range commits {
			if !seen[commit.SHA] {
				// Normalize the repository URL
				normalizedCommitRepoURL := cp.normalizeRepoURL(commit.RepoURL)
				commit.RepoURL = normalizedCommitRepoURL

				allCommits = append(allCommits, commit)
				seen[commit.SHA] = true
			}
		}
	}

	return allCommits, nil
}

// getRepoURLFromHistory extracts repository URL from ArgoCD application history.
func (cp *CommitProcessor) getRepoURLFromHistory(app *v1alpha1.Application, commitSHA string) string {
	for _, historyItem := range app.Status.History {
		if historyItem.Revision == commitSHA && historyItem.Source.RepoURL != "" {
			return historyItem.Source.RepoURL
		}
	}
	return ""
}

// normalizeRepoURL normalizes repository URLs for comparison.
func (cp *CommitProcessor) normalizeRepoURL(repoURL string) string {
	// Remove .git suffix and normalize
	normalized := strings.TrimSuffix(repoURL, ".git")
	normalized = strings.TrimSuffix(normalized, "/")
	return normalized
}
