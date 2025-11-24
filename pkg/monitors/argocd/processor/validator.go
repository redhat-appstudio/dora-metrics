// Package processor provides ArgoCD event processing functionality.
package processor

import (
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// AppValidator validates ArgoCD application state before processing.
type AppValidator struct{}

// NewAppValidator creates a new application validator.
func NewAppValidator() *AppValidator {
	return &AppValidator{}
}

// ShouldProcess checks if an application should be processed.
// Returns true if the application meets all processing criteria.
// REQUIRES: Healthy status, Synced status, and valid revision.
func (v *AppValidator) ShouldProcess(app *v1alpha1.Application, syncRevision string) bool {
	return v.isHealthy(app) && v.isSynced(app) && v.hasRevision(syncRevision)
}

// isHealthy checks if the application health status is acceptable for processing.
// Accepts "Healthy" and "Unknown" statuses - rejects "Degraded", "Missing", etc.
func (v *AppValidator) isHealthy(app *v1alpha1.Application) bool {
	status := app.Status.Health.Status
	return status == "Healthy" || status == "Unknown"
}

// isSynced checks if the application sync status is acceptable for processing.
// Accepts "Synced" and "Unknown" statuses - rejects "OutOfSync" and other statuses.
func (v *AppValidator) isSynced(app *v1alpha1.Application) bool {
	status := app.Status.Sync.Status
	return status == "Synced" || status == "Unknown"
}

// hasRevision checks if the revision is not empty.
func (v *AppValidator) hasRevision(revision string) bool {
	return revision != ""
}

// IsRevisionInHistory checks if the sync revision exists anywhere in the deployment history.
// Returns true if the revision is found in any history entry, false otherwise.
func (v *AppValidator) IsRevisionInHistory(app *v1alpha1.Application, syncRevision string) bool {
	if len(app.Status.History) == 0 {
		return false
	}
	// Check if revision exists anywhere in history
	for _, historyItem := range app.Status.History {
		if historyItem.Revision == syncRevision {
			return true
		}
	}
	return false
}

// GetDeployedTimestamp extracts the deployed timestamp for a revision from history.
func (v *AppValidator) GetDeployedTimestamp(app *v1alpha1.Application, revision string) time.Time {
	for _, historyItem := range app.Status.History {
		if historyItem.Revision == revision && !historyItem.DeployedAt.IsZero() {
			return historyItem.DeployedAt.Time
		}
	}
	return time.Time{}
}
