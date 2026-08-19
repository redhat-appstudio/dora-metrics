package argocd

import (
	"github.com/gofiber/fiber/v3"
)

// RegisterRoutes registers all ArgoCD API routes.
func RegisterRoutes(app *fiber.App, handler *Handler) {
	// Create API v1 group
	v1 := app.Group("/api/v1")

	// ArgoCD routes
	argocd := v1.Group("/argocd")

	if handler != nil {
		// JSON endpoint
		argocd.Get("/applications", handler.ListApplicationsJSON)
		// TOON endpoint
		argocd.Get("/applications.toon", handler.ListApplicationsTOON)
	} else {
		// Add fallback endpoint when handler is nil
		argocd.Get("/applications", func(c *fiber.Ctx) error {
			return c.Status(503).JSON(fiber.Map{
				"error": "ArgoCD client not available",
			})
		})
	}
}
