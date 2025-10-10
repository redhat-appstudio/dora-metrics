package webrca

import (
	"context"
	"fmt"
	"sync"
	"time"

	integrations "github.com/redhat-appstudio/dora-metrics/pkg/integrations"
	"github.com/redhat-appstudio/dora-metrics/pkg/logger"
)

// IncidentState tracks the state of an incident for deduplication and status change detection
type IncidentState struct {
	IncidentID string
	Status     string
	UpdatedAt  time.Time
	Processed  bool
}

// Incidents handles WebRCA incident monitoring business logic.
// It provides high-level operations for fetching and processing incidents.
type Incidents struct {
	client        *Client
	incidentState map[string]*IncidentState
	mu            sync.RWMutex
}

// NewIncidents creates a new WebRCA incidents handler with the provided client.
// It initializes the incidents processor for monitoring operations.
func NewIncidents(client *Client) *Incidents {
	return &Incidents{
		client:        client,
		incidentState: make(map[string]*IncidentState),
	}
}

// Check performs a complete incident check and filtering operation.
// It fetches all incidents from the WebRCA API, filters for Konflux-related incidents,
// and intelligently sends only new incidents or status changes to DevLake integration.
func (i *Incidents) Check(ctx context.Context) error {
	start := time.Now()

	incidents, err := i.client.GetAllIncidents(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch incidents: %w", err)
	}

	duration := time.Since(start)
	logger.Debugf("WebRCA incident check completed: %d total items in %v", len(incidents), duration)

	// Filter incidents by konflux product and process state changes
	konfluxIncidents := 0
	newIncidents := 0
	statusChanges := 0
	resolvedIncidents := 0

	for _, incident := range incidents {
		if incident.IsKonfluxIncident() {
			konfluxIncidents++

			// Check if this is a new incident or status change
			isNew, isStatusChange, isResolved := i.processIncident(ctx, &incident)

			if isNew {
				newIncidents++
			}
			if isStatusChange {
				statusChanges++
			}
			if isResolved {
				resolvedIncidents++
			}
		}
	}

	// Only log if there are significant changes
	if newIncidents > 0 || statusChanges > 0 || resolvedIncidents > 0 {
		logger.Infof("Konflux incidents processed: %d total (New: %d, Status changes: %d, Resolved: %d)",
			konfluxIncidents, newIncidents, statusChanges, resolvedIncidents)
	} else {
		logger.Debugf("Konflux incidents: %d out of %d total incidents (No changes)", konfluxIncidents, len(incidents))
	}

	return nil
}

// processIncident processes a single incident and determines if it should be sent to DevLake
// Returns: (isNew, isStatusChange, isResolved)
func (i *Incidents) processIncident(ctx context.Context, incident *Incident) (bool, bool, bool) {
	i.mu.Lock()
	defer i.mu.Unlock()

	incidentID := incident.GetIncidentID()
	currentStatus := incident.GetStatus()
	currentUpdatedAt := incident.GetUpdatedAt()

	// Get previous state
	prevState, exists := i.incidentState[incidentID]

	// Determine if this is a new incident
	isNew := !exists || !prevState.Processed

	// Determine if status changed
	isStatusChange := exists && prevState.Status != currentStatus

	// Determine if incident is resolved
	isResolved := incident.IsResolved()

	// Determine if we should send to DevLake
	shouldSend := isNew || isStatusChange

	if shouldSend {
		// Send to DevLake integration
		if err := integrations.GetManager().SendIncidentEventToDevLake(ctx, incident, 0); err != nil {
			logger.Errorf("Failed to send incident %s to DevLake: %v", incidentID, err)
		} else {
			// Only log important status changes at Info level
			if isNew {
				logger.Infof("New incident sent to DevLake: %s (Status: %s)", incidentID, currentStatus)
			} else if isStatusChange {
				logger.Infof("Incident status change sent to DevLake: %s (%s -> %s)", incidentID, prevState.Status, currentStatus)
			} else {
				logger.Debugf("Incident sent to DevLake: %s (Status: %s)", incidentID, currentStatus)
			}
		}
	}

	// If incident is resolved, try to close it in DevLake
	if isResolved && exists && prevState.Status != currentStatus {
		if err := integrations.GetManager().CloseIncidentInDevLake(ctx, incidentID); err != nil {
			logger.Errorf("Failed to close incident %s in DevLake: %v", incidentID, err)
		} else {
			logger.Infof("Incident resolved and closed in DevLake: %s", incidentID)
		}
	}

	// Update state tracking
	i.incidentState[incidentID] = &IncidentState{
		IncidentID: incidentID,
		Status:     currentStatus,
		UpdatedAt:  currentUpdatedAt,
		Processed:  true,
	}

	return isNew, isStatusChange, isResolved
}
