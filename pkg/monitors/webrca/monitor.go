package webrca

import (
	"context"
	"time"

	"github.com/redhat-appstudio/dora-metrics/pkg/logger"
)

// Monitor orchestrates WebRCA incident monitoring with periodic checks.
// It manages the lifecycle of incident monitoring, handles periodic API calls,
// and provides start/stop controls for the monitoring process.
type Monitor struct {
	incidents *Incidents
	interval  time.Duration
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewMonitor creates a new WebRCA incident monitor with proper configuration.
// It initializes the client, incidents handler, and sets up the monitoring interval.
//
// The function performs the following operations:
// 1. Validates the offline token is provided
// 2. Sets up default interval if not specified
// 3. Creates HTTP client for WebRCA API access
// 4. Initializes incidents service for data processing
// 5. Sets up context for lifecycle management
//
// Parameters:
//   - apiURL: WebRCA API endpoint URL
//   - offlineToken: OAuth2 offline token for API authentication
//   - interval: Time interval between monitoring checks
//
// Returns a configured Monitor instance or nil if offline token is missing.
func NewMonitor(apiURL, offlineToken string, interval time.Duration) *Monitor {
	if offlineToken == "" {
		logger.Warnf(" %s (offlineToken)", ErrMissingConfig)
		return nil
	}

	if interval <= 0 {
		interval = DefaultCheckInterval
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := NewClient(apiURL, offlineToken)
	incidents := NewIncidents(client)

	return &Monitor{
		incidents: incidents,
		interval:  interval,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start begins WebRCA incident monitoring with periodic checks.
// It runs an initial check immediately, then continues checking at the configured interval.
// This method blocks until the monitor is stopped or the context is cancelled.
//
// The monitoring process:
// 1. Performs an immediate incident check
// 2. Sets up a ticker for periodic checks
// 3. Continues monitoring until stopped
func (m *Monitor) Start() {
	if m == nil || m.incidents == nil {
		return
	}

	logger.Infof("Starting WebRCA incident monitoring - interval: %v", m.interval)

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	// Run initial check
	if err := m.incidents.Check(m.ctx); err != nil {
		logger.Errorf("Incident check failed: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := m.incidents.Check(m.ctx); err != nil {
				logger.Errorf("Incident check failed: %v", err)
			}
		case <-m.ctx.Done():
			logger.Infof("WebRCA incident monitoring stopped")
			return
		}
	}
}

// Stop gracefully stops WebRCA incident monitoring.
// It cancels the context and cleans up resources, ensuring proper shutdown
// of the monitoring process.
func (m *Monitor) Stop() {
	if m != nil && m.cancel != nil {
		m.cancel()
	}
}
