package main

import (
	"github.com/redhat-appstudio/dora-metrics/internal/config"
)

// Default values for server configuration
const (
	DefaultPort        = config.DefaultPort
	DefaultEnvironment = config.DefaultEnvironment
	DefaultLogLevel    = config.DefaultLogLevel
)

// WebRCA and ArgoCD configuration is now YAML-only
// No default values needed for command-line flags

// Valid values for validation
const (
	ValidEnvironmentDevelopment = config.ValidEnvironmentDevelopment
	ValidEnvironmentProduction  = config.ValidEnvironmentProduction

	ValidLogLevelDebug = config.ValidLogLevelDebug
	ValidLogLevelInfo  = config.ValidLogLevelInfo
	ValidLogLevelWarn  = config.ValidLogLevelWarn
	ValidLogLevelError = config.ValidLogLevelError
)

// Help and version text
const (
	AppName        = "DORA Metrics Server"
	AppDescription = "A professional Go Fiber server with dual monitoring capabilities"
)

// ServerFlags holds all command-line flags for the DORA Metrics Server.
// It provides a structured way to parse and validate command-line arguments
// for server configuration. WebRCA and ArgoCD monitoring services are
// configured exclusively through YAML files following GitOps principles.
type ServerFlags struct {
	// Server configuration flags
	// HTTP server port number
	Port string
	// Deployment environment (development/production/staging)
	Environment string
	// Logging verbosity level (debug/info/warn/error)
	LogLevel string

	// WebRCA and ArgoCD configuration is now YAML-only for GitOps approach
	// These services are configured through config.yaml file

	// General flags
	// Show help information and exit
	Help bool
	// Show version information and exit
	Version bool
}

// Interface methods for config package
// These methods implement the config.Flags interface to allow the config package
// to access flag values without depending on the specific flag implementation.

// GetPort returns the configured server port number.
func (f *ServerFlags) GetPort() string {
	return f.Port
}

// GetEnvironment returns the configured deployment environment.
func (f *ServerFlags) GetEnvironment() string {
	return f.Environment
}

// GetLogLevel returns the configured logging verbosity level.
func (f *ServerFlags) GetLogLevel() string {
	return f.LogLevel
}

// WebRCA and ArgoCD configuration methods removed - now YAML-only
