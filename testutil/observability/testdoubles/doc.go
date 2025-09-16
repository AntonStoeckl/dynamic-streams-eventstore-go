// Package testdoubles provides test doubles (spies) for observability interfaces.
//
// This package contains spy implementations for OpenTelemetry-compatible observability
// interfaces used by the EventStore:
//   - MetricsCollectorSpy: captures metrics recording calls for verification
//   - TracingCollectorSpy: captures distributed tracing spans and events
//   - ContextualLoggerSpy: captures structured logging with context
//   - LogHandlerSpy: captures slog handler calls and attributes
//
// These test doubles enable comprehensive testing of observability instrumentation
// without requiring actual telemetry backends.
package testdoubles
