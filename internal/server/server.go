package server

import (
	"context"
	"log"

	"github.com/goccy/go-json"

	"github.com/redhat-appstudio/dora-metrics/apis/common"
	"github.com/redhat-appstudio/dora-metrics/internal/config"
	"github.com/redhat-appstudio/dora-metrics/internal/handlers"
	"github.com/redhat-appstudio/dora-metrics/internal/version"
	"github.com/redhat-appstudio/dora-metrics/pkg/integrations"
	"github.com/redhat-appstudio/dora-metrics/pkg/logger"
	argocdmonitor "github.com/redhat-appstudio/dora-metrics/pkg/monitors/argocd"
	"github.com/redhat-appstudio/dora-metrics/pkg/monitors/webrca"
	"github.com/redhat-appstudio/dora-metrics/pkg/storage"

	argocdclient "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// Server represents the HTTP server instance with all its components.
// It encapsulates the Fiber application, configuration, and monitoring services
// for the DORA Metrics Server.
type Server struct {
	// app is the Fiber HTTP application instance
	app *fiber.App

	// cfg contains the server configuration
	cfg *config.Config

	// webrcaMonitor handles WebRCA incident monitoring
	webrcaMonitor *webrca.Monitor

	// argocdMonitor handles ArgoCD application monitoring
	argocdMonitor argocdmonitor.Monitor
}

// New creates and initializes a new Server instance with the provided configuration.
// It sets up the Fiber application with middleware, routes, and monitoring services.
// The server will be ready to start after this function returns.
func New(cfg *config.Config) *Server {
	// Initialize logger first
	if err := logger.InitFromConfig(cfg); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Initialize devlake integration
	integrationManager := integrations.GetManager()
	integrationManager.RegisterDevLakeIntegration(
		cfg.Integration.DevLake.BaseURL,
		cfg.Integration.DevLake.ProjectID,
		cfg.Integration.DevLake.Enabled,
		cfg.Integration.DevLake.TimeoutSeconds,
		cfg.Integration.DevLake.Teams,
	)
	
	// Log DevLake integration configuration
	if cfg.Integration.DevLake.Enabled {
		logger.Infof("DevLake integration: enabled (base URL: %s, global project ID: %s)", cfg.Integration.DevLake.BaseURL, cfg.Integration.DevLake.ProjectID)
		if len(cfg.Integration.DevLake.Teams) > 0 {
			logger.Infof("DevLake teams configured: %d team(s)", len(cfg.Integration.DevLake.Teams))
			for _, team := range cfg.Integration.DevLake.Teams {
				logger.Infof("  Team: %s (project ID: %s) - Components: %v", team.Name, team.ProjectID, team.ArgocdComponents)
			}
		} else {
			logger.Infof("DevLake integration: no teams configured - deployments will only be sent to global project")
		}
	} else {
		logger.Infof("DevLake integration: disabled")
	}

	// Create Fiber app with faster JSON encoder
	app := fiber.New(fiber.Config{
		AppName:     "DORA Metrics Server " + version.GetVersion(),
		JSONEncoder: json.Marshal,
		JSONDecoder: json.Unmarshal,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(common.ErrorResponse{
				Error:   true,
				Message: err.Error(),
			})
		},
	})

	// Middleware
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	// Create ArgoCD client for API endpoints
	var argocdClient *argocdclient.Clientset
	if cfg.ArgoCD.Enabled {
		argocdConfig := &argocdmonitor.Config{
			Enabled:           cfg.ArgoCD.Enabled,
			Namespaces:        cfg.ArgoCD.Namespaces,
			ComponentsToIgnore: cfg.ArgoCD.ComponentsToIgnore,
			KnownClusters:     cfg.ArgoCD.KnownClusters,
		}

		argocdMonitorClient, err := argocdmonitor.CreateArgoCDClient(argocdConfig)
		if err != nil {
			logger.Errorf("Failed to create ArgoCD client for API: %v", err)
			logger.Warn("ArgoCD API endpoints will not be available")
		} else {
			argocdClient = argocdMonitorClient.GetArgoCDClient()
			logger.Info("ArgoCD client created successfully for API")
		}
	}

	// Setup routes
	handlers.SetupRoutes(app, argocdClient, cfg.ArgoCD.Namespaces, cfg.ArgoCD.ComponentsToIgnore, cfg.ArgoCD.KnownClusters)

	// Initialize storage client if enabled
	var storageClient *storage.RedisClient
	if cfg.Storage.Redis.Enabled {
		storageConfig := storage.StorageConfig{
			Redis: storage.RedisConfig{
				Enabled:   cfg.Storage.Redis.Enabled,
				Address:   cfg.Storage.Redis.Address,
				Password:  cfg.Storage.Redis.Password,
				Database:  cfg.Storage.Redis.Database,
				KeyPrefix: cfg.Storage.Redis.KeyPrefix,
			},
		}

		var err error
		storageClient, err = storage.NewManager(storageConfig)
		if err != nil {
			logger.Fatalf("Failed to initialize Redis storage client: %v", err)
		}
		logger.Infof("Redis storage client initialized successfully - Address: %s", cfg.Storage.Redis.Address)
	}

	// Initialize WebRCA monitor if enabled
	var webrcaMonitor *webrca.Monitor
	if cfg.WebRCA.Enabled && cfg.WebRCA.Token != "" {
		// Use global logger
		webrcaMonitor = webrca.NewMonitor(cfg.WebRCA.APIURL, cfg.WebRCA.Token, cfg.WebRCA.Interval)
		if webrcaMonitor != nil {
			logger.Infof("WebRCA incident monitoring enabled - API URL: %s, Check interval: %v", cfg.WebRCA.APIURL, cfg.WebRCA.Interval)
		}
	} else if cfg.WebRCA.Enabled {
		logger.Warnf("WebRCA monitoring enabled but OFFLINE_TOKEN environment variable not set")
	}

	// Initialize ArgoCD monitor if enabled
	var argocdMonitor argocdmonitor.Monitor
	if cfg.ArgoCD.Enabled {
		// Validate required ArgoCD configuration - strict validation
		if len(cfg.ArgoCD.Namespaces) == 0 {
			logger.Fatalf("ArgoCD monitoring enabled but namespaces not specified in config.yaml")
		}
		if len(cfg.ArgoCD.KnownClusters) == 0 {
			logger.Fatalf("ArgoCD monitoring enabled but known_clusters not specified in config.yaml")
		}

		// Set known clusters from configuration
		argocdmonitor.SetKnownClusters(cfg.ArgoCD.KnownClusters)

		argocdConfig := &argocdmonitor.Config{
			Enabled:           cfg.ArgoCD.Enabled,
			Namespaces:        cfg.ArgoCD.Namespaces,
			ComponentsToIgnore: cfg.ArgoCD.ComponentsToIgnore,
			KnownClusters:     cfg.ArgoCD.KnownClusters,
		}

		// Create ArgoCD monitor with storage client
		var err error
		argocdMonitor, err = argocdmonitor.NewMonitor(argocdConfig, storageClient)
		if err != nil {
			logger.Fatalf("Failed to initialize ArgoCD monitor: %v", err)
		}
		if argocdMonitor == nil {
			logger.Fatal("ArgoCD monitor is nil after initialization")
		}
		logger.Infof("ArgoCD application monitoring enabled - Namespaces: %v, Known clusters: %d", cfg.ArgoCD.Namespaces, len(cfg.ArgoCD.KnownClusters))
	}

	return &Server{
		app:           app,
		cfg:           cfg,
		webrcaMonitor: webrcaMonitor,
		argocdMonitor: argocdMonitor,
	}
}

// Start starts the HTTP server and all monitoring services.
// It launches background goroutines for WebRCA and ArgoCD monitoring,
// then starts the HTTP server to listen for incoming requests.
// Returns an error if the server fails to start.
func (s *Server) Start() error {
	// Start WebRCA monitor in background if enabled
	if s.webrcaMonitor != nil {
		logger.Info("Starting WebRCA incident monitoring thread...")
		go s.webrcaMonitor.Start()
	}

	// Start ArgoCD monitor in background if enabled
	if s.argocdMonitor != nil {
		logger.Info("Starting ArgoCD application monitoring thread...")
		go func() {
			if err := s.argocdMonitor.Start(context.Background()); err != nil {
				logger.Fatalf("Failed to start ArgoCD monitor: %v", err)
			}
		}()
	}

	return s.app.Listen(":" + s.cfg.Port)
}
