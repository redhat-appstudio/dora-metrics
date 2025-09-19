package logger

import (
	"github.com/redhat-appstudio/dora-metrics/internal/config"
)

func FromConfig(cfg *config.Config) *Config {
	loggerConfig := DefaultConfig()

	if cfg.LogLevel != "" {
		loggerConfig.Level = LogLevel(cfg.LogLevel)
	}

	if cfg.Environment == config.ValidEnvironmentProduction {
		loggerConfig.Format = "json"
	} else {
		loggerConfig.Format = "console"
	}

	loggerConfig.OutputPath = "stdout"

	return loggerConfig
}

func InitFromConfig(cfg *config.Config) error {
	loggerConfig := FromConfig(cfg)
	return Init(loggerConfig)
}
