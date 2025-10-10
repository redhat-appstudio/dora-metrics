package handlers

import (
	"github.com/redhat-appstudio/dora-metrics/apis/argocd"
	"github.com/redhat-appstudio/dora-metrics/apis/health"
	"github.com/redhat-appstudio/dora-metrics/internal/version"

	argocdclient "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"github.com/gofiber/fiber/v2"
)

// SetupRoutes configures all HTTP routes for the DORA Metrics Server.
// It registers API endpoints for health checks and other services using the API machinery pattern.
// This function should be called during server initialization.
func SetupRoutes(app *fiber.App, argocdClient *argocdclient.Clientset, argocdNamespaces, argocdComponentsToMonitor []string) {
	// Register all APIs here - just add one line per API
	health.RegisterRoutes(app)

	// Register ArgoCD API if client is available
	if argocdClient != nil {
		argocdHandler, err := argocd.NewHandler(argocdClient, argocdNamespaces, argocdComponentsToMonitor)
		if err != nil {
			// Log error but continue - ArgoCD API will not be available
			// The error is already logged in NewHandler
		} else {
			argocd.RegisterRoutes(app, argocdHandler)
		}
	}

	// Future APIs would be added like this:
	// users.RegisterRoutes(app)
	// metrics.RegisterRoutes(app)
	// auth.RegisterRoutes(app)

	// Root endpoint
	app.Get("/", RootHandler)
}

// RootHandler handles requests to the root endpoint ("/").
// It returns basic server information including name, version, and available API endpoints.
func RootHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"message": "DORA Metrics Server",
		"version": version.GetShortVersion(),
		"docs":    "/api/v1/health",
	})
}
