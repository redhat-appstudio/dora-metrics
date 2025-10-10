package config

// Default configuration values
const (
	// DefaultPort is the default HTTP server port
	DefaultPort = "3000"

	// DefaultEnvironment is the default deployment environment
	DefaultEnvironment = "development"

	// DefaultLogLevel is the default logging level
	DefaultLogLevel = "info"
)

// Valid environment values
const (
	ValidEnvironmentDevelopment = "development"
	ValidEnvironmentProduction  = "production"
)

// Valid log level values
const (
	ValidLogLevelDebug = "debug"
	ValidLogLevelInfo  = "info"
	ValidLogLevelWarn  = "warn"
	ValidLogLevelError = "error"
)
