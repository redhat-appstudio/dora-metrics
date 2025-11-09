package main

import (
	"log"

	"github.com/redhat-appstudio/dora-metrics/internal/config"
	"github.com/redhat-appstudio/dora-metrics/internal/server"
	"github.com/redhat-appstudio/dora-metrics/pkg/logger"

	"github.com/joho/godotenv"
)

// main is the entry point for the DORA Metrics Server application.
// It performs the following operations:
//  1. Parses command-line flags for server configuration
//  2. Loads environment variables from .env file if present
//  3. Loads configuration from YAML files with flag overrides
//  4. Initializes the HTTP server with monitoring services
//  5. Starts WebRCA incident monitoring (if enabled)
//  6. Starts ArgoCD application monitoring (if enabled)
//  7. Begins listening for HTTP requests
//
// The application supports graceful shutdown and proper resource cleanup.
func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Load configuration from YAML and environment variables (cached for performance)
	cfg := config.LoadCached()

	// Create and start server
	srv := server.New(cfg)

	logger.Infof(" Starting on port %s", cfg.Port)
	logger.Infof(" Environment: %s", cfg.Environment)
	logger.Infof("Log level: %s", cfg.LogLevel)

	if cfg.WebRCA.Enabled {
		logger.Infof("WebRCA monitoring: enabled (interval: %s)", cfg.WebRCA.Interval)
	} else {
		logger.Infof("WebRCA monitoring: disabled")
	}

	if cfg.ArgoCD.Enabled {
		logger.Infof("ArgoCD monitoring: enabled (namespaces: %v)", cfg.ArgoCD.Namespaces)
		if len(cfg.ArgoCD.ComponentsToIgnore) > 0 {
			logger.Infof("ArgoCD components to ignore: %v", cfg.ArgoCD.ComponentsToIgnore)
		} else {
			logger.Infof("ArgoCD monitoring: all components will be monitored")
		}
		if len(cfg.ArgoCD.KnownClusters) > 0 {
			logger.Infof("ArgoCD known clusters: %v", cfg.ArgoCD.KnownClusters)
		}
	} else {
		logger.Infof("ArgoCD monitoring: disabled")
	}

	if cfg.Integration.DevLake.Enabled {
		logger.Infof("DevLake integration: enabled (global project ID: %s)", cfg.Integration.DevLake.ProjectID)
		if len(cfg.Integration.DevLake.Teams) > 0 {
			logger.Infof("DevLake teams: %d team(s) configured for component routing", len(cfg.Integration.DevLake.Teams))
		}
	} else {
		logger.Infof("DevLake integration: disabled")
	}

	if err := srv.Start(); err != nil {
		logger.Fatalf("Server failed to start: %v", err)
	}
}
