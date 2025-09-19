package logger

import (
	"go.uber.org/zap"
)

type DoraMetricsLogger struct{}

func NewDoraMetricsLogger() *DoraMetricsLogger {
	return &DoraMetricsLogger{}
}

func (d *DoraMetricsLogger) Infof(format string, args ...interface{}) {
	if Sugar != nil {
		Sugar.WithOptions(zap.AddCallerSkip(1)).Infof(format, args...)
	}
}

func (d *DoraMetricsLogger) Warnf(format string, args ...interface{}) {
	if Sugar != nil {
		Sugar.WithOptions(zap.AddCallerSkip(1)).Warnf(format, args...)
	}
}

func (d *DoraMetricsLogger) Errorf(format string, args ...interface{}) {
	if Sugar != nil {
		Sugar.WithOptions(zap.AddCallerSkip(1)).Errorf(format, args...)
	}
}

func (d *DoraMetricsLogger) Debugf(format string, args ...interface{}) {
	if Sugar != nil {
		Sugar.WithOptions(zap.AddCallerSkip(1)).Debugf(format, args...)
	}
}

var DoraMetrics = NewDoraMetricsLogger()
