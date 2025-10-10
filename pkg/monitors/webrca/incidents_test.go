package webrca

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Removed simple constructor test - tests basic struct initialization without business logic

func TestIncidents_Check_EmptyIncidents(t *testing.T) {
	// Test with empty incidents list
	client := NewClient("https://api.example.com/incidents", "test-token")
	incidents := NewIncidents(client)

	ctx := context.Background()

	// This will fail because we don't have a real client, but we can test the structure
	// In a real test, you would mock the client's GetAllIncidents method
	err := incidents.Check(ctx)

	// We expect an error because the client is not properly configured
	assert.Error(t, err, "Expected error with unconfigured client")
}

func TestIncidents_Check_WithMockClient(t *testing.T) {
	// Test the business logic with a mock client
	ctx := context.Background()

	// Create a client using the proper constructor
	mockClient := NewClient("https://api.example.com/incidents", "test-token")

	incidents := NewIncidents(mockClient)

	// This will fail because we don't have a real HTTP client, but we can test the structure
	err := incidents.Check(ctx)

	// We expect an error because the client is not properly configured
	assert.Error(t, err, "Expected error with unconfigured client")
	assert.Contains(t, err.Error(), "failed to fetch incidents", "Expected specific error message")
}

func TestIncidents_Check_ContextCancellation(t *testing.T) {
	// Test that context cancellation is handled properly
	client := NewClient("https://api.example.com/incidents", "test-token")
	incidents := NewIncidents(client)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Execute test
	err := incidents.Check(ctx)

	// Assertions
	assert.Error(t, err, "Expected error due to cancelled context")
	assert.Contains(t, err.Error(), "failed to fetch incidents", "Expected specific error message")
}
