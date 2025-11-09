// Package parser provides DevLake payload formatting for ArgoCD deployments.
// It creates structured payloads that can be sent to DevLake for DORA metrics
// collection and analysis.
package parser

import (
	"context"
	"fmt"
	"time"

	"github.com/redhat-appstudio/dora-metrics/pkg/integrations"
	"github.com/redhat-appstudio/dora-metrics/pkg/logger"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/api"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd/github"
	"github.com/redhat-appstudio/dora-metrics/pkg/storage"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// Formatter handles DevLake payload formatting.
// It converts ArgoCD application information and commit history into
// DevLake-compatible deployment payloads.
type Formatter struct {
	githubClient github.Client
	storage      *storage.RedisClient
	devlake      *integrations.DevLakeIntegration
}

// NewFormatter creates a new DevLake formatter instance.
func NewFormatter(githubClient github.Client, storage *storage.RedisClient) *Formatter {
	// Create a minimal DevLake integration instance for formatting only
	// This is not used for sending, only for date formatting
	devlake := integrations.NewDevLakeIntegration("", "", false, 30, nil)
	return &Formatter{
		githubClient: githubClient,
		storage:      storage,
		devlake:      devlake,
	}
}

// FormatDeployment creates a DevLake deployment payload from application information.
// Returns the deployment and a boolean indicating whether there are commits to include.
func (f *Formatter) FormatDeployment(
	app *v1alpha1.Application,
	appInfo *api.ApplicationInfo,
	deployedAt time.Time,
	commits []storage.CommitInfo,
) (integrations.DevLakeCICDDeployment, bool) {
	deploymentID := appInfo.Revision
	repoURL := f.getRepoURLFromHistory(app, deploymentID)
	infraCommitMsg := f.getCommitMessageFromGitHub(deploymentID)

	devlakeCommits := f.createDevLakeCommits(commits, deployedAt, repoURL, deploymentID, infraCommitMsg, appInfo.Component)

	// If no commits to include, return empty deployment and false
	if len(devlakeCommits) == 0 {
		return integrations.DevLakeCICDDeployment{}, false
	}

	result := f.determineResult(app)

	componentName := appInfo.Component
	if componentName == "" {
		componentName = app.Name
	}

	// Create a meaningful DisplayTitle for AI agents with structured information
	// Format: "ArgoCD Deployment | Component: {component} | Namespace: {namespace} | Revision: {revision} | Status: {result} | Deployed: {timestamp}"
	namespace := appInfo.Namespace
	if namespace == "" {
		namespace = app.Namespace
	}
	displayTitle := fmt.Sprintf("ArgoCD Deployment | Component: %s | Namespace: %s | Revision: %s | Status: %s | Deployed: %s",
		componentName,
		namespace,
		deploymentID,
		result,
		deployedAt.Format("2006-01-02 15:04:05 MST"))
	name := fmt.Sprintf("deploy to production %s", deploymentID)

	// Calculate proper timeline
	startedDate, finishedDate := f.calculateTimeline(devlakeCommits, deployedAt)

	// Format dates using DevLake format
	createdDateStr := f.devlake.FormatDevLakeDate(deployedAt)
	startedDateStr := f.devlake.FormatDevLakeDate(startedDate)
	finishedDateStr := f.devlake.FormatDevLakeDate(finishedDate)

	return integrations.DevLakeCICDDeployment{
		ID:                deploymentID,
		CreatedDate:       &createdDateStr,
		StartedDate:       startedDateStr,
		FinishedDate:      finishedDateStr,
		Environment:       "PRODUCTION",
		Result:            result,
		DisplayTitle:      &displayTitle,
		Name:              &name,
		DeploymentCommits: devlakeCommits,
	}, true
}

// createDevLakeCommits creates DevLake commit entries from commit history.
func (f *Formatter) createDevLakeCommits(
	commits []storage.CommitInfo,
	deployedAt time.Time,
	repoURL, deploymentID, infraCommitMsg, component string,
) []integrations.DevLakeDeploymentCommit {
	devlakeCommits := make([]integrations.DevLakeDeploymentCommit, 0) // Initialize as empty slice, not nil

	// Add all commits (including infra-deployments commit which is already in the commits slice)
	for _, commit := range commits {
		// Check if this commit has already been sent to DevLake for this component
		if f.storage != nil {
			alreadySent, err := f.storage.IsDevLakeCommitProcessed(context.Background(), commit.SHA, component)
			if err != nil {
				logger.Warnf("Failed to check if commit %s was already sent to DevLake for component %s: %v", commit.SHA, component, err)
			} else if alreadySent {
				continue
			}
		}

		displayTitle := commit.Message
		name := commit.Message

		// Use the commit's repository URL if available, otherwise fall back to main repo URL
		commitRepoURL := commit.RepoURL
		if commitRepoURL == "" {
			commitRepoURL = repoURL
		}

		// Use commit creation date as StartedDate, deployment time as FinishedDate
		// This is REQUIRED for DevLake compliance - we must have the real commit date
		startedDate := commit.CreatedAt
		if startedDate.IsZero() {
			logger.Errorf("CRITICAL: Commit %s has zero CreatedAt - this violates DevLake requirements", commit.SHA)
			// Don't use fallback - we need the real commit date
			continue // Skip this commit if we don't have its creation date
		}
		logger.Infof("Using commit creation date for %s: StartedDate=%v, FinishedDate=%v", commit.SHA, startedDate, deployedAt)

		// Format dates using DevLake format
		startedDateStr := f.devlake.FormatDevLakeDate(startedDate)
		finishedDateStr := f.devlake.FormatDevLakeDate(deployedAt)

		devlakeCommits = append(devlakeCommits, integrations.DevLakeDeploymentCommit{
			RepoURL:      commitRepoURL,
			RefName:      commit.SHA,
			StartedDate:  startedDateStr,
			FinishedDate: finishedDateStr,
			CommitSHA:    commit.SHA,
			CommitMsg:    commit.Message,
			Result:       "SUCCESS",
			DisplayTitle: &displayTitle,
			Name:         &name,
		})

		// Mark this commit as sent to DevLake for this component
		if f.storage != nil {
			if err := f.storage.MarkDevLakeCommitAsProcessed(context.Background(), commit.SHA, component); err != nil {
				logger.Errorf("Failed to mark commit %s as sent to DevLake for component %s: %v", commit.SHA, component, err)
			}
		}
	}

	return devlakeCommits
}

// calculateTimeline calculates the proper StartedDate and FinishedDate for a deployment.
func (f *Formatter) calculateTimeline(devlakeCommits []integrations.DevLakeDeploymentCommit, deployedAt time.Time) (time.Time, time.Time) {
	if len(devlakeCommits) == 0 {
		return deployedAt, deployedAt
	}

	// Find the earliest StartedDate from all commits
	var earliestStarted time.Time
	hasStartedDate := false

	for _, commit := range devlakeCommits {
		// Parse the StartedDate string back to time.Time for comparison
		if startedDate, err := time.Parse("2006-01-02T15:04:05+00:00", commit.StartedDate); err == nil && !startedDate.IsZero() {
			if !hasStartedDate || startedDate.Before(earliestStarted) {
				earliestStarted = startedDate
				hasStartedDate = true
			}
		}
	}

	if !hasStartedDate {
		earliestStarted = deployedAt
	}

	return earliestStarted, deployedAt
}

// determineResult determines the deployment result based on application health.
// Defaults to SUCCESS unless the application is explicitly unhealthy.
func (f *Formatter) determineResult(app *v1alpha1.Application) string {
	if app.Status.Health.Status == "Unhealthy" {
		return "FAILED"
	}
	return "SUCCESS"
}

// getRepoURLFromHistory extracts repository URL from ArgoCD application history.
func (f *Formatter) getRepoURLFromHistory(app *v1alpha1.Application, commitSHA string) string {
	for _, historyItem := range app.Status.History {
		if historyItem.Revision == commitSHA && historyItem.Source.RepoURL != "" {
			return historyItem.Source.RepoURL
		}
	}
	return ""
}

// getCommitMessageFromGitHub retrieves commit message from GitHub.
func (f *Formatter) getCommitMessageFromGitHub(commitSHA string) string {
	// Validate commit SHA is not empty
	if commitSHA == "" {
		return "Commit (unknown)"
	}

	if f.githubClient == nil {
		if len(commitSHA) >= 8 {
			return fmt.Sprintf("Commit %s", commitSHA[:8])
		}
		return fmt.Sprintf("Commit %s", commitSHA)
	}

	// Try to find the repository for this commit
	repoURL, err := f.githubClient.FindRepositoryForCommit(commitSHA)
	if err != nil {
		if len(commitSHA) >= 8 {
			return fmt.Sprintf("Commit %s", commitSHA[:8])
		}
		return fmt.Sprintf("Commit %s", commitSHA)
	}

	// Get the commit message
	commitMsg := f.githubClient.GetCommitMessage(commitSHA, repoURL)
	if commitMsg == "" {
		if len(commitSHA) >= 8 {
			return fmt.Sprintf("Commit %s", commitSHA[:8])
		}
		return fmt.Sprintf("Commit %s", commitSHA)
	}

	return commitMsg
}
