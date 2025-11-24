package github

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/redhat-appstudio/dora-metrics/pkg/logger"
	"github.com/redhat-appstudio/dora-metrics/pkg/storage"

	"github.com/google/go-github/v53/github"
	"golang.org/x/oauth2"
)

var (
	commitHashRegex = regexp.MustCompile(`^[a-fA-F0-9]{7,40}$`)
)

// client implements the GitHub Client interface.
type client struct {
	github *github.Client
	config *Config
}

// NewClient creates a new GitHub client instance.
func NewClient(config *Config) Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.Token},
	)
	tc := oauth2.NewClient(ctx, ts)

	githubClient := github.NewClient(tc)
	if config.BaseURL != "" {
		baseURL, err := url.Parse(config.BaseURL)
		if err == nil {
			// Ensure BaseURL has trailing slash
			if !strings.HasSuffix(baseURL.Path, "/") {
				baseURL.Path += "/"
			}
			githubClient.BaseURL = baseURL
		}
	}

	return &client{
		github: githubClient,
		config: config,
	}
}

// IsValidCommit checks if a string is a valid commit hash.
func (c *client) IsValidCommit(commitSHA string) (bool, error) {
	return commitHashRegex.MatchString(commitSHA), nil
}

// FindRepositoryForCommit searches for the repository containing the given commit.
func (c *client) FindRepositoryForCommit(commitSHA string) (string, error) {
	// Validate commit SHA is not empty
	if commitSHA == "" {
		return "", fmt.Errorf("commit SHA cannot be empty")
	}

	ctx := context.Background()

	query := fmt.Sprintf("hash:%s", commitSHA)
	opts := &github.SearchOptions{
		Sort:        "indexed",
		Order:       "desc",
		ListOptions: github.ListOptions{PerPage: 10}, // Get more results to find the original repo
	}

	result, _, err := c.github.Search.Commits(ctx, query, opts)
	if err != nil {
		return "", fmt.Errorf("failed to search for commit: %w", err)
	}

	if len(result.Commits) == 0 {
		return "", fmt.Errorf("commit %s not found", commitSHA)
	}

	// Look for the original repository (not infra-deployments)
	for _, commit := range result.Commits {
		repoURL := *commit.Repository.HTMLURL
		// Skip infra-deployments as it's usually the merge target
		if !strings.Contains(repoURL, "infra-deployments") {
			return repoURL, nil
		}
	}

	// If all results are infra-deployments, return the first one
	commit := result.Commits[0]
	return *commit.Repository.HTMLURL, nil
}

// GetCommitHistoryBetween retrieves commit history between two commits.
func (c *client) GetCommitHistoryBetween(oldSHA, newSHA, repoURL string) ([]storage.CommitInfo, error) {
	// Validate commit SHAs are not empty
	if oldSHA == "" {
		return nil, fmt.Errorf("old commit SHA cannot be empty")
	}
	if newSHA == "" {
		return nil, fmt.Errorf("new commit SHA cannot be empty")
	}

	ctx := context.Background()

	owner, repo := parseRepoURL(repoURL)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid repository URL: %s", repoURL)
	}

	comparison, _, err := c.github.Repositories.CompareCommits(ctx, owner, repo, oldSHA, newSHA, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to compare commits: %w", err)
	}

	var commits []storage.CommitInfo
	for _, commit := range comparison.Commits {
		var commitDate time.Time

		// Try Author date first, then Committer date
		if commit.Commit.Author != nil && commit.Commit.Author.Date != nil {
			commitDate = commit.Commit.Author.Date.Time
		} else if commit.Commit.Committer != nil && commit.Commit.Committer.Date != nil {
			commitDate = commit.Commit.Committer.Date.Time
		}
		// If both are nil, commitDate remains zero time

		commits = append(commits, storage.CommitInfo{
			SHA:       *commit.SHA,
			Message:   *commit.Commit.Message,
			RepoURL:   repoURL,
			CreatedAt: commitDate,
		})
	}

	return commits, nil
}

// GetCommitMessage retrieves the commit message for a given commit.
func (c *client) GetCommitMessage(commitSHA, repoURL string) string {
	// Validate commit SHA is not empty
	if commitSHA == "" {
		logger.Warnf("GetCommitMessage called with empty commit SHA")
		return ""
	}

	ctx := context.Background()

	owner, repo := parseRepoURL(repoURL)
	if owner == "" || repo == "" {
		return ""
	}

	commit, _, err := c.github.Repositories.GetCommit(ctx, owner, repo, commitSHA, nil)
	if err != nil {
		logger.Warnf("Failed to get commit message for %s: %v", commitSHA, err)
		return ""
	}

	return *commit.Commit.Message
}

// GetCommitDate retrieves the commit creation date for a given commit.
func (c *client) GetCommitDate(commitSHA, repoURL string) time.Time {
	// Validate commit SHA is not empty
	if commitSHA == "" {
		logger.Warnf("GetCommitDate called with empty commit SHA")
		return time.Time{}
	}

	ctx := context.Background()

	owner, repo := parseRepoURL(repoURL)
	if owner == "" || repo == "" {
		logger.Warnf("Failed to parse repo URL %s for commit %s", repoURL, commitSHA)
		return time.Time{}
	}

	commit, _, err := c.github.Repositories.GetCommit(ctx, owner, repo, commitSHA, nil)
	if err != nil {
		logger.Errorf("Failed to get commit date for %s in %s/%s: %v", commitSHA, owner, repo, err)
		return time.Time{}
	}

	// Check commit structure
	if commit.Commit == nil {
		logger.Errorf("Commit object has nil Commit field for %s", commitSHA)
		return time.Time{}
	}

	var commitDate time.Time

	// Try Author date first (when the commit was authored)
	if commit.Commit.Author != nil && commit.Commit.Author.Date != nil {
		commitDate = commit.Commit.Author.Date.Time
	} else if commit.Commit.Committer != nil && commit.Commit.Committer.Date != nil {
		// Fallback to Committer date (when the commit was committed)
		commitDate = commit.Commit.Committer.Date.Time
	} else {
		logger.Errorf("Both Author and Committer dates are nil for commit %s", commitSHA)
		return time.Time{}
	}
	return commitDate
}

// parseRepoURL extracts owner and repository name from a GitHub URL.
func parseRepoURL(url string) (string, string) {
	// Remove .git suffix if present
	url = strings.TrimSuffix(url, ".git")

	// Extract owner/repo from URL
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2], parts[len(parts)-1]
	}

	return "", ""
}
