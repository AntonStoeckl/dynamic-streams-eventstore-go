package shell

import (
	"context"
	"fmt"
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

const (
	// CommandHandlerDurationMetric tracks command handler execution duration (OpenTelemetry-compatible).
	CommandHandlerDurationMetric = "commandhandler_handle_duration_seconds"
	// CommandHandlerCallsMetric tracks total command handler calls.
	CommandHandlerCallsMetric = "commandhandler_handle_calls_total"
	// CommandHandlerIdempotentMetric tracks idempotent operations.
	CommandHandlerIdempotentMetric = "commandhandler_idempotent_operations_total"

	// StatusSuccess indicates successful command completion.
	StatusSuccess = "success"
	// StatusError indicates command processing error.
	StatusError = "error"
	// StatusIdempotent indicates no state change was needed.
	StatusIdempotent = "idempotent"

	// LogMsgCommandStarted is logged when command processing begins.
	LogMsgCommandStarted = "command handler started"
	// LogMsgCommandCompleted is logged when command processing succeeds.
	LogMsgCommandCompleted = "command handler completed"
	// LogMsgCommandFailed is logged when command processing fails.
	LogMsgCommandFailed = "command handler failed"

	// LogAttrCommandType identifies the command type in logs.
	LogAttrCommandType = "command_type"
	// LogAttrStatus indicates the command processing status.
	LogAttrStatus = "status"
	// LogAttrDurationMS indicates the processing duration in milliseconds.
	LogAttrDurationMS = "duration_ms"
	// LogAttrBusinessOutcome classifies the business result.
	LogAttrBusinessOutcome = "business_outcome"
	// LogAttrEventCount indicates the number of events processed.
	LogAttrEventCount = "event_count"
	// LogAttrError contains error details.
	LogAttrError = "error"

	// SpanNameCommandHandle is the tracing span name for command handling.
	SpanNameCommandHandle = "commandhandler.handle"
)

// Interface aliases for convenience when using command handler observability.
// These match the EventStore observability interfaces for consistency.

// MetricsCollector interface for collecting command handler performance metrics.
type MetricsCollector = eventstore.MetricsCollector

// ContextualMetricsCollector extends MetricsCollector with context-aware methods.
type ContextualMetricsCollector = eventstore.ContextualMetricsCollector

// TracingCollector interface for distributed tracing in command handlers.
type TracingCollector = eventstore.TracingCollector

// SpanContext represents an active tracing span.
type SpanContext = eventstore.SpanContext

// ContextualLogger interface for context-aware logging in command handlers.
type ContextualLogger = eventstore.ContextualLogger

// Logger interface for basic logging in command handlers.
type Logger = eventstore.Logger

// ClassifyBusinessOutcome analyzes domain events to determine the business outcome.
// This function uses the IsErrorEvent() method to classify the result of command processing.
func ClassifyBusinessOutcome(eventsToAppend core.DomainEvents) string {
	if len(eventsToAppend) == 0 {
		return StatusIdempotent
	}

	for _, event := range eventsToAppend {
		if event.IsErrorEvent() {
			return StatusError
		}
	}

	return StatusSuccess
}

// BuildCommandLabels creates standard metric labels for command handler operations.
func BuildCommandLabels(commandType, status string) map[string]string {
	return map[string]string{
		LogAttrCommandType: commandType,
		LogAttrStatus:      status,
	}
}

// ToMilliseconds converts a time.Duration to float64 milliseconds with precision.
func ToMilliseconds(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / 1e6
}

// RecordCommandMetrics is a helper function to record all relevant metrics for a command operation.
// It handles both context-aware and basic metrics collectors automatically.
func RecordCommandMetrics(
	ctx context.Context,
	collector MetricsCollector,
	commandType string,
	status string,
	duration time.Duration,
) {
	if collector == nil {
		return
	}

	labels := BuildCommandLabels(commandType, status)

	// Record duration metric
	if contextualCollector, ok := collector.(ContextualMetricsCollector); ok {
		contextualCollector.RecordDurationContext(ctx, CommandHandlerDurationMetric, duration, labels)
		contextualCollector.IncrementCounterContext(ctx, CommandHandlerCallsMetric, labels)
	} else {
		collector.RecordDuration(CommandHandlerDurationMetric, duration, labels)
		collector.IncrementCounter(CommandHandlerCallsMetric, labels)
	}

	// Record idempotent operations separately
	if status == StatusIdempotent {
		idempotentLabels := BuildCommandLabels(commandType, StatusIdempotent)
		if contextualCollector, ok := collector.(ContextualMetricsCollector); ok {
			contextualCollector.IncrementCounterContext(ctx, CommandHandlerIdempotentMetric, idempotentLabels)
		} else {
			collector.IncrementCounter(CommandHandlerIdempotentMetric, idempotentLabels)
		}
	}
}

// StartCommandSpan starts a distributed tracing span for command operations.
// Returns the updated context and span context, or original context and nil if tracing is disabled.
func StartCommandSpan(
	ctx context.Context,
	tracingCollector TracingCollector,
	commandType string,
) (context.Context, SpanContext) {
	if tracingCollector == nil {
		return ctx, nil
	}

	attrs := map[string]string{
		LogAttrCommandType: commandType,
	}

	return tracingCollector.StartSpan(ctx, SpanNameCommandHandle, attrs)
}

// FinishCommandSpan completes a distributed tracing span with the operation outcome.
func FinishCommandSpan(
	tracingCollector TracingCollector,
	span SpanContext,
	status string,
	duration time.Duration,
	err error,
) {
	if tracingCollector == nil || span == nil {
		return
	}

	attrs := map[string]string{
		LogAttrStatus:     status,
		LogAttrDurationMS: formatDurationMS(duration),
	}

	if err != nil {
		attrs[LogAttrError] = err.Error()
	}

	tracingCollector.FinishSpan(span, status, attrs)
}

// LogCommandStart logs the beginning of command processing.
func LogCommandStart(
	ctx context.Context,
	logger Logger,
	contextualLogger ContextualLogger,
	commandType string,
) {
	if contextualLogger != nil {
		contextualLogger.InfoContext(ctx, LogMsgCommandStarted, LogAttrCommandType, commandType)
	} else if logger != nil {
		logger.Info(LogMsgCommandStarted, LogAttrCommandType, commandType)
	}
}

// LogCommandSuccess logs successful command completion.
func LogCommandSuccess(
	ctx context.Context,
	logger Logger,
	contextualLogger ContextualLogger,
	commandType string,
	businessOutcome string,
	duration time.Duration,
) {
	args := []any{
		LogAttrCommandType, commandType,
		LogAttrBusinessOutcome, businessOutcome,
		LogAttrDurationMS, ToMilliseconds(duration),
	}

	if contextualLogger != nil {
		contextualLogger.InfoContext(ctx, LogMsgCommandCompleted, args...)
	} else if logger != nil {
		logger.Info(LogMsgCommandCompleted, args...)
	}
}

// LogCommandError logs command processing errors.
func LogCommandError(
	ctx context.Context,
	logger Logger,
	contextualLogger ContextualLogger,
	commandType string,
	err error,
) {
	args := []any{
		LogAttrCommandType, commandType,
		LogAttrError, err.Error(),
	}

	if contextualLogger != nil {
		contextualLogger.ErrorContext(ctx, LogMsgCommandFailed, args...)
	} else if logger != nil {
		logger.Error(LogMsgCommandFailed, args...)
	}
}

// formatDurationMS formats duration in milliseconds for span attributes.
func formatDurationMS(duration time.Duration) string {
	return formatFloat(ToMilliseconds(duration), 2)
}

// formatFloat formats a float64 with specified precision.
func formatFloat(value float64, _ int) string {
	return fmt.Sprintf("%.2f", value)
}