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

// TeamConfig holds configuration for a team's DevLake project.
// Teams enable routing deployment events to team-specific DevLake projects in addition to the global project.
// This allows teams to have their own DORA metrics dashboards while maintaining organization-wide visibility.
//
// Deployment routing behavior:
// - Deployments are ALWAYS sent to the global project (DevLakeConfig.ProjectID)
// - If a component matches a team's ArgocdComponents list, the deployment is ALSO sent to that team's project
// - A component can belong to multiple teams (deployment sent to all matching team projects)
// - Components not matching any team only go to the global project
type TeamConfig struct {
	// Team name for identification and logging (e.g., "team-platform", "team-ui")
	Name string

	// DevLake project ID where this team's deployment events will be sent
	// This project should be configured in DevLake to receive webhook events
	// Format: string ID (e.g., "2", "3")
	ProjectID string

	// List of ArgoCD component names that belong to this team
	// Component names are extracted from ArgoCD application names (format: component-cluster)
	// Example: ["build-service", "crossplane-control-plane", "konflux-ui"]
	ArgocdComponents []string
}

// DevLakeConfig holds configuration for DevLake integration.
// DevLake is a data engineering platform that collects, transforms, and visualizes software development data.
// This integration sends ArgoCD deployment events to DevLake projects for DORA metrics calculation.
//
// Integration workflow:
// 1. ArgoCD application deployments are detected and parsed (component, revision, commits)
// 2. Deployment events are formatted according to DevLake's CICD deployment API specification
// 3. Events are sent via HTTP POST to DevLake webhook endpoints
// 4. DevLake processes events to calculate DORA metrics (deployment frequency, lead time, MTTR, change failure rate)
//
// Authentication: Requires DEVLAKE_WEBHOOK_TOKEN environment variable containing the webhook authentication token
type DevLakeConfig struct {
	// Whether DevLake integration is enabled
	// When false, deployment events are not sent to DevLake (monitoring continues locally)
	Enabled bool

	// DevLake instance base URL where API requests will be sent
	// Format: "https://devlake.example.com" or "http://localhost:4000"
	// The integration will append API paths: /api/rest/plugins/webhook/connections/{project_id}/deployments
	BaseURL string

	// Global DevLake project ID that receives ALL deployment events from all components
	// This provides organization-wide DORA metrics visibility
	// Format: string ID (e.g., "1", "11")
	// Note: Deployments are always sent here, regardless of team configurations
	ProjectID string

	// HTTP request timeout in seconds for DevLake API calls
	// Default: 30 seconds
	// Increase if DevLake instance is slow to respond
	TimeoutSeconds int

	// Team project configurations for routing deployments to team-specific DevLake projects
	// Each team configuration maps ArgoCD components to a team's DevLake project
	// Deployments are sent to global project AND all matching team projects
	// Leave empty to only use the global project
	Teams []TeamConfig
}

// IntegrationYAMLConfig represents integration configuration in YAML format.
// It contains DevLake and other integration settings that can be configured
// via YAML configuration files.
type IntegrationYAMLConfig struct {
	// DevLake integration configuration
	DevLake DevLakeYAMLConfig `yaml:"devlake"`
}

// TeamYAMLConfig holds team configuration in YAML format.
// This structure is used for unmarshaling team configurations from YAML files.
// See TeamConfig for detailed field descriptions and deployment routing behavior.
type TeamYAMLConfig struct {
	// Team name for identification and logging
	// Example: "team-platform", "team-ui"
	Name string `yaml:"name"`

	// DevLake project ID where this team's deployment events will be sent
	// Format: string ID (e.g., "2", "3")
	ProjectID string `yaml:"project_id"`

	// List of ArgoCD component names that belong to this team
	// Component names are extracted from ArgoCD application names
	// Example: ["build-service", "crossplane-control-plane"]
	ArgocdComponents []string `yaml:"argocd_components"`
}

// DevLakeYAMLConfig holds DevLake configuration in YAML format.
// This structure is used for unmarshaling configuration from YAML files.
// See DevLakeConfig for detailed field descriptions and integration behavior.
type DevLakeYAMLConfig struct {
	// Whether DevLake integration is enabled (true/false)
	Enabled bool `yaml:"enabled"`

	// DevLake instance base URL
	// Example: "https://devlake.example.com"
	BaseURL string `yaml:"base_url"`

	// Global DevLake project ID that receives all deployment events
	// This project provides organization-wide DORA metrics
	ProjectID string `yaml:"project_id"`

	// HTTP request timeout in seconds for DevLake API calls
	// Default: 30 seconds
	TimeoutSeconds int `yaml:"timeout_seconds"`

	// Team project configurations for routing deployments to team-specific projects
	// Deployments are sent to global project AND all matching team projects
	Teams []TeamYAMLConfig `yaml:"teams"`
}
