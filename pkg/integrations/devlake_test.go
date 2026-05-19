package integrations

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/redhat-appstudio/dora-metrics/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockIncident implements the IncidentData interface for testing
type mockIncident struct {
	incidentID    string
	summary       string
	description   string
	status        string
	createdAt     time.Time
	updatedAt     time.Time
	resolvedAt    *time.Time
	lastChangedAt *time.Time
	closedAt      *time.Time
	products      []string
}

func (m *mockIncident) GetIncidentID() string        { return m.incidentID }
func (m *mockIncident) GetSummary() string           { return m.summary }
func (m *mockIncident) GetDescription() string       { return m.description }
func (m *mockIncident) GetStatus() string            { return m.status }
func (m *mockIncident) GetCreatedAt() time.Time      { return m.createdAt }
func (m *mockIncident) GetUpdatedAt() time.Time      { return m.updatedAt }
func (m *mockIncident) GetResolvedAt() *time.Time    { return m.resolvedAt }
func (m *mockIncident) GetLastChangedAt() *time.Time { return m.lastChangedAt }
func (m *mockIncident) GetClosedAt() *time.Time      { return m.closedAt }
func (m *mockIncident) GetProducts() []string        { return m.products }

// Helper function to create a time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}

func TestNewDevLakeIntegration(t *testing.T) {
	tests := []struct {
		name           string
		baseURL        string
		projectID      string
		enabled        bool
		timeoutSeconds int
		expectedName   string
	}{
		{
			name:           "valid configuration",
			baseURL:        "https://devlake.example.com",
			projectID:      "test-project-123",
			enabled:        true,
			timeoutSeconds: 30,
			expectedName:   "devlake",
		},
		{
			name:           "disabled integration",
			baseURL:        "https://devlake.example.com",
			projectID:      "test-project-123",
			enabled:        false,
			timeoutSeconds: 30,
			expectedName:   "devlake",
		},
		{
			name:           "zero timeout uses default",
			baseURL:        "https://devlake.example.com",
			projectID:      "test-project-123",
			enabled:        true,
			timeoutSeconds: 0,
			expectedName:   "devlake",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integration := NewDevLakeIntegration(tt.baseURL, tt.projectID, tt.enabled, tt.timeoutSeconds, nil)
			assert.NotNil(t, integration)
			assert.Equal(t, tt.enabled, integration.enabled)
			assert.Equal(t, tt.expectedName, integration.name)
			assert.Equal(t, tt.baseURL, integration.baseURL)
			assert.Equal(t, tt.projectID, integration.projectID)
			assert.NotNil(t, integration.httpClient)

			// Verify timeout defaults to 30 if zero or negative
			if tt.timeoutSeconds <= 0 {
				assert.Equal(t, 30, integration.timeoutSeconds)
			} else {
				assert.Equal(t, tt.timeoutSeconds, integration.timeoutSeconds)
			}
		})
	}
}

func TestDevLakeIntegration_IsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
	}{
		{"enabled", true},
		{"disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integration := NewDevLakeIntegration("http://test", "proj-1", tt.enabled, 30, nil)
			assert.Equal(t, tt.enabled, integration.IsEnabled())
		})
	}
}

func TestDevLakeIntegration_FormatDevLakeDate(t *testing.T) {
	integration := NewDevLakeIntegration("http://test", "proj-1", true, 30, nil)

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "valid timestamp",
			time:     time.Date(2026, 3, 15, 14, 30, 45, 0, time.UTC),
			expected: "2026-03-15T14:30:45+00:00",
		},
		{
			name:     "zero time",
			time:     time.Time{},
			expected: "",
		},
		{
			name:     "timestamp with non-UTC timezone",
			time:     time.Date(2026, 3, 15, 14, 30, 45, 0, time.FixedZone("EST", -5*3600)),
			expected: "2026-03-15T19:30:45+00:00", // Converted to UTC
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := integration.FormatDevLakeDate(tt.time)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDevLakeIntegration_ResolutionDatePriority(t *testing.T) {
	// Test the resolution date priority logic: resolved_at > last_changed_at > updated_at
	now := time.Now().UTC()
	resolvedTime := now.Add(-2 * time.Hour)
	lastChangedTime := now.Add(-1 * time.Hour)
	updatedTime := now

	tests := []struct {
		name              string
		incident          *mockIncident
		expectedDateField string // which field should be used
		expectResDate     bool   // should ResolutionDate be set
	}{
		{
			name: "resolved_at takes priority",
			incident: &mockIncident{
				incidentID:    "ITN-2026-00001",
				summary:       "Test incident",
				description:   "Test description",
				status:        "resolved",
				createdAt:     now.Add(-3 * time.Hour),
				updatedAt:     updatedTime,
				resolvedAt:    timePtr(resolvedTime),
				lastChangedAt: timePtr(lastChangedTime),
				products:      []string{"konflux"},
			},
			expectedDateField: "resolved_at",
			expectResDate:     true,
		},
		{
			name: "last_changed_at when resolved_at is nil",
			incident: &mockIncident{
				incidentID:    "ITN-2026-00002",
				summary:       "Test incident",
				description:   "Test description",
				status:        "resolved",
				createdAt:     now.Add(-3 * time.Hour),
				updatedAt:     updatedTime,
				resolvedAt:    nil,
				lastChangedAt: timePtr(lastChangedTime),
				products:      []string{"konflux"},
			},
			expectedDateField: "last_changed_at",
			expectResDate:     true,
		},
		{
			name: "updated_at as last resort",
			incident: &mockIncident{
				incidentID:    "ITN-2026-00003",
				summary:       "Test incident",
				description:   "Test description",
				status:        "resolved",
				createdAt:     now.Add(-3 * time.Hour),
				updatedAt:     updatedTime,
				resolvedAt:    nil,
				lastChangedAt: nil,
				products:      []string{"konflux"},
			},
			expectedDateField: "updated_at",
			expectResDate:     true,
		},
		{
			name: "closed status uses same logic",
			incident: &mockIncident{
				incidentID:    "ITN-2026-00004",
				summary:       "Test incident",
				description:   "Test description",
				status:        "closed",
				createdAt:     now.Add(-3 * time.Hour),
				updatedAt:     updatedTime,
				resolvedAt:    timePtr(resolvedTime),
				lastChangedAt: timePtr(lastChangedTime),
				products:      []string{"konflux"},
			},
			expectedDateField: "resolved_at",
			expectResDate:     true,
		},
		{
			name: "open incident has no resolution date",
			incident: &mockIncident{
				incidentID:    "ITN-2026-00005",
				summary:       "Test incident",
				description:   "Test description",
				status:        "open",
				createdAt:     now.Add(-3 * time.Hour),
				updatedAt:     updatedTime,
				resolvedAt:    nil,
				lastChangedAt: timePtr(lastChangedTime),
				products:      []string{"konflux"},
			},
			expectedDateField: "",
			expectResDate:     false,
		},
		{
			name: "non-konflux incident is skipped",
			incident: &mockIncident{
				incidentID:    "ITN-2026-00006",
				summary:       "Test incident",
				description:   "Test description",
				status:        "resolved",
				createdAt:     now.Add(-3 * time.Hour),
				updatedAt:     updatedTime,
				resolvedAt:    timePtr(resolvedTime),
				lastChangedAt: timePtr(lastChangedTime),
				products:      []string{"ROSA Classic"},
			},
			expectedDateField: "",
			expectResDate:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server to capture the request
			var capturedPayload DevLakeIssue
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				json.Unmarshal(body, &capturedPayload)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			// Set environment variable for token
			t.Setenv("DEVLAKE_WEBHOOK_TOKEN", "test-token-123")

			integration := NewDevLakeIntegration(server.URL, "test-project", true, 30, nil)
			ctx := context.Background()

			err := integration.SendIncidentEvent(ctx, tt.incident, 0)

			// Non-Konflux incidents should return nil without error (skipped)
			if !strings.Contains(strings.Join(tt.incident.products, ","), "konflux") {
				assert.NoError(t, err)
				return
			}

			require.NoError(t, err)

			// Verify resolution date handling
			if tt.expectResDate {
				assert.NotEmpty(t, capturedPayload.ResolutionDate, "ResolutionDate should be set")

				// Verify the correct field was used based on priority
				switch tt.expectedDateField {
				case "resolved_at":
					expectedDate := integration.FormatDevLakeDate(*tt.incident.resolvedAt)
					assert.Equal(t, expectedDate, capturedPayload.ResolutionDate)
				case "last_changed_at":
					expectedDate := integration.FormatDevLakeDate(*tt.incident.lastChangedAt)
					assert.Equal(t, expectedDate, capturedPayload.ResolutionDate)
				case "updated_at":
					expectedDate := integration.FormatDevLakeDate(tt.incident.updatedAt)
					assert.Equal(t, expectedDate, capturedPayload.ResolutionDate)
				}
			} else {
				assert.Empty(t, capturedPayload.ResolutionDate, "ResolutionDate should not be set for open incidents")
			}

			// Verify other fields
			assert.Equal(t, tt.incident.incidentID, capturedPayload.IssueKey)
			assert.Equal(t, tt.incident.summary, capturedPayload.Title)
			assert.Equal(t, "INCIDENT", capturedPayload.Type)
		})
	}
}

func TestDevLakeIntegration_GetTeamsForComponent(t *testing.T) {
	teams := []config.TeamConfig{
		{
			Name:             "team-a",
			ProjectID:        "proj-a",
			ArgocdComponents: []string{"component-1", "component-2"},
		},
		{
			Name:             "team-b",
			ProjectID:        "proj-b",
			ArgocdComponents: []string{"component-2", "component-3"},
		},
		{
			Name:             "team-c",
			ProjectID:        "proj-c",
			ArgocdComponents: []string{"component-4"},
		},
	}

	integration := NewDevLakeIntegration("http://test", "proj-1", true, 30, teams)

	tests := []struct {
		name          string
		component     string
		expectedTeams []string
	}{
		{
			name:          "component in one team",
			component:     "component-1",
			expectedTeams: []string{"team-a"},
		},
		{
			name:          "component in multiple teams",
			component:     "component-2",
			expectedTeams: []string{"team-a", "team-b"},
		},
		{
			name:          "component not found",
			component:     "non-existent",
			expectedTeams: []string{},
		},
		{
			name:          "empty component",
			component:     "",
			expectedTeams: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := integration.GetTeamsForComponent(tt.component)

			if len(tt.expectedTeams) == 0 {
				assert.Nil(t, result)
			} else {
				require.Len(t, result, len(tt.expectedTeams))
				for i, expectedTeam := range tt.expectedTeams {
					assert.Equal(t, expectedTeam, result[i].Name)
				}
			}
		})
	}
}

func TestDevLakeIntegration_ExtractComponentFromDisplayTitle(t *testing.T) {
	integration := NewDevLakeIntegration("http://test", "proj-1", true, 30, nil)

	tests := []struct {
		name         string
		displayTitle *string
		expected     string
	}{
		{
			name:         "new format",
			displayTitle: stringPtr("ArgoCD Deployment | Component: my-component | Cluster: prod | Environment: production | Revision: abc123 | Commits: 5 | Status: success | Deployed: 2026-03-15"),
			expected:     "my-component",
		},
		{
			name:         "old format",
			displayTitle: stringPtr("Production Deployment component: legacy-component, revision abc123 (2026-03-15)"),
			expected:     "legacy-component",
		},
		{
			name:         "nil display title",
			displayTitle: nil,
			expected:     "",
		},
		{
			name:         "empty display title",
			displayTitle: stringPtr(""),
			expected:     "",
		},
		{
			name:         "no component in title",
			displayTitle: stringPtr("Some random deployment title"),
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := integration.extractComponentFromDisplayTitle(tt.displayTitle)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDevLakeIntegration_MapToDevLakeStatus(t *testing.T) {
	integration := NewDevLakeIntegration("http://test", "proj-1", true, 30, nil)

	tests := []struct {
		name               string
		webrcaStatus       string
		isResolved         bool
		expectedStatus     string
		expectedOrigStatus string
	}{
		{
			name:               "resolved status",
			webrcaStatus:       "resolved",
			isResolved:         true,
			expectedStatus:     "DONE",
			expectedOrigStatus: "resolved",
		},
		{
			name:               "closed status",
			webrcaStatus:       "closed",
			isResolved:         true,
			expectedStatus:     "DONE",
			expectedOrigStatus: "closed",
		},
		{
			name:               "open status",
			webrcaStatus:       "open",
			isResolved:         false,
			expectedStatus:     "TODO",
			expectedOrigStatus: "open",
		},
		{
			name:               "investigating status",
			webrcaStatus:       "investigating",
			isResolved:         false,
			expectedStatus:     "TODO",
			expectedOrigStatus: "open",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, origStatus := integration.mapToDevLakeStatus(tt.webrcaStatus, tt.isResolved)
			assert.Equal(t, tt.expectedStatus, status)
			assert.Equal(t, tt.expectedOrigStatus, origStatus)
		})
	}
}

func TestDevLakeIntegration_SendIncidentEvent_Disabled(t *testing.T) {
	integration := NewDevLakeIntegration("http://test", "proj-1", false, 30, nil)

	incident := &mockIncident{
		incidentID: "ITN-2026-00001",
		products:   []string{"konflux"},
	}

	err := integration.SendIncidentEvent(context.Background(), incident, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestDevLakeIntegration_SendIncidentEvent_MissingToken(t *testing.T) {
	integration := NewDevLakeIntegration("http://test", "proj-1", true, 30, nil)

	incident := &mockIncident{
		incidentID: "ITN-2026-00001",
		summary:    "Test",
		status:     "open",
		createdAt:  time.Now(),
		updatedAt:  time.Now(),
		products:   []string{"konflux"},
	}

	// Ensure token env var is not set
	t.Setenv("DEVLAKE_WEBHOOK_TOKEN", "")

	err := integration.SendIncidentEvent(context.Background(), incident, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token")
}

// Helper function to create a string pointer
func stringPtr(s string) *string {
	return &s
}
