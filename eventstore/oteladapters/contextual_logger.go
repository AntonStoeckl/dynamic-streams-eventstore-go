// Package oteladapters provides OpenTelemetry adapters for the eventstore observability interfaces.
// These adapters enable seamless integration with OpenTelemetry for users who want
// plug-and-play observability without implementing the interfaces themselves.
package oteladapters

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/log"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

// SlogBridgeLogger implements eventstore.ContextualLogger using the OpenTelemetry slog bridge.
// This is the recommended implementation as it provides automatic trace correlation
// and works seamlessly with Go's standard log/slog package.
type SlogBridgeLogger struct {
	logger *slog.Logger
}

// NewSlogBridgeLogger creates a new contextual logger using the OpenTelemetry slog bridge.
// It creates an OpenTelemetry-enabled logger with automatic trace correlation.
// The logger uses the global OpenTelemetry LoggerProvider.
func NewSlogBridgeLogger(name string) *SlogBridgeLogger {
	// Create OpenTelemetry slog handler with automatic trace correlation
	logger := otelslog.NewLogger(name)
	return &SlogBridgeLogger{logger: logger}
}

// NewSlogBridgeLoggerWithHandler creates a new contextual logger using the provided slog.Handler.
// Note: This does NOT add OpenTelemetry trace correlation - it uses the handler as-is.
// For trace correlation, use NewSlogBridgeLogger() instead.
// This function is provided for compatibility when you need to use a specific slog.Handler.
func NewSlogBridgeLoggerWithHandler(name string, handler slog.Handler) *SlogBridgeLogger {
	// Use the provided handler directly - no OpenTelemetry integration
	logger := slog.New(handler)
	return &SlogBridgeLogger{logger: logger}
}

// DebugContext logs a debug message with context.
func (l *SlogBridgeLogger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.logger.DebugContext(ctx, msg, args...)
}

// InfoContext logs an info message with context.
func (l *SlogBridgeLogger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.logger.InfoContext(ctx, msg, args...)
}

// WarnContext logs a warning message with context.
func (l *SlogBridgeLogger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.logger.WarnContext(ctx, msg, args...)
}

// ErrorContext logs an error message with context.
func (l *SlogBridgeLogger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.logger.ErrorContext(ctx, msg, args...)
}

// Ensure SlogBridgeLogger implements eventstore.ContextualLogger.
var _ eventstore.ContextualLogger = (*SlogBridgeLogger)(nil)

// OTelLogger implements eventstore.ContextualLogger using the OpenTelemetry logging API directly.
// This provides more control over the logging implementation but requires more setup.
// Use this if you need direct control over OpenTelemetry log records.
type OTelLogger struct {
	logger log.Logger
}

// NewOTelLogger creates a new contextual logger using the OpenTelemetry logging API directly.
// This gives you more control over log record creation but requires manual setup of the logger.
func NewOTelLogger(logger log.Logger) *OTelLogger {
	return &OTelLogger{logger: logger}
}

// DebugContext logs a debug message with context using OpenTelemetry log API.
func (l *OTelLogger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.emit(ctx, log.SeverityDebug, msg, args...)
}

// InfoContext logs an info message with context using OpenTelemetry log API.
func (l *OTelLogger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.emit(ctx, log.SeverityInfo, msg, args...)
}

// WarnContext logs a warning message with context using OpenTelemetry log API.
func (l *OTelLogger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.emit(ctx, log.SeverityWarn, msg, args...)
}

// ErrorContext logs an error message with context using OpenTelemetry log API.
func (l *OTelLogger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.emit(ctx, log.SeverityError, msg, args...)
}

// emit creates and emits an OpenTelemetry log record with the specified severity.
func (l *OTelLogger) emit(ctx context.Context, severity log.Severity, msg string, args ...any) {
	record := log.Record{}
	record.SetSeverity(severity)
	record.SetBody(log.StringValue(msg))

	// Convert args to OpenTelemetry log attributes
	// Args come in key-value pairs like slog
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			if key, ok := args[i].(string); ok {
				value := args[i+1]
				record.AddAttributes(log.String(key, stringValue(value)))
			}
		}
	}

	l.logger.Emit(ctx, record)
}

// stringValue converts any value to string for OpenTelemetry attributes.
func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return slog.AnyValue(v).String()
}

// Ensure OTelLogger implements eventstore.ContextualLogger.
var _ eventstore.ContextualLogger = (*OTelLogger)(nil)
