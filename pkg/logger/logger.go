package logger

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Global logger instances for application-wide use
var (
	// Logger is the main Zap logger instance for structured logging
	Logger *zap.Logger
	// Sugar is the sugared logger for convenient printf-style logging
	Sugar *zap.SugaredLogger
)

// LogLevel represents the available logging levels for the application
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// Config holds configuration for the logger system
type Config struct {
	Level      LogLevel
	Format     string
	OutputPath string
}

// DefaultConfig returns a default logger configuration
func DefaultConfig() *Config {
	return &Config{
		Level:      LogLevelInfo,
		Format:     "console",
		OutputPath: "stdout",
	}
}

// Init initializes the global logger with the provided configuration
func Init(cfg *Config) error {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	level, err := parseLogLevel(cfg.Level)
	if err != nil {
		return err
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	if cfg.Format == "console" {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoderConfig.TimeKey = "timestamp"
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	}

	var encoder zapcore.Encoder
	switch cfg.Format {
	case "json":
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	case "console":
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	default:
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	var output zapcore.WriteSyncer
	switch cfg.OutputPath {
	case "stdout":
		output = zapcore.AddSync(os.Stdout)
	case "stderr":
		output = zapcore.AddSync(os.Stderr)
	default:
		file, err := os.OpenFile(cfg.OutputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		output = zapcore.AddSync(file)
	}

	core := zapcore.NewCore(encoder, output, level)
	Logger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	Sugar = Logger.Sugar()

	return nil
}

func parseLogLevel(level LogLevel) (zapcore.Level, error) {
	switch strings.ToLower(string(level)) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.InfoLevel, nil
	}
}

func Debug(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.WithOptions(zap.AddCallerSkip(1)).Debug(msg, fields...)
	}
}

func Info(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.WithOptions(zap.AddCallerSkip(1)).Info(msg, fields...)
	}
}

func Warn(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.WithOptions(zap.AddCallerSkip(1)).Warn(msg, fields...)
	}
}

func Error(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.WithOptions(zap.AddCallerSkip(1)).Error(msg, fields...)
	}
}

func Fatal(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.WithOptions(zap.AddCallerSkip(1)).Fatal(msg, fields...)
	}
}

func Debugf(template string, args ...interface{}) {
	if Sugar != nil {
		Sugar.WithOptions(zap.AddCallerSkip(1)).Debugf(template, args...)
	}
}

func Infof(template string, args ...interface{}) {
	if Sugar != nil {
		Sugar.WithOptions(zap.AddCallerSkip(1)).Infof(template, args...)
	}
}

func Warnf(template string, args ...interface{}) {
	if Sugar != nil {
		Sugar.WithOptions(zap.AddCallerSkip(1)).Warnf(template, args...)
	}
}

func Errorf(template string, args ...interface{}) {
	if Sugar != nil {
		Sugar.WithOptions(zap.AddCallerSkip(1)).Errorf(template, args...)
	}
}

func Fatalf(template string, args ...interface{}) {
	if Sugar != nil {
		Sugar.WithOptions(zap.AddCallerSkip(1)).Fatalf(template, args...)
	}
}
