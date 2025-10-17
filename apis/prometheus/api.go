package prometheus

import (
	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes registers all Prometheus API routes.
// This provides a Prometheus-compatible API wrapper for Grafana integration.
func RegisterRoutes(app *fiber.App, handler *Handler) {
	// Create API v1 group for Prometheus compatibility
	v1 := app.Group("/api/v1")

	if handler != nil {
		// Standard Prometheus API endpoints that Grafana expects
		v1.Get("/label/__name__/values", handler.GetMetricNames)
		v1.Get("/query", handler.Query)
		v1.Get("/query_range", handler.QueryRange)
		v1.Get("/series", handler.GetSeries)
		v1.Get("/labels", handler.GetLabels)
		v1.Get("/label/:name/values", handler.GetLabelValues)
		v1.Get("/targets", handler.GetTargets)
		v1.Get("/targets/metadata", handler.GetTargetsMetadata)
		v1.Get("/rules", handler.GetRules)
		v1.Get("/alerts", handler.GetAlerts)
		v1.Get("/status/config", handler.GetConfig)
		v1.Get("/status/flags", handler.GetFlags)
		v1.Get("/status/runtimeinfo", handler.GetRuntimeInfo)
		v1.Get("/status/buildinfo", handler.GetBuildInfo)
		v1.Get("/status/tsdb", handler.GetTSDBStatus)
		v1.Get("/status/walreplay", handler.GetWALReplayStatus)
	} else {
		// Add fallback endpoints when handler is nil
		v1.Get("/label/__name__/values", func(c *fiber.Ctx) error {
			c.Set("Content-Type", "application/json")
			return c.Status(503).JSON(fiber.Map{
				"status": "error",
				"error":  "Prometheus API not available",
			})
		})
	}
}
