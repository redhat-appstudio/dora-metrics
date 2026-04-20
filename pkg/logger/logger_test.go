package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, LogLevelInfo, cfg.Level)
	assert.Equal(t, "console", cfg.Format)
	assert.Equal(t, "stdout", cfg.OutputPath)
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name          string
		level         LogLevel
		expectedLevel zapcore.Level
		expectError   bool
	}{
		{
			name:          "debug level",
			level:         LogLevelDebug,
			expectedLevel: zapcore.DebugLevel,
			expectError:   false,
		},
		{
			name:          "info level",
			level:         LogLevelInfo,
			expectedLevel: zapcore.InfoLevel,
			expectError:   false,
		},
		{
			name:          "warn level",
			level:         LogLevelWarn,
			expectedLevel: zapcore.WarnLevel,
			expectError:   false,
		},
		{
			name:          "warning alias",
			level:         "warning",
			expectedLevel: zapcore.WarnLevel,
			expectError:   false,
		},
		{
			name:          "error level",
			level:         LogLevelError,
			expectedLevel: zapcore.ErrorLevel,
			expectError:   false,
		},
		{
			name:          "uppercase debug",
			level:         "DEBUG",
			expectedLevel: zapcore.DebugLevel,
			expectError:   false,
		},
		{
			name:          "mixed case info",
			level:         "InFo",
			expectedLevel: zapcore.InfoLevel,
			expectError:   false,
		},
		{
			name:          "invalid level defaults to info",
			level:         "invalid",
			expectedLevel: zapcore.InfoLevel,
			expectError:   false,
		},
		{
			name:          "empty level defaults to info",
			level:         "",
			expectedLevel: zapcore.InfoLevel,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, err := parseLogLevel(tt.level)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedLevel, level)
			}
		})
	}
}

func TestInit(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name:        "nil config uses defaults",
			config:      nil,
			expectError: false,
		},
		{
			name: "console format to stdout",
			config: &Config{
				Level:      LogLevelDebug,
				Format:     "console",
				OutputPath: "stdout",
			},
			expectError: false,
		},
		{
			name: "json format to stderr",
			config: &Config{
				Level:      LogLevelInfo,
				Format:     "json",
				OutputPath: "stderr",
			},
			expectError: false,
		},
		{
			name: "warn level",
			config: &Config{
				Level:      LogLevelWarn,
				Format:     "console",
				OutputPath: "stdout",
			},
			expectError: false,
		},
		{
			name: "error level",
			config: &Config{
				Level:      LogLevelError,
				Format:     "json",
				OutputPath: "stdout",
			},
			expectError: false,
		},
		{
			name: "default format when unknown",
			config: &Config{
				Level:      LogLevelInfo,
				Format:     "unknown-format",
				OutputPath: "stdout",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Init(tt.config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, Logger, "Logger should be initialized")
				assert.NotNil(t, Sugar, "Sugar logger should be initialized")
			}
		})
	}
}

func TestLoggingFunctions_NilLogger(t *testing.T) {
	// Test that logging functions don't panic when logger is nil
	originalLogger := Logger
	originalSugar := Sugar
	defer func() {
		Logger = originalLogger
		Sugar = originalSugar
	}()

	Logger = nil
	Sugar = nil

	// These should not panic
	assert.NotPanics(t, func() { Debug("test") })
	assert.NotPanics(t, func() { Info("test") })
	assert.NotPanics(t, func() { Warn("test") })
	assert.NotPanics(t, func() { Error("test") })

	assert.NotPanics(t, func() { Debugf("test %s", "arg") })
	assert.NotPanics(t, func() { Infof("test %s", "arg") })
	assert.NotPanics(t, func() { Warnf("test %s", "arg") })
	assert.NotPanics(t, func() { Errorf("test %s", "arg") })
}

func TestLoggingFunctions_WithLogger(t *testing.T) {
	// Initialize logger for testing
	err := Init(&Config{
		Level:      LogLevelDebug,
		Format:     "json",
		OutputPath: "stdout",
	})
	require.NoError(t, err)

	// Test that logging functions don't panic with initialized logger
	assert.NotPanics(t, func() { Debug("debug message") })
	assert.NotPanics(t, func() { Info("info message") })
	assert.NotPanics(t, func() { Warn("warn message") })
	assert.NotPanics(t, func() { Error("error message") })

	assert.NotPanics(t, func() { Debugf("debug %s", "formatted") })
	assert.NotPanics(t, func() { Infof("info %s", "formatted") })
	assert.NotPanics(t, func() { Warnf("warn %s", "formatted") })
	assert.NotPanics(t, func() { Errorf("error %s", "formatted") })
}

func TestLogLevel_Constants(t *testing.T) {
	// Verify the log level constants are defined correctly
	assert.Equal(t, LogLevel("debug"), LogLevelDebug)
	assert.Equal(t, LogLevel("info"), LogLevelInfo)
	assert.Equal(t, LogLevel("warn"), LogLevelWarn)
	assert.Equal(t, LogLevel("error"), LogLevelError)
}
