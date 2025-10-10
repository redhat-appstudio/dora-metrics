package health

import "time"

// HealthResponse represents the health check response structure.
// It contains server status information for monitoring and health checks,
// including uptime, version, and service availability.
type HealthResponse struct {
	// Status indicates the current server status (e.g., "healthy", "unhealthy")
	Status string `json:"status"`

	// Timestamp is when the health check was performed
	Timestamp time.Time `json:"timestamp"`

	// Version is the server version information
	Version string `json:"version"`

	// Uptime is the server uptime duration
	Uptime string `json:"uptime"`
}
