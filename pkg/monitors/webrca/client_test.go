package webrca

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Removed simple constructor test - tests basic struct initialization without business logic

func TestClient_isTokenValid(t *testing.T) {
	tests := []struct {
		name        string
		accessToken string
		tokenExpiry time.Time
		expected    bool
	}{
		{
			name:        "valid token",
			accessToken: "valid-token",
			tokenExpiry: time.Now().Add(1 * time.Hour),
			expected:    true,
		},
		{
			name:        "expired token",
			accessToken: "expired-token",
			tokenExpiry: time.Now().Add(-1 * time.Hour),
			expected:    false,
		},
		{
			name:        "empty token",
			accessToken: "",
			tokenExpiry: time.Now().Add(1 * time.Hour),
			expected:    false,
		},
		{
			name:        "token expiring now",
			accessToken: "expiring-token",
			tokenExpiry: time.Now(),
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				accessToken: tt.accessToken,
				tokenExpiry: tt.tokenExpiry,
			}

			result := client.isTokenValid()
			assert.Equal(t, tt.expected, result, "Expected correct token validity")
		})
	}
}

func TestClient_cacheToken(t *testing.T) {
	client := &Client{}

	token := &TokenResponse{
		AccessToken: "test-access-token",
		ExpiresIn:   3600, // 1 hour
	}

	client.cacheToken(token)

	assert.Equal(t, "test-access-token", client.accessToken, "Expected access token to be cached")
	assert.True(t, client.tokenExpiry.After(time.Now()), "Expected token expiry to be in the future")
	assert.True(t, client.tokenExpiry.Before(time.Now().Add(1*time.Hour)), "Expected token expiry to be before full duration due to buffer")
}

func TestClient_ContextCancellation(t *testing.T) {
	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := NewClient("https://api.example.com/incidents", "test-token")

	incidents, err := client.GetAllIncidents(ctx)

	assert.Error(t, err, "Expected error due to cancelled context")
	assert.Nil(t, incidents, "Expected nil incidents")
}
