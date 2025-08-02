package postgresengine

import (
	"context"
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
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

// Option defines a functional option for configuring EventStore.
type Option func(*EventStore) error

// WithTableName sets the table name for the EventStore.
func WithTableName(tableName string) Option {
	return func(es *EventStore) error {
		if tableName == "" {
			return eventstore.ErrEmptyEventsTableName
		}

		es.eventTableName = tableName

		return nil
	}
}

// WithLogger sets the logger for the EventStore.
// The logger will receive messages at different levels based on the logger's configured level:
//
// Debug level: SQL queries with execution timing (development use)
// Info level: Event counts, durations, concurrency conflicts (production-safe)
// Warn level: Non-critical issues like cleanup failures
// Error level: Critical failures that cause operation failures.
func WithLogger(logger Logger) Option {
	return func(es *EventStore) error {
		es.logger = logger
		return nil
	}
}

// WithMetrics sets the metricsCollector collector for the EventStore.
// The metricsCollector collector will receive performance and operational metricsCollector including
// query/append durations, event counts, concurrency conflicts, and database errors.
func WithMetrics(collector MetricsCollector) Option {
	return func(es *EventStore) error {
		es.metricsCollector = collector
		return nil
	}
}

// WithTracing sets the tracing collector for the EventStore.
// The tracing collector will receive distributed tracing information including
// span creation for query/append operations, context propagation, and error tracking.
func WithTracing(collector TracingCollector) Option {
	return func(es *EventStore) error {
		es.tracingCollector = collector
		return nil
	}
}

// WithContextualLogger sets the contextual logger for the EventStore.
// The contextual logger will receive log messages with context information including
// automatic trace/span correlation when tracing is enabled, enabling unified observability.
func WithContextualLogger(logger ContextualLogger) Option {
	return func(es *EventStore) error {
		es.contextualLogger = logger
		return nil
	}
}
