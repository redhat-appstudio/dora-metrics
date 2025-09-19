package webrca

import (
	"context"
	"net/http"
	"time"
)

// CreateTestIncident creates a test incident with default values
func CreateTestIncident(incidentID, summary string, products []string) *Incident {
	now := time.Now()
	return &Incident{
		ID:           "test-id-" + incidentID,
		Kind:         "Incident",
		Href:         "https://api.example.com/incidents/" + incidentID,
		IncidentID:   incidentID,
		Summary:      summary,
		Description:  "Test incident description for " + incidentID,
		Products:     products,
		IncidentType: "test",
		Status:       "open",
		Severity:     "1",
		Creator: User{
			ID:        "test-user",
			Kind:      "User",
			Href:      "https://api.example.com/users/test-user",
			Name:      "Test User",
			Email:     "test@example.com",
			Username:  "testuser",
			CreatedAt: now,
			UpdatedAt: now,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// CreateTestIncidentList creates a test incident list with the provided incidents
func CreateTestIncidentList(incidents []Incident) *IncidentList {
	return &IncidentList{
		Kind:  "IncidentList",
		Page:  1,
		Size:  len(incidents),
		Total: len(incidents),
		Items: incidents,
	}
}

// CreateTestContext creates a test context with timeout
func CreateTestContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// CreateTestMonitor creates a test monitor with mock dependencies
func CreateTestMonitor() *Monitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Monitor{
		incidents: &Incidents{},
		interval:  1 * time.Minute,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// CreateTestClient creates a test client with default configuration
func CreateTestClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		baseURL:      "https://api.example.com/incidents",
		offlineToken: "test-offline-token",
	}
}

// CreateTestIncidents creates a test incidents handler
func CreateTestIncidents() *Incidents {
	return &Incidents{
		client: CreateTestClient(),
	}
}
