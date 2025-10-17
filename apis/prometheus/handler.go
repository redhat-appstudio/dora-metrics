package prometheus

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Handler handles Prometheus API requests.
type Handler struct {
	// argocdMetricsURL is the URL to the ArgoCD metrics endpoint
	argocdMetricsURL string
}

// NewHandler creates a new Prometheus API handler.
func NewHandler(argocdMetricsURL string) (*Handler, error) {
	// For now, we'll create a simple handler without the Prometheus client
	// to avoid circular dependencies. The query endpoints will return mock data.
	return &Handler{
		argocdMetricsURL: argocdMetricsURL,
	}, nil
}

// GetMetricNames handles GET /api/v1/label/__name__/values
// Returns all available metric names
func (h *Handler) GetMetricNames(c *fiber.Ctx) error {
	// For DORA metrics, we know the available metrics
	metricNames := []string{
		"argocd_application_info",
		"argocd_application_count_total",
	}

	c.Set("Content-Type", "application/json")
	return c.JSON(fiber.Map{
		"status": "success",
		"data":   metricNames,
	})
}

// Query handles GET /api/v1/query
// Executes a PromQL query
func (h *Handler) Query(c *fiber.Ctx) error {
	query := c.Query("query")
	if query == "" {
		return c.Status(400).JSON(fiber.Map{
			"status":    "error",
			"errorType": "bad_data",
			"error":     "missing query parameter",
		})
	}

	// For now, return mock data for common queries
	// This allows Grafana to connect and discover metrics
	var result interface{}

	switch {
	case query == "argocd_application_count_total":
		// Return mock data for application count
		result = []fiber.Map{
			{
				"metric": fiber.Map{
					"__name__":  "argocd_application_count_total",
					"cluster":   "kflux-ocp-p01",
					"namespace": "argocd",
				},
				"value": []interface{}{time.Now().Unix(), "5"},
			},
			{
				"metric": fiber.Map{
					"__name__":  "argocd_application_count_total",
					"cluster":   "kflux-osp-p01",
					"namespace": "argocd",
				},
				"value": []interface{}{time.Now().Unix(), "3"},
			},
			{
				"metric": fiber.Map{
					"__name__":  "argocd_application_count_total",
					"cluster":   "kflux-prd-es01",
					"namespace": "argocd",
				},
				"value": []interface{}{time.Now().Unix(), "2"},
			},
			{
				"metric": fiber.Map{
					"__name__":  "argocd_application_count_total",
					"cluster":   "stone-prod-p01",
					"namespace": "argocd",
				},
				"value": []interface{}{time.Now().Unix(), "4"},
			},
		}
	case strings.HasPrefix(query, "argocd_application_info"):
		// Return mock data for application info
		result = []fiber.Map{
			{
				"metric": fiber.Map{
					"__name__":      "argocd_application_info",
					"cluster":       "kflux-ocp-p01",
					"namespace":     "argocd",
					"name":          "my-app",
					"health_status": "Healthy",
					"sync_status":   "Synced",
				},
				"value": []interface{}{time.Now().Unix(), "1"},
			},
		}
	default:
		// Return empty result for unknown queries
		result = []fiber.Map{}
	}

	response := fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"resultType": "vector",
			"result":     result,
		},
	}

	c.Set("Content-Type", "application/json")
	return c.JSON(response)
}

// QueryRange handles GET /api/v1/query_range
// Executes a PromQL query over a time range
func (h *Handler) QueryRange(c *fiber.Ctx) error {
	query := c.Query("query")
	if query == "" {
		return c.Status(400).JSON(fiber.Map{
			"status":    "error",
			"errorType": "bad_data",
			"error":     "missing query parameter",
		})
	}

	// For now, return mock data for range queries
	// This allows Grafana to create time-series graphs
	var result interface{}

	switch {
	case query == "argocd_application_count_total":
		// Return mock time series data
		now := time.Now()
		var values [][]interface{}
		for i := 0; i < 10; i++ {
			timestamp := now.Add(-time.Duration(i) * time.Minute).Unix()
			value := 5 + i%3 // Vary the value slightly
			values = append(values, []interface{}{timestamp, fmt.Sprintf("%d", value)})
		}

		result = []fiber.Map{
			{
				"metric": fiber.Map{
					"__name__":  "argocd_application_count_total",
					"cluster":   "kflux-ocp-p01",
					"namespace": "argocd",
				},
				"values": values,
			},
			{
				"metric": fiber.Map{
					"__name__":  "argocd_application_count_total",
					"cluster":   "kflux-prd-rh02",
					"namespace": "argocd",
				},
				"values": values,
			},
			{
				"metric": fiber.Map{
					"__name__":  "argocd_application_count_total",
					"cluster":   "stone-prod-p02",
					"namespace": "argocd",
				},
				"values": values,
			},
		}
	default:
		// Return empty result for unknown queries
		result = []fiber.Map{}
	}

	response := fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"resultType": "matrix",
			"result":     result,
		},
	}

	c.Set("Content-Type", "application/json")
	return c.JSON(response)
}

// GetSeries handles GET /api/v1/series
// Returns series data matching the query
func (h *Handler) GetSeries(c *fiber.Ctx) error {
	// Parse match[] parameters
	var matches []string
	queries := c.Queries()
	for key, value := range queries {
		if strings.HasPrefix(key, "match[]") {
			matches = append(matches, value)
		}
	}

	if len(matches) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"status":    "error",
			"errorType": "bad_data",
			"error":     "missing match[] parameter",
		})
	}

	// Parse time range (for future use)
	_ = c.Query("start")
	_ = c.Query("end")

	// For now, return mock series data
	// This allows Grafana to discover available series
	result := []fiber.Map{
		{
			"__name__":  "argocd_application_count_total",
			"cluster":   "kflux-ocp-p01",
			"namespace": "argocd",
		},
		{
			"__name__":  "argocd_application_info",
			"cluster":   "kflux-ocp-p01",
			"namespace": "argocd",
			"name":      "my-app",
		},
	}

	response := fiber.Map{
		"status": "success",
		"data":   result,
	}

	c.Set("Content-Type", "application/json")
	return c.JSON(response)
}

// GetLabels handles GET /api/v1/labels
// Returns all available label names
func (h *Handler) GetLabels(c *fiber.Ctx) error {
	// For DORA metrics, we know the available labels
	labels := []string{
		"namespace",
		"name",
		"cluster",
		"environment",
		"component",
		"health_status",
		"health_value",
		"sync_status",
		"sync_value",
		"image",
	}

	c.Set("Content-Type", "application/json")
	return c.JSON(fiber.Map{
		"status": "success",
		"data":   labels,
	})
}

// GetLabelValues handles GET /api/v1/label/:name/values
// Returns all values for a specific label
func (h *Handler) GetLabelValues(c *fiber.Ctx) error {
	labelName := c.Params("name")
	if labelName == "" {
		return c.Status(400).JSON(fiber.Map{
			"status":    "error",
			"errorType": "bad_data",
			"error":     "missing label name",
		})
	}

	// For DORA metrics, return known values for common labels
	var values []string
	switch labelName {
	case "cluster":
		// Use the exact clusters from your configuration
		values = []string{
			"kflux-ocp-p01",
			"kflux-osp-p01",
			"kflux-prd-es01",
			"kflux-prd-rh02",
			"kflux-prd-rh03",
			"kflux-rhel-p01",
			"stone-prd-host1",
			"stone-prd-rh01",
			"stone-prod-p01",
			"stone-prod-p02",
			"all",
		}
	case "environment":
		values = []string{"production", "staging", "development"}
	case "health_status":
		values = []string{"Healthy", "Progressing", "Degraded", "Suspended", "Missing", "Unknown"}
	case "sync_status":
		values = []string{"Synced", "OutOfSync", "Unknown"}
	default:
		values = []string{} // Empty for unknown labels
	}

	c.Set("Content-Type", "application/json")
	return c.JSON(fiber.Map{
		"status": "success",
		"data":   values,
	})
}

// GetTargets handles GET /api/v1/targets
// Returns target information
func (h *Handler) GetTargets(c *fiber.Ctx) error {
	// Return a simple target for DORA metrics
	targets := fiber.Map{
		"activeTargets": []fiber.Map{
			{
				"discoveredLabels": fiber.Map{
					"__address__":      h.argocdMetricsURL,
					"__metrics_path__": "/api/v1/argocd/metrics",
					"__scheme__":       "https",
				},
				"labels": fiber.Map{
					"instance": h.argocdMetricsURL,
					"job":      "dora-metrics-argocd",
				},
				"scrapePool":         "dora-metrics-argocd",
				"scrapeUrl":          h.argocdMetricsURL + "/api/v1/argocd/metrics",
				"globalUrl":          h.argocdMetricsURL + "/api/v1/argocd/metrics",
				"lastError":          "",
				"lastScrape":         time.Now().Format(time.RFC3339),
				"lastScrapeDuration": 0.1,
				"health":             "up",
				"scrapeInterval":     "30s",
				"scrapeTimeout":      "10s",
			},
		},
		"droppedTargets": []fiber.Map{},
	}

	c.Set("Content-Type", "application/json")
	return c.JSON(fiber.Map{
		"status": "success",
		"data":   targets,
	})
}

// GetTargetsMetadata handles GET /api/v1/targets/metadata
// Returns target metadata
func (h *Handler) GetTargetsMetadata(c *fiber.Ctx) error {
	// Return empty metadata for now
	c.Set("Content-Type", "application/json")
	return c.JSON(fiber.Map{
		"status": "success",
		"data":   []fiber.Map{},
	})
}

// GetRules handles GET /api/v1/rules
// Returns recording and alerting rules
func (h *Handler) GetRules(c *fiber.Ctx) error {
	// Return empty rules for now
	c.Set("Content-Type", "application/json")
	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"groups": []fiber.Map{},
		},
	})
}

// GetAlerts handles GET /api/v1/alerts
// Returns active alerts
func (h *Handler) GetAlerts(c *fiber.Ctx) error {
	// Return empty alerts for now
	c.Set("Content-Type", "application/json")
	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"alerts": []fiber.Map{},
		},
	})
}

// GetConfig handles GET /api/v1/status/config
// Returns Prometheus configuration
func (h *Handler) GetConfig(c *fiber.Ctx) error {
	config := fiber.Map{
		"yaml": "# DORA Metrics Prometheus API Wrapper\n# This is a simplified configuration for Grafana compatibility",
	}

	c.Set("Content-Type", "application/json")
	return c.JSON(fiber.Map{
		"status": "success",
		"data":   config,
	})
}

// GetFlags handles GET /api/v1/status/flags
// Returns Prometheus flags
func (h *Handler) GetFlags(c *fiber.Ctx) error {
	flags := fiber.Map{
		"web.listen-address":   ":8080",
		"web.enable-lifecycle": "false",
		"web.enable-admin-api": "false",
	}

	c.Set("Content-Type", "application/json")
	return c.JSON(fiber.Map{
		"status": "success",
		"data":   flags,
	})
}

// GetRuntimeInfo handles GET /api/v1/status/runtimeinfo
// Returns runtime information
func (h *Handler) GetRuntimeInfo(c *fiber.Ctx) error {
	runtimeInfo := fiber.Map{
		"startTime":           time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		"CWD":                 "/app",
		"reloadConfigSuccess": true,
		"lastConfigTime":      time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		"corruptionCount":     0,
		"goroutineCount":      10,
		"GOMAXPROCS":          1,
		"GOGC":                "100",
		"GODEBUG":             "",
		"storageRetention":    "15d",
	}

	c.Set("Content-Type", "application/json")
	return c.JSON(fiber.Map{
		"status": "success",
		"data":   runtimeInfo,
	})
}

// GetBuildInfo handles GET /api/v1/status/buildinfo
// Returns build information
func (h *Handler) GetBuildInfo(c *fiber.Ctx) error {
	buildInfo := fiber.Map{
		"version":   "2.40.0",
		"revision":  "dora-metrics-1.0.0",
		"branch":    "main",
		"buildUser": "dora-metrics",
		"buildDate": time.Now().Format("20060102-15:04:05"),
		"goVersion": "go1.21.0",
	}

	c.Set("Content-Type", "application/json")
	return c.JSON(fiber.Map{
		"status": "success",
		"data":   buildInfo,
	})
}

// GetTSDBStatus handles GET /api/v1/status/tsdb
// Returns TSDB status
func (h *Handler) GetTSDBStatus(c *fiber.Ctx) error {
	tsdbStatus := fiber.Map{
		"headStats": fiber.Map{
			"numSeries":  100,
			"numSamples": 1000,
			"numChunks":  500,
			"minTime":    time.Now().Add(-24 * time.Hour).UnixMilli(),
			"maxTime":    time.Now().UnixMilli(),
		},
		"seriesCountByMetricName":     []fiber.Map{},
		"labelValueCountByLabelName":  []fiber.Map{},
		"memoryInBytesByLabelName":    []fiber.Map{},
		"seriesCountByLabelValuePair": []fiber.Map{},
	}

	c.Set("Content-Type", "application/json")
	return c.JSON(fiber.Map{
		"status": "success",
		"data":   tsdbStatus,
	})
}

// GetWALReplayStatus handles GET /api/v1/status/walreplay
// Returns WAL replay status
func (h *Handler) GetWALReplayStatus(c *fiber.Ctx) error {
	walStatus := fiber.Map{
		"min":     0,
		"max":     0,
		"current": 0,
		"state":   "done",
	}

	c.Set("Content-Type", "application/json")
	return c.JSON(fiber.Map{
		"status": "success",
		"data":   walStatus,
	})
}

// Helper functions

func parseTime(timeStr string) (time.Time, error) {
	if timestamp, err := strconv.ParseFloat(timeStr, 64); err == nil {
		return time.Unix(int64(timestamp), 0), nil
	}
	return time.Parse(time.RFC3339, timeStr)
}

func parseDuration(durationStr string) (time.Duration, error) {
	return time.ParseDuration(durationStr)
}
