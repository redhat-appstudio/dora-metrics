// Package processor provides commit processing functionality for ArgoCD deployments.
// It handles commit history retrieval, validation, and DevLake formatting.
package processor

import (
	"context"
	"fmt"

	"github.com/redhat-appstudio/dora-metrics/pkg/logger"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/api"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/github"
	"github.com/redhat-appstudio/dora-metrics/pkg/storage"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// CommitProcessor handles commit-related operations for ArgoCD deployments.
type CommitProcessor struct {
	githubClient   github.Client
	storage        *storage.RedisClient
	imageProcessor *ImageProcessor
	commitHelper   *commitHelper
	blacklist      []string
}

// NewCommitProcessor creates a new commit processor instance.
func NewCommitProcessor(githubClient github.Client, storage *storage.RedisClient, repositoryBlacklist []string) *CommitProcessor {
	if len(repositoryBlacklist) > 0 {
		logger.Infof("Repository blacklist initialized with %d repository(ies): %v", len(repositoryBlacklist), repositoryBlacklist)
	} else {
		logger.Debugf("Repository blacklist is empty - no repositories will be filtered")
	}
	return &CommitProcessor{
		githubClient:   githubClient,
		storage:        storage,
		imageProcessor: NewImageProcessor(githubClient),
		commitHelper:   newCommitHelper(githubClient),
		blacklist:      repositoryBlacklist,
	}
}

// GetCommitHistoryForDeployment gets the complete commit history for a deployment.
func (cp *CommitProcessor) GetCommitHistoryForDeployment(app *v1alpha1.Application, appInfo *api.ApplicationInfo) []storage.CommitInfo {
	// Early check: Skip if revision is empty - return empty, no logs
	if appInfo.Revision == "" {
		return []storage.CommitInfo{}
	}

	// Extract and validate images
	validImages := cp.imageProcessor.ExtractValidImages(appInfo.Images)
	if len(validImages) == 0 {
		logger.Debugf("No valid commit images found for application %s, will only include infra-deployments commit", app.Name)
	}

	// Get commit history for changed images
	commitHistory, err := cp.getCommitHistoryForChangedImages(app, appInfo, validImages)
	if err != nil {
		// Only log if it's not an empty revision error
		if appInfo.Revision != "" {
			logger.Debugf("Failed to get commit history for %s: %v", app.Name, err)
		}
		commitHistory = []storage.CommitInfo{}
	}

	// If no commit history from changes, at least include current commits from images
	if len(commitHistory) == 0 {
		logger.Debugf("No commit history from changes, trying createCommitsFromImages")
		commitHistory = cp.createCommitsFromImages(app, appInfo, validImages)
		logger.Debugf("createCommitsFromImages returned %d commits", len(commitHistory))
	}

	logger.Debugf("Final commit history has %d commits for application %s", len(commitHistory), app.Name)

	// Filter out commits from blacklisted repositories
	// IMPORTANT: Only commits from blacklisted repos are removed - all other commits are kept
	filteredHistory := cp.filterBlacklistedRepos(commitHistory)
	if len(filteredHistory) < len(commitHistory) {
		logger.Infof("Filtered out %d commit(s) from blacklisted repositories for application %s, keeping %d commit(s) for DevLake payload",
			len(commitHistory)-len(filteredHistory), app.Name, len(filteredHistory))
	}

	// Return filtered history - if any commits remain, they will be included in the payload
	return filteredHistory
}

// filterBlacklistedRepos filters out commits from blacklisted repositories.
func (cp *CommitProcessor) filterBlacklistedRepos(commits []storage.CommitInfo) []storage.CommitInfo {
	if len(cp.blacklist) == 0 {
		logger.Debugf("No repository blacklist configured, skipping filter")
		return commits
	}

	// Normalize blacklist URLs for comparison (remove .git suffix and trailing slashes)
	normalizedBlacklist := make(map[string]bool)
	for _, repo := range cp.blacklist {
		normalized := cp.commitHelper.normalizeRepoURL(repo)
		normalizedBlacklist[normalized] = true
		logger.Debugf("Blacklisted repository (normalized): %s -> %s", repo, normalized)
	}

	var filtered []storage.CommitInfo
	for _, commit := range commits {
		normalizedRepo := cp.commitHelper.normalizeRepoURL(commit.RepoURL)
		if normalizedBlacklist[normalizedRepo] {
			logger.Infof("FILTERED: Commit %s from blacklisted repository %s (normalized: %s) - will NOT be included in DevLake payload",
				commit.SHA, commit.RepoURL, normalizedRepo)
			continue
		}
		filtered = append(filtered, commit)
	}

	if len(filtered) < len(commits) {
		logger.Infof("Repository blacklist filter: removed %d commit(s) from %d total commits",
			len(commits)-len(filtered), len(commits))
	}

	return filtered
}

// createCommitsFromImages creates commit info from current image tags and revision.
func (cp *CommitProcessor) createCommitsFromImages(app *v1alpha1.Application, appInfo *api.ApplicationInfo, validImages []string) []storage.CommitInfo {
	deployedRevision := appInfo.Revision
	if deployedRevision == "" {
		return []storage.CommitInfo{}
	}

	logger.Debugf("createCommitsFromImages called with %d valid images for %s", len(validImages), app.Name)
	var commits []storage.CommitInfo

	// Add deployment revision commit
	revisionCommit := cp.createCommitFromSHA(app, deployedRevision)
	if revisionCommit.SHA != "" {
		commits = append(commits, revisionCommit)
	}

	// Add commits from valid image tags (only if different from revision)
	for _, image := range validImages {
		tag := cp.imageProcessor.extractTagFromImage(image)
		if tag == "" || tag == deployedRevision {
			continue
		}

		if cp.commitHelper.isCommitAlreadyAdded(commits, tag) {
			continue
		}

		imageCommit := cp.createCommitFromSHA(app, tag)
		if imageCommit.SHA != "" {
			commits = append(commits, imageCommit)
		}
	}

	return commits
}

// createCommitFromSHA creates a CommitInfo from a commit SHA.
func (cp *CommitProcessor) createCommitFromSHA(app *v1alpha1.Application, commitSHA string) storage.CommitInfo {
	repoURL := cp.commitHelper.findRepositoryForCommit(app, commitSHA)
	message, date, err := cp.commitHelper.getCommitInfo(commitSHA, repoURL)
	if err != nil {
		logger.Errorf("CRITICAL: Could not get commit info for %s: %v - violates DevLake requirements", commitSHA, err)
		return storage.CommitInfo{}
	}

	return cp.commitHelper.createCommitInfo(commitSHA, message, repoURL, date)
}

// getCommitHistoryForChangedImages gets commit history for images that have changed.
func (cp *CommitProcessor) getCommitHistoryForChangedImages(
	app *v1alpha1.Application,
	appInfo *api.ApplicationInfo,
	validImages []string,
) ([]storage.CommitInfo, error) {
	// Use the revision from appInfo (sync revision, validated against history in event processor)
	deployedRevision := appInfo.Revision

	// Early check: Skip if revision is empty - return empty, no logs
	if deployedRevision == "" {
		return []storage.CommitInfo{}, nil
	}

	var allCommits []storage.CommitInfo
	seen := make(map[string]bool) // Track by SHA only to prevent duplicates

	// Always include the deployment revision commit
	revisionCommit := cp.createCommitFromSHA(app, deployedRevision)
	if revisionCommit.SHA == "" {
		return []storage.CommitInfo{}, fmt.Errorf("failed to get commit info for %s", deployedRevision)
	}
	allCommits = append(allCommits, revisionCommit)
	seen[deployedRevision] = true

	// Get previous deployment
	prevDeployment, err := cp.storage.GetDeployment(context.Background(), appInfo.Component, appInfo.Cluster)
	if err != nil || prevDeployment == nil {
		logger.Debugf("No previous deployment found for cluster %s, will add current image commits", appInfo.Cluster)
		// If no previous deployment, add current image commits only if there are valid images
		if len(validImages) == 0 {
			logger.Debugf("No valid image commits found, only returning infra-deployments commit (count: %d)", len(allCommits))
			return allCommits, nil
		}

		for _, image := range validImages {
			tag := cp.imageProcessor.extractTagFromImage(image)
			if tag == "" {
				logger.Warnf("Skipping image %s - no tag extracted", image)
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

			// Create commit info for this image tag
			imageCommit := cp.createCommitFromSHA(app, tag)
			if imageCommit.SHA == "" {
				continue // Skip if we can't get commit info
			}
			allCommits = append(allCommits, imageCommit)
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
				commit.RepoURL = cp.commitHelper.normalizeRepoURL(commit.RepoURL)
				allCommits = append(allCommits, commit)
				seen[commit.SHA] = true
			}
		}
	}

	return allCommits, nil
}
