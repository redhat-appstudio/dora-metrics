package argocd

import (
	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes registers all ArgoCD API routes.
func RegisterRoutes(app *fiber.App, handler *Handler) {
	// Create API v1 group
	v1 := app.Group("/api/v1")

	// ArgoCD routes
	argocd := v1.Group("/argocd")

	if handler != nil {
		// Prometheus metrics endpoint
		argocd.Get("/metrics", handler.ListApplications)
	} else {
		// Add a fallback endpoint when handler is nil
		argocd.Get("/metrics", func(c *fiber.Ctx) error {
			c.Set("Content-Type", "text/plain")
			return c.Status(503).SendString("# ERROR: ArgoCD client not available\n")
		})
	}
}
