package webrca

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIncident_GetIncidentID(t *testing.T) {
	tests := []struct {
		name       string
		incidentID string
		expected   string
	}{
		{
			name:       "valid incident ID",
			incidentID: "ITN-2025-00123",
			expected:   "ITN-2025-00123",
		},
		{
			name:       "empty incident ID",
			incidentID: "",
			expected:   "",
		},
		{
			name:       "numeric incident ID",
			incidentID: "12345",
			expected:   "12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			incident := &Incident{
				IncidentID: tt.incidentID,
			}
			result := incident.GetIncidentID()
			assert.Equal(t, tt.expected, result, "Expected correct incident ID")
		})
	}
}

func TestIncident_GetSummary(t *testing.T) {
	tests := []struct {
		name     string
		summary  string
		expected string
	}{
		{
			name:     "valid summary",
			summary:  "Database connection timeout",
			expected: "Database connection timeout",
		},
		{
			name:     "empty summary",
			summary:  "",
			expected: "",
		},
		{
			name:     "long summary",
			summary:  "This is a very long summary that describes the incident in detail with multiple sentences and various information",
			expected: "This is a very long summary that describes the incident in detail with multiple sentences and various information",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			incident := &Incident{
				Summary: tt.summary,
			}
			result := incident.GetSummary()
			assert.Equal(t, tt.expected, result, "Expected correct summary")
		})
	}
}

func TestIncident_GetDescription(t *testing.T) {
	tests := []struct {
		name        string
		description string
		expected    string
	}{
		{
			name:        "valid description",
			description: "The application is experiencing intermittent database connection timeouts",
			expected:    "The application is experiencing intermittent database connection timeouts",
		},
		{
			name:        "empty description",
			description: "",
			expected:    "",
		},
		{
			name:        "multiline description",
			description: "Issue Description:\n- Database connections are timing out\n- Affects user login functionality\n- Started at 14:30 UTC",
			expected:    "Issue Description:\n- Database connections are timing out\n- Affects user login functionality\n- Started at 14:30 UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			incident := &Incident{
				Description: tt.description,
			}
			result := incident.GetDescription()
			assert.Equal(t, tt.expected, result, "Expected correct description")
		})
	}
}

func TestIncident_GetStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected string
	}{
		{
			name:     "open status",
			status:   "open",
			expected: "open",
		},
		{
			name:     "in_progress status",
			status:   "in_progress",
			expected: "in_progress",
		},
		{
			name:     "resolved status",
			status:   "resolved",
			expected: "resolved",
		},
		{
			name:     "closed status",
			status:   "closed",
			expected: "closed",
		},
		{
			name:     "empty status",
			status:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			incident := &Incident{
				Status: tt.status,
			}
			result := incident.GetStatus()
			assert.Equal(t, tt.expected, result, "Expected correct status")
		})
	}
}

func TestIncident_GetCreatedAt(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		createdAt time.Time
		expected  time.Time
	}{
		{
			name:      "valid created time",
			createdAt: now,
			expected:  now,
		},
		{
			name:      "zero time",
			createdAt: time.Time{},
			expected:  time.Time{},
		},
		{
			name:      "past time",
			createdAt: now.Add(-24 * time.Hour),
			expected:  now.Add(-24 * time.Hour),
		},
		{
			name:      "future time",
			createdAt: now.Add(24 * time.Hour),
			expected:  now.Add(24 * time.Hour),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			incident := &Incident{
				CreatedAt: tt.createdAt,
			}
			result := incident.GetCreatedAt()
			assert.Equal(t, tt.expected, result, "Expected correct created time")
		})
	}
}

func TestIncident_GetUpdatedAt(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		updatedAt time.Time
		expected  time.Time
	}{
		{
			name:      "valid updated time",
			updatedAt: now,
			expected:  now,
		},
		{
			name:      "zero time",
			updatedAt: time.Time{},
			expected:  time.Time{},
		},
		{
			name:      "past time",
			updatedAt: now.Add(-1 * time.Hour),
			expected:  now.Add(-1 * time.Hour),
		},
		{
			name:      "future time",
			updatedAt: now.Add(1 * time.Hour),
			expected:  now.Add(1 * time.Hour),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			incident := &Incident{
				UpdatedAt: tt.updatedAt,
			}
			result := incident.GetUpdatedAt()
			assert.Equal(t, tt.expected, result, "Expected correct updated time")
		})
	}
}

func TestIncident_GetterMethods_WithCompleteData(t *testing.T) {
	// Test all getter methods with a complete incident
	now := time.Now()
	incident := &Incident{
		IncidentID:  "ITN-2025-00123",
		Summary:     "Database connection timeout",
		Description: "The application is experiencing intermittent database connection timeouts",
		Status:      "open",
		CreatedAt:   now.Add(-2 * time.Hour),
		UpdatedAt:   now.Add(-1 * time.Hour),
	}

	// Test all getter methods
	assert.Equal(t, "ITN-2025-00123", incident.GetIncidentID(), "Expected correct incident ID")
	assert.Equal(t, "Database connection timeout", incident.GetSummary(), "Expected correct summary")
	assert.Equal(t, "The application is experiencing intermittent database connection timeouts", incident.GetDescription(), "Expected correct description")
	assert.Equal(t, "open", incident.GetStatus(), "Expected correct status")
	assert.Equal(t, now.Add(-2*time.Hour), incident.GetCreatedAt(), "Expected correct created time")
	assert.Equal(t, now.Add(-1*time.Hour), incident.GetUpdatedAt(), "Expected correct updated time")
}
