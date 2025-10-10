package webrca

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewMonitor(t *testing.T) {
	tests := []struct {
		name         string
		apiURL       string
		offlineToken string
		interval     time.Duration
		expectNil    bool
	}{
		{
			name:         "valid configuration",
			apiURL:       "https://api.example.com/incidents",
			offlineToken: "valid-token",
			interval:     5 * time.Minute,
			expectNil:    false,
		},
		{
			name:         "empty offline token",
			apiURL:       "https://api.example.com/incidents",
			offlineToken: "",
			interval:     5 * time.Minute,
			expectNil:    true,
		},
		{
			name:         "zero interval uses default",
			apiURL:       "https://api.example.com/incidents",
			offlineToken: "valid-token",
			interval:     0,
			expectNil:    false,
		},
		{
			name:         "negative interval uses default",
			apiURL:       "https://api.example.com/incidents",
			offlineToken: "valid-token",
			interval:     -1 * time.Minute,
			expectNil:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor := NewMonitor(tt.apiURL, tt.offlineToken, tt.interval)

			if tt.expectNil {
				assert.Nil(t, monitor, "Expected monitor to be nil")
			} else {
				assert.NotNil(t, monitor, "Expected monitor to not be nil")
				if monitor != nil {
					assert.NotNil(t, monitor.incidents, "Expected incidents to be initialized")
					assert.NotNil(t, monitor.ctx, "Expected context to be initialized")
					assert.NotNil(t, monitor.cancel, "Expected cancel function to be initialized")

					if tt.interval <= 0 {
						assert.Equal(t, DefaultCheckInterval, monitor.interval, "Expected default interval")
					} else {
						assert.Equal(t, tt.interval, monitor.interval, "Expected custom interval")
					}
				}
			}
		})
	}
}

func TestMonitor_Stop(t *testing.T) {
	tests := []struct {
		name    string
		monitor *Monitor
	}{
		{
			name:    "nil monitor",
			monitor: nil,
		},
		{
			name: "monitor with nil cancel",
			monitor: &Monitor{
				incidents: &Incidents{},
				interval:  5 * time.Minute,
				ctx:       context.Background(),
				cancel:    nil,
			},
		},
		{
			name: "valid monitor",
			monitor: &Monitor{
				incidents: &Incidents{},
				interval:  5 * time.Minute,
				ctx:       context.Background(),
				cancel:    func() {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that Stop doesn't panic
			assert.NotPanics(t, func() {
				tt.monitor.Stop()
			})
		})
	}
}

func TestMonitor_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	client := NewClient("https://api.example.com/incidents", "test-token")
	incidents := NewIncidents(client)
	monitor := &Monitor{
		incidents: incidents,
		interval:  10 * time.Millisecond,
		ctx:       ctx,
		cancel:    cancel,
	}

	// Start monitor in goroutine
	done := make(chan bool)
	go func() {
		monitor.Start()
		done <- true
	}()

	// Cancel context after short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	select {
	case <-done:
		// Monitor stopped due to context cancellation
	case <-time.After(1 * time.Second):
		t.Fatal("Monitor did not stop after context cancellation")
	}
}
