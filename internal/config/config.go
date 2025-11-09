package config

import (
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	// Cache for configuration to avoid repeated file reads
	configCache *Config
	configOnce  sync.Once
)

// Load creates a new Config instance using only YAML configuration.
// This is a convenience function that calls LoadWithFlags with nil flags,
// making it suitable for applications that don't use command-line flags.
//
// Returns a Config instance loaded from configs/config.yaml.
func Load() *Config {
	return LoadWithFlags(nil)
}

// LoadCached creates a cached Config instance using only YAML configuration.
// This function caches the configuration after the first load for better performance.
//
// Returns a cached Config instance loaded from configs/config.yaml.
func LoadCached() *Config {
	configOnce.Do(func() {
		configCache = LoadWithFlags(nil)
	})
	return configCache
}

// Flags defines the interface for command-line flag access.
// It provides methods to retrieve server configuration flags while keeping
// WebRCA and ArgoCD configuration YAML-only for GitOps compliance.
type Flags interface {
	GetPort() string
	GetEnvironment() string
	GetLogLevel() string
	// WebRCA and ArgoCD configuration is now YAML-only for GitOps approach
}

// LoadWithFlags creates a new Config instance by loading configuration from
// YAML files and applying command-line flag overrides where appropriate.
//
// Configuration precedence (highest to lowest):
// 1. Command-line flags (for server settings only)
// 2. Environment variables
// 3. YAML configuration files
// 4. Default values
//
// WebRCA and ArgoCD configuration is YAML-only for GitOps compliance.
// The function validates required configuration and returns appropriate defaults.
//
// Parameters:
//   - flgs: Command-line flags interface (can be nil)
//
// Returns a fully configured Config instance ready for use.
func LoadWithFlags(flgs Flags) *Config {
	yamlConfig := loadFromYAML()

	// WebRCA interval from YAML only - no defaults
	webrcaIntervalStr := yamlConfig.WebRCA.Interval
	if webrcaIntervalStr == "" {
		webrcaIntervalStr = getEnv("WEBRCA_INTERVAL", "30m")
	}
	webrcaInterval, err := time.ParseDuration(webrcaIntervalStr)
	if err != nil {
		webrcaInterval = 30 * time.Minute
	}

	// ArgoCD namespaces from YAML only - no defaults
	argocdNamespaces := yamlConfig.ArgoCD.Namespaces

	// ArgoCD components to ignore from YAML only
	argocdComponentsToIgnore := yamlConfig.ArgoCD.ComponentsToIgnore

	// ArgoCD known clusters from YAML only - no defaults
	argocdKnownClusters := yamlConfig.ArgoCD.KnownClusters

	// Token from environment or YAML only
	token := getEnv("OFFLINE_TOKEN", yamlConfig.WebRCA.Token)

	port := getEnv("PORT", yamlConfig.Server.Port)
	if port == "" {
		port = DefaultPort
	}
	if flgs != nil && flgs.GetPort() != "" {
		port = flgs.GetPort()
	}

	environment := getEnv("ENVIRONMENT", yamlConfig.Server.Environment)
	if environment == "" {
		environment = DefaultEnvironment
	}
	if flgs != nil && flgs.GetEnvironment() != "" {
		environment = flgs.GetEnvironment()
	}

	logLevel := getEnv("LOG_LEVEL", yamlConfig.Server.LogLevel)
	if logLevel == "" {
		logLevel = DefaultLogLevel
	}
	if flgs != nil && flgs.GetLogLevel() != "" {
		logLevel = flgs.GetLogLevel()
	}

	// WebRCA configuration - YAML only (GitOps approach)
	webrcaEnabled := yamlConfig.WebRCA.Enabled
	webrcaAPIURL := yamlConfig.WebRCA.APIURL
	// No default URL - must be specified in YAML if WebRCA is enabled

	// ArgoCD configuration - YAML only (GitOps approach)
	argocdEnabled := yamlConfig.ArgoCD.Enabled

	// Redis configuration - support environment variables
	redisConfig := yamlConfig.Storage.Redis
	redisHost := getEnv("REDIS_HOST", "")
	redisPort := getEnv("REDIS_PORT", "")
	redisPassword := getEnv("REDIS_PASSWORD", redisConfig.Password)

	// Build Redis address from environment variables or use YAML config
	redisAddress := redisConfig.Address
	if redisHost != "" && redisPort != "" {
		redisAddress = redisHost + ":" + redisPort
	} else if redisHost != "" {
		redisAddress = redisHost + ":6379" // Default port
	}

	return &Config{
		Port:        port,
		Environment: environment,
		LogLevel:    logLevel,
		WebRCA: WebRCAConfig{
			Enabled:  webrcaEnabled,
			APIURL:   webrcaAPIURL,
			Token:    token,
			Interval: webrcaInterval,
		},
		ArgoCD: ArgoCDConfig{
			Enabled:           argocdEnabled,
			Namespaces:        argocdNamespaces,
			ComponentsToIgnore: argocdComponentsToIgnore,
			KnownClusters:     argocdKnownClusters,
		},
		Storage: StorageConfig{
			Redis: RedisYAMLConfig{
				Enabled:   redisConfig.Enabled,
				Address:   redisAddress,
				Password:  redisPassword,
				Database:  redisConfig.Database,
				KeyPrefix: redisConfig.KeyPrefix,
			},
		},
		Integration: IntegrationConfig{
			DevLake: DevLakeConfig{
				Enabled:        yamlConfig.Integration.DevLake.Enabled,
				BaseURL:        yamlConfig.Integration.DevLake.BaseURL,
				ProjectID:      yamlConfig.Integration.DevLake.ProjectID,
				TimeoutSeconds: yamlConfig.Integration.DevLake.TimeoutSeconds,
				Teams:          convertTeamYAMLToConfig(yamlConfig.Integration.DevLake.Teams),
			},
		},
	}
}

func loadFromYAML() *YAMLConfig {
	config := &YAMLConfig{}
	data, err := os.ReadFile("configs/config.yaml")
	if err != nil {
		return config
	}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return config
	}
	return config
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// convertTeamYAMLToConfig converts TeamYAMLConfig slice to TeamConfig slice
func convertTeamYAMLToConfig(yamlTeams []TeamYAMLConfig) []TeamConfig {
	if yamlTeams == nil {
		return nil
	}
	teams := make([]TeamConfig, len(yamlTeams))
	for i, yamlTeam := range yamlTeams {
		teams[i] = TeamConfig{
			Name:            yamlTeam.Name,
			ProjectID:       yamlTeam.ProjectID,
			ArgocdComponents: yamlTeam.ArgocdComponents,
		}
	}
	return teams
}
