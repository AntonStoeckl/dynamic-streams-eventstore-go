package eventstore

import (
	"context"
	"time"
)

// Logger interface for SQL query logging, operational metricsCollector, warnings, and error reporting.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// MetricsCollector interface for collecting EventStore performance and operational metricsCollector.
type MetricsCollector interface {
	RecordDuration(metric string, duration time.Duration, labels map[string]string)
	IncrementCounter(metric string, labels map[string]string)
	RecordValue(metric string, value float64, labels map[string]string)
}

// SpanContext represents an active tracing span that can be finished and updated with attributes.
type SpanContext interface {
	SetStatus(status string)
	AddAttribute(key, value string)
}

// TracingCollector interface for collecting distributed tracing information from EventStore operations.
// This interface follows the same dependency-free pattern as MetricsCollector, allowing users to integrate
// with any tracing backend (OpenTelemetry, Jaeger, Zipkin, etc.) by implementing this interface.
type TracingCollector interface {
	StartSpan(ctx context.Context, name string, attrs map[string]string) (context.Context, SpanContext)
	FinishSpan(spanCtx SpanContext, status string, attrs map[string]string)
}

// ContextualLogger interface for context-aware logging with automatic trace correlation.
// This interface follows the same dependency-free pattern as MetricsCollector and TracingCollector,
// allowing users to integrate with any logging backend (OpenTelemetry, structured loggers, etc.)
// that supports context-based correlation and automatic trace/span ID inclusion.
type ContextualLogger interface {
	DebugContext(ctx context.Context, msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
}
