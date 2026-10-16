package health

import (
	"github.com/gofiber/fiber/v3"
)

// RegisterRoutes registers all health API routes with the Fiber application.
// It sets up the health check endpoint under the /api/v1 path.
func RegisterRoutes(app *fiber.App) {
	// Health API group
	healthGroup := app.Group("/api/v1")

	// Health check endpoint
	healthGroup.Get("/health", HealthHandler)
}
