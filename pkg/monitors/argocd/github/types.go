// Package github provides GitHub API integration for ArgoCD monitoring.
// It handles commit validation, repository discovery, and commit history retrieval
// to support deployment tracking and DORA metrics collection.
package github

import (
	"time"

	"github.com/redhat-appstudio/dora-metrics/pkg/storage"
)

// Client defines the interface for GitHub API operations.
// It provides methods for validating commits, finding repositories,
// and retrieving commit history and PR information.
type Client interface {
	// IsValidCommit checks if a string is a valid Git commit hash
	IsValidCommit(commitSHA string) (bool, error)

	// FindRepositoryForCommit searches for the repository containing the given commit
	FindRepositoryForCommit(commitSHA string) (string, error)

	// GetCommitHistoryBetween retrieves commit history between two commits
	GetCommitHistoryBetween(oldSHA, newSHA, repoURL string) ([]storage.CommitInfo, error)

	// GetCommitMessage retrieves the commit message for a given commit
	GetCommitMessage(commitSHA, repoURL string) string

	// GetCommitDate retrieves the commit creation date for a given commit
	GetCommitDate(commitSHA, repoURL string) time.Time

	// GetPRInfoForCommit retrieves PR information for a given commit
	GetPRInfoForCommit(commitSHA, repoURL string) (*storage.PRInfo, error)
}

// Config holds GitHub client configuration.
type Config struct {
	// Token is the GitHub personal access token for API authentication
	Token string

	// BaseURL is the GitHub API base URL (for GitHub Enterprise)
	BaseURL string
}
