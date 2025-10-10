package health

import (
	"time"

	"github.com/redhat-appstudio/dora-metrics/internal/version"

	"github.com/gofiber/fiber/v2"
)

var startTime = time.Now()

// HealthHandler handles health check requests and returns server status information.
// It provides uptime, version, and current status for monitoring purposes.
func HealthHandler(c *fiber.Ctx) error {
	uptime := time.Since(startTime)

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   version.GetShortVersion(),
		Uptime:    uptime.String(),
	}

	return c.JSON(response)
}
