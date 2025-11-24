// Package processor provides ArgoCD event processing functionality.
package processor

import (
	"fmt"
	"strings"
	"time"

	"github.com/redhat-appstudio/dora-metrics/pkg/logger"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/github"
	"github.com/redhat-appstudio/dora-metrics/pkg/storage"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// commitHelper provides common commit processing operations.
type commitHelper struct {
	githubClient github.Client
}

// newCommitHelper creates a new commit helper.
func newCommitHelper(githubClient github.Client) *commitHelper {
	return &commitHelper{
		githubClient: githubClient,
	}
}

// findRepositoryForCommit finds the repository URL for a commit.
// It tries history first, then GitHub API, then falls back to infra-deployments.
func (ch *commitHelper) findRepositoryForCommit(app *v1alpha1.Application, commitSHA string) string {
	// Try history first (no API call)
	if repoURL := ch.getRepoURLFromHistory(app, commitSHA); repoURL != "" {
		logger.Debugf("Found commit %s repository from history: %s", commitSHA, repoURL)
		return repoURL
	}

	// Try GitHub API
	repoURL, err := ch.githubClient.FindRepositoryForCommit(commitSHA)
	if err == nil {
		logger.Debugf("Found commit %s repository via GitHub search: %s", commitSHA, repoURL)
		return repoURL
	}

	// Fallback to infra-deployments
	logger.Debugf("Using fallback infra-deployments repo for commit %s", commitSHA)
	return "https://github.com/redhat-appstudio/infra-deployments.git"
}

// getCommitInfo retrieves commit information (message, date) for a commit.
func (ch *commitHelper) getCommitInfo(commitSHA, repoURL string) (message string, date time.Time, err error) {
	message = ch.githubClient.GetCommitMessage(commitSHA, repoURL)
	if message == "" {
		message = ch.formatCommitMessage(commitSHA)
	}

	date = ch.githubClient.GetCommitDate(commitSHA, repoURL)
	if date.IsZero() {
		return "", time.Time{}, fmt.Errorf("failed to get commit date for %s", commitSHA)
	}

	return message, date, nil
}

// formatCommitMessage formats a commit message fallback.
func (ch *commitHelper) formatCommitMessage(commitSHA string) string {
	if len(commitSHA) >= 8 {
		return fmt.Sprintf("Commit %s", commitSHA[:8])
	}
	return fmt.Sprintf("Commit %s", commitSHA)
}

// createCommitInfo creates a CommitInfo from commit details.
func (ch *commitHelper) createCommitInfo(commitSHA, message, repoURL string, createdAt time.Time) storage.CommitInfo {
	return storage.CommitInfo{
		SHA:       commitSHA,
		Message:   message,
		RepoURL:   ch.normalizeRepoURL(repoURL),
		CreatedAt: createdAt,
	}
}

// getRepoURLFromHistory extracts repository URL from ArgoCD application history.
func (ch *commitHelper) getRepoURLFromHistory(app *v1alpha1.Application, commitSHA string) string {
	for _, historyItem := range app.Status.History {
		if historyItem.Revision == commitSHA && historyItem.Source.RepoURL != "" {
			return historyItem.Source.RepoURL
		}
	}
	return ""
}

// normalizeRepoURL normalizes repository URLs for comparison.
// Handles variations like:
// - https://github.com/user/repo
// - https://github.com/user/repo.git
// - https://github.com/user/repo/
// - http://github.com/user/repo (converts to https)
func (ch *commitHelper) normalizeRepoURL(repoURL string) string {
	if repoURL == "" {
		return ""
	}
	// Convert to lowercase for case-insensitive comparison
	normalized := strings.ToLower(repoURL)
	// Remove .git suffix
	normalized = strings.TrimSuffix(normalized, ".git")
	// Remove trailing slashes
	normalized = strings.TrimSuffix(normalized, "/")
	// Normalize http to https
	normalized = strings.Replace(normalized, "http://", "https://", 1)
	return normalized
}

// isCommitAlreadyAdded checks if a commit SHA is already in the commits list.
func (ch *commitHelper) isCommitAlreadyAdded(commits []storage.CommitInfo, sha string) bool {
	for _, commit := range commits {
		if commit.SHA == sha {
			return true
		}
	}
	return false
}
