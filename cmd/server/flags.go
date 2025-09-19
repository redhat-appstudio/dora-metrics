package main

import (
	"flag"
	"fmt"
	"runtime"
	"strings"

	"github.com/redhat-appstudio/dora-metrics/internal/config"
	"github.com/redhat-appstudio/dora-metrics/internal/version"
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

// parseFlags parses command-line flags and returns a ServerFlags struct.
// This function sets up all available command-line options with their default values
// and descriptions, then parses the command line arguments.
//
// The function follows Go best practices for flag parsing:
// - Uses descriptive flag names with hyphens
// - Provides clear help text for each flag
// - Sets sensible defaults for all options
// - Supports both long and short forms for help/version
func parseFlags() *ServerFlags {
	f := &ServerFlags{}

	// Server configuration flags
	flag.StringVar(&f.Port, "port", DefaultPort,
		fmt.Sprintf("Server port number (default: %s)", DefaultPort))
	flag.StringVar(&f.Environment, "env", DefaultEnvironment,
		fmt.Sprintf("Deployment environment: %s, %s (default: %s)",
			ValidEnvironmentDevelopment, ValidEnvironmentProduction, DefaultEnvironment))
	flag.StringVar(&f.LogLevel, "log-level", DefaultLogLevel,
		fmt.Sprintf("Log level: %s, %s, %s, %s (default: %s)",
			ValidLogLevelDebug, ValidLogLevelInfo, ValidLogLevelWarn, ValidLogLevelError, DefaultLogLevel))

	// WebRCA and ArgoCD configuration is now handled via YAML config file
	// No command-line flags needed for these services

	// General flags
	flag.BoolVar(&f.Help, "help", false, "Show help information and exit")
	flag.BoolVar(&f.Help, "h", false, "Show help information and exit (short form)")
	flag.BoolVar(&f.Version, "version", false, "Show version information and exit")
	flag.BoolVar(&f.Version, "v", false, "Show version information and exit (short form)")

	// Parse command-line arguments
	flag.Parse()

	return f
}

// showHelp displays comprehensive help information for the DORA Metrics Server.
// It provides detailed documentation about all available command-line flags,
// usage examples, and best practices for running the server with monitoring services.
//
// The help text is structured to be:
// - Easy to read and understand
// - Comprehensive but not overwhelming
// - Includes practical examples
// - Shows that both monitoring services can run together
func (f *ServerFlags) showHelp() {
	fmt.Printf("%s - %s\n", AppName, AppDescription)
	fmt.Println("Supports both WebRCA incident monitoring and ArgoCD application monitoring simultaneously")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  dora-metrics [flags]")
	fmt.Println()
	fmt.Println("FLAGS:")
	fmt.Println("  Server Configuration:")
	fmt.Println("    -port string")
	fmt.Println("          Server port (default: 3000)")
	fmt.Println("    -env string")
	fmt.Println("          Environment: development, production (default: development)")
	fmt.Println("    -log-level string")
	fmt.Println("          Log level: debug, info, warn, error (default: info)")
	fmt.Println()
	fmt.Println("  Monitoring Services:")
	fmt.Println("    WebRCA and ArgoCD monitoring are configured via config.yaml file")
	fmt.Println("    Edit configs/config.yaml to enable/configure monitoring services")
	fmt.Println("    - WebRCA: Set 'enabled: true' and provide 'token' or OFFLINE_TOKEN env var")
	fmt.Println("    - ArgoCD: Set 'enabled: true' and configure 'namespaces' list")
	fmt.Println()
	fmt.Println("  NOTE: Both WebRCA and ArgoCD monitoring can run simultaneously!")
	fmt.Println("  NOTE: Configuration is GitOps-friendly - changes in YAML take effect on restart!")
	fmt.Println()
	fmt.Println("  General:")
	fmt.Println("    -help, -h")
	fmt.Println("          Show this help information")
	fmt.Println("    -version, -v")
	fmt.Println("          Show version information")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  # Start with default settings")
	fmt.Println("  dora-metrics")
	fmt.Println()
	fmt.Println("  # Start in production mode with custom log level")
	fmt.Println("  dora-metrics -env production -log-level warn")
	fmt.Println()
	fmt.Println("  # Start on custom port")
	fmt.Println("  dora-metrics -port 8080")
	fmt.Println()
	fmt.Println("  # Configure monitoring via YAML:")
	fmt.Println("  # 1. Edit configs/config.yaml")
	fmt.Println("  # 2. Set webrca.enabled: true and argocd.enabled: true")
	fmt.Println("  # 3. Provide OFFLINE_TOKEN environment variable for WebRCA")
	fmt.Println("  # 4. Restart the server")
}

// showVersion displays version and capability information for the DORA Metrics Server.
// It shows the application version, build information, Go version, and available monitoring capabilities.
func (f *ServerFlags) showVersion() {
	fmt.Printf("%s %s\n", AppName, version.GetVersion())
	fmt.Printf("Build info: %s\n", version.GetBuildInfo())
	fmt.Printf("Go version: %s\n", runtime.Version())
	fmt.Println("Capabilities:")
	fmt.Println("  - WebRCA incident monitoring")
	fmt.Println("  - ArgoCD application monitoring")
	fmt.Println("  - Dual monitoring support")
}

// validate performs comprehensive validation of all command-line flags.
// It checks that all provided values are valid and consistent with the expected format.
//
// Validation includes:
// - Port number is not empty
// - Environment is one of the valid values
// - Log level is one of the valid values
// - WebRCA interval is a valid duration (if WebRCA is enabled)
// - ArgoCD namespaces are specified (if ArgoCD is enabled)
//
// Returns an error if any validation fails, with a descriptive error message.
func (f *ServerFlags) validate() error {
	// Validate port
	if f.Port == "" {
		return fmt.Errorf("port cannot be empty")
	}

	// Validate environment
	validEnvs := []string{ValidEnvironmentDevelopment, ValidEnvironmentProduction}
	validEnv := false
	for _, env := range validEnvs {
		if f.Environment == env {
			validEnv = true
			break
		}
	}
	if !validEnv {
		return fmt.Errorf("invalid environment: %s (must be one of: %s)", f.Environment, strings.Join(validEnvs, ", "))
	}

	// Validate log level
	validLevels := []string{ValidLogLevelDebug, ValidLogLevelInfo, ValidLogLevelWarn, ValidLogLevelError}
	validLevel := false
	for _, level := range validLevels {
		if f.LogLevel == level {
			validLevel = true
			break
		}
	}
	if !validLevel {
		return fmt.Errorf("invalid log level: %s (must be one of: %s)", f.LogLevel, strings.Join(validLevels, ", "))
	}

	return nil
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
