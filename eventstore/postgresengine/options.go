package postgresengine

import (
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

// Logger is an alias for eventstore.Logger for convenience when using postgresengine.
// It provides methods for SQL query logging, operational metrics, warnings, and error reporting.
type Logger = eventstore.Logger

// MetricsCollector is an alias for eventstore.MetricsCollector for convenience when using postgresengine.
// It provides methods for collecting EventStore performance and operational metrics.
type MetricsCollector = eventstore.MetricsCollector

// SpanContext is an alias for eventstore.SpanContext for convenience when using postgresengine.
// It represents an active tracing span that can be finished and updated with attributes.
type SpanContext = eventstore.SpanContext

// TracingCollector is an alias for eventstore.TracingCollector for convenience when using postgresengine.
// It provides methods for collecting distributed tracing information from EventStore operations.
type TracingCollector = eventstore.TracingCollector

// ContextualLogger is an alias for eventstore.ContextualLogger for convenience when using postgresengine.
// It provides methods for context-aware logging with automatic trace correlation.
type ContextualLogger = eventstore.ContextualLogger

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
func WithLogger(logger eventstore.Logger) Option {
	return func(es *EventStore) error {
		es.logger = logger
		return nil
	}
}

// WithMetrics sets the metricsCollector collector for the EventStore.
// The metricsCollector collector will receive performance and operational metricsCollector including
// query/append durations, event counts, concurrency conflicts, and database errors.
func WithMetrics(collector eventstore.MetricsCollector) Option {
	return func(es *EventStore) error {
		es.metricsCollector = collector
		return nil
	}
}

// WithTracing sets the tracing collector for the EventStore.
// The tracing collector will receive distributed tracing information including
// span creation for query/append operations, context propagation, and error tracking.
func WithTracing(collector eventstore.TracingCollector) Option {
	return func(es *EventStore) error {
		es.tracingCollector = collector
		return nil
	}
}

// WithContextualLogger sets the contextual logger for the EventStore.
// The contextual logger will receive log messages with context information including
// automatic trace/span correlation when tracing is enabled, enabling unified observability.
func WithContextualLogger(logger eventstore.ContextualLogger) Option {
	return func(es *EventStore) error {
		es.contextualLogger = logger
		return nil
	}
}
