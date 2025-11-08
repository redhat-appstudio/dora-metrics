package config

import "time"

// Config represents the main application configuration structure.
// It contains all configuration settings for the DORA Metrics Server,
// including server settings, WebRCA monitoring, ArgoCD monitoring, and storage.
type Config struct {
	// HTTP server port (e.g., "3000")
	Port string

	// Application environment (e.g., "development", "production")
	Environment string

	// Logging level (e.g., "info", "debug", "warn", "error")
	LogLevel string

	// WebRCA incident monitoring configuration
	WebRCA WebRCAConfig

	// ArgoCD application monitoring configuration
	ArgoCD ArgoCDConfig

	// Deployment history storage configuration
	Storage StorageConfig

	// Integration configuration for external systems
	Integration IntegrationConfig
}

// WebRCAConfig holds configuration for WebRCA incident monitoring.
// It contains settings for API access, authentication, and monitoring intervals
// used to track incidents from the Red Hat WebRCA system.
type WebRCAConfig struct {
	// Whether WebRCA monitoring is enabled (true/false)
	Enabled bool

	// WebRCA API endpoint URL (e.g., "https://api.openshift.com/api/web-rca/v1/incidents")
	APIURL string

	// OAuth2 offline token for API authentication
	Token string

	// Polling interval for incident checks (e.g., "1h", "30m")
	Interval time.Duration
}

// ServerConfig represents server-related configuration settings.
// It contains HTTP server configuration including port, environment,
// and logging settings that can be overridden by command-line flags.
type ServerConfig struct {
	// HTTP server port (e.g., "3000")
	Port string `yaml:"port"`

	// Application environment (e.g., "development", "production")
	Environment string `yaml:"environment"`

	// Logging level (e.g., "info", "debug", "warn", "error")
	LogLevel string `yaml:"log_level"`
}

// WebRCAYAMLConfig represents WebRCA monitoring configuration from YAML files.
// It mirrors the WebRCAConfig structure but uses YAML tags for proper
// unmarshaling from configuration files.
type WebRCAYAMLConfig struct {
	// Whether WebRCA monitoring is enabled (true/false)
	Enabled bool `yaml:"enabled"`

	// WebRCA API endpoint URL (e.g., "https://api.openshift.com/api/web-rca/v1/incidents")
	APIURL string `yaml:"api_url"`

	// OAuth2 offline token for API authentication
	Token string `yaml:"token"`

	// Polling interval as string (e.g., "1h", "30m")
	Interval string `yaml:"interval"`
}

// ArgoCDConfig holds configuration for ArgoCD application monitoring.
// It contains settings for watching ArgoCD applications across multiple
// namespaces and filtering applications to monitor.
type ArgoCDConfig struct {
	// Whether ArgoCD monitoring is enabled (true/false)
	Enabled bool

	// Kubernetes namespaces to watch (e.g., ["argocd", "production"])
	Namespaces []string

	// Components to ignore (all other components will be monitored)
	// These components will be excluded from monitoring across all known clusters
	ComponentsToIgnore []string

	// Known cluster names for parsing (e.g., ["kflux-ocp-p01", "pentest-p01"])
	KnownClusters []string
}

// ArgoCDYAMLConfig represents ArgoCD monitoring configuration from YAML files.
// It mirrors the ArgoCDConfig structure but uses YAML tags for proper
// unmarshaling from configuration files.
type ArgoCDYAMLConfig struct {
	// Whether ArgoCD monitoring is enabled (true/false)
	Enabled bool `yaml:"enabled"`

	// Kubernetes namespaces to watch (e.g., ["argocd", "production"])
	Namespaces []string `yaml:"namespaces"`

	// Components to ignore (all other components will be monitored)
	// These components will be excluded from monitoring across all known clusters
	ComponentsToIgnore []string `yaml:"components_to_ignore"`

	// Known cluster names for parsing (e.g., ["kflux-ocp-p01", "pentest-p01"])
	KnownClusters []string `yaml:"known_clusters"`
}

// StorageConfig holds configuration for deployment history storage.
type StorageConfig struct {
	// Redis storage configuration
	Redis RedisYAMLConfig `yaml:"redis"`
}

// RedisYAMLConfig represents Redis configuration from YAML files.
type RedisYAMLConfig struct {
	// Whether Redis storage is enabled (true/false)
	Enabled bool `yaml:"enabled"`

	// Redis server address (e.g., "localhost:6379")
	Address string `yaml:"address"`

	// Redis password for authentication
	Password string `yaml:"password"`

	// Redis database number (0-15)
	Database int `yaml:"database"`

	// Key prefix for all Redis keys (e.g., "dora-metrics")
	KeyPrefix string `yaml:"key_prefix"`
}

// YAMLConfig represents the structure of the YAML configuration file.
// It defines the complete structure for configs/config.yaml and provides
// the root configuration object for the application.
type YAMLConfig struct {
	// Server configuration settings
	Server ServerConfig `yaml:"server"`

	// WebRCA monitoring configuration
	WebRCA WebRCAYAMLConfig `yaml:"webrca"`

	// ArgoCD monitoring configuration
	ArgoCD ArgoCDYAMLConfig `yaml:"argocd"`

	// Storage configuration
	Storage StorageConfig `yaml:"storage"`

	// Integration configuration
	Integration IntegrationYAMLConfig `yaml:"integration"`
}

// IntegrationConfig holds configuration for external system integrations.
// It contains settings for DevLake and other integration endpoints.
type IntegrationConfig struct {
	// DevLake integration configuration
	DevLake DevLakeConfig
}

// DevLakeConfig holds configuration for DevLake integration.
// It contains settings for DevLake API access and data synchronization.
type DevLakeConfig struct {
	// Whether DevLake integration is enabled (true/false)
	Enabled bool

	// DevLake base URL (e.g., "http://localhost:4000")
	BaseURL string

	// DevLake project ID for webhook connections (e.g., "11")
	ProjectID string

	// API timeout in seconds (default: 30)
	TimeoutSeconds int
}

// IntegrationYAMLConfig represents integration configuration in YAML format.
// It contains DevLake and other integration settings that can be configured
// via YAML configuration files.
type IntegrationYAMLConfig struct {
	// DevLake integration configuration
	DevLake DevLakeYAMLConfig `yaml:"devlake"`
}

// DevLakeYAMLConfig holds DevLake configuration in YAML format.
// It contains settings for DevLake API access and data synchronization
// that can be configured via YAML configuration files.
type DevLakeYAMLConfig struct {
	// Whether DevLake integration is enabled (true/false)
	Enabled bool `yaml:"enabled"`

	// DevLake base URL (e.g., "http://localhost:4000")
	BaseURL string `yaml:"base_url"`

	// DevLake project ID for webhook connections (e.g., "11")
	ProjectID string `yaml:"project_id"`

	// API timeout in seconds (default: 30)
	TimeoutSeconds int `yaml:"timeout_seconds"`
}
