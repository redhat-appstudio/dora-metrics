package webrca

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Removed redundant integration tests that use mocked servers - these are better covered by unit tests

func TestClient_Integration_Pagination(t *testing.T) {
	// Create test server that handles pagination
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock incidents endpoint with pagination
		page := r.URL.Query().Get("page")
		incidentList := IncidentList{
			Kind:  "IncidentList",
			Page:  1,
			Size:  100,
			Total: 3,
		}

		if page == "1" {
			// Return exactly DefaultPageSize items to trigger pagination
			incidentList.Items = make([]Incident, 100)
			for i := 0; i < 100; i++ {
				incidentList.Items[i] = Incident{
					IncidentID: fmt.Sprintf("ITN-%03d", i+1),
					Summary:    fmt.Sprintf("Test incident %d", i+1),
					Products:   []string{"konflux"},
					Status:     "open",
					CreatedAt:  time.Now(),
					UpdatedAt:  time.Now(),
				}
			}
		} else if page == "2" {
			// Return the remaining items (less than DefaultPageSize to stop pagination)
			incidentList.Items = []Incident{
				{
					IncidentID: "ITN-101",
					Summary:    "Test incident 101",
					Products:   []string{"konflux"},
					Status:     "open",
					CreatedAt:  time.Now(),
					UpdatedAt:  time.Now(),
				},
				{
					IncidentID: "ITN-102",
					Summary:    "Test incident 102",
					Products:   []string{"konflux"},
					Status:     "open",
					CreatedAt:  time.Now(),
					UpdatedAt:  time.Now(),
				},
			}
		} else {
			incidentList.Items = []Incident{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(incidentList)
	}))
	defer server.Close()

	client := &Client{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		baseURL:      server.URL + "/incidents",
		offlineToken: "test-token",
		accessToken:  "test-access-token",
		tokenExpiry:  time.Now().Add(1 * time.Hour),
	}

	ctx := context.Background()
	incidents, err := client.GetAllIncidents(ctx)

	assert.NoError(t, err, "Expected no error")
	assert.Len(t, incidents, 102, "Expected 102 incidents from pagination")
	assert.Equal(t, "ITN-001", incidents[0].IncidentID, "Expected correct incident ID")
	assert.Equal(t, "ITN-002", incidents[1].IncidentID, "Expected correct incident ID")
	assert.Equal(t, "ITN-101", incidents[100].IncidentID, "Expected correct incident ID from page 2")
	assert.Equal(t, "ITN-102", incidents[101].IncidentID, "Expected correct incident ID from page 2")
}

// Removed redundant monitor integration test - this is better covered by unit tests
