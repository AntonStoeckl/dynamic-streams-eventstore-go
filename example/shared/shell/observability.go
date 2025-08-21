package shell

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

const (
	// CommandHandlerDurationMetric tracks command handler execution duration (OpenTelemetry-compatible).
	CommandHandlerDurationMetric = "commandhandler_handle_duration_seconds"

	// CommandHandlerCallsMetric tracks total command handler calls.
	CommandHandlerCallsMetric = "commandhandler_handle_calls_total"

	// CommandHandlerIdempotentMetric tracks idempotent operations.
	CommandHandlerIdempotentMetric = "commandhandler_idempotent_operations_total"

	// CommandHandlerCanceledMetric tracks canceled operations.
	CommandHandlerCanceledMetric = "commandhandler_canceled_operations_total"

	// CommandHandlerTimeoutMetric tracks timeout operations.
	CommandHandlerTimeoutMetric = "commandhandler_timeout_operations_total"

	// CommandHandlerConcurrencyConflictMetric tracks concurrency conflict operations.
	CommandHandlerConcurrencyConflictMetric = "commandhandler_concurrency_conflicts_total"

	// QueryHandlerDurationMetric tracks query handler execution duration (OpenTelemetry-compatible).
	QueryHandlerDurationMetric = "queryhandler_handle_duration_seconds"

	// QueryHandlerCallsMetric tracks total query handler calls.
	QueryHandlerCallsMetric = "queryhandler_handle_calls_total"

	// QueryHandlerCanceledMetric tracks canceled query operations.
	QueryHandlerCanceledMetric = "queryhandler_canceled_operations_total"

	// QueryHandlerTimeoutMetric tracks timeout query operations.
	QueryHandlerTimeoutMetric = "queryhandler_timeout_operations_total"

	// CommandHandlerRetriesMetric tracks retry attempts in command handlers.
	//
	// Labels:
	//   - command_type: Type of command being retried (e.g., "CancelReaderContract")
	//   - attempt_number: Which retry attempt (1, 2, 3, 4, 5)
	//   - error_type: Category of error causing retry (e.g., "concurrency_conflict")
	//
	// Cardinality: O(command_types × max_attempts × error_types)
	// Expected: ~6 commands × 5 attempts × 3 error types = ~90 series
	//
	// Use cases:
	//   - Alert on high retry rates: rate(commandhandler_retries_total[5m])
	//   - Retry success rate: (total commands - max_retries_reached) / total commands
	CommandHandlerRetriesMetric = "commandhandler_retries_total"

	// CommandHandlerRetryDelayMetric tracks retry delays in command handlers.
	//
	// Labels:
	//   - command_type: Type of command being retried
	//   - attempt_number: Which retry attempt (1, 2, 3, 4, 5)
	//
	// Cardinality: O(command_types × max_attempts)
	// Expected: ~6 commands × 5 attempts = ~30 series
	//
	// Use cases:
	//   - Monitor backoff behavior: histogram_quantile(0.95, commandhandler_retry_delay_seconds)
	//   - Detect thundering herd: sudden spikes in delay distribution
	CommandHandlerRetryDelayMetric = "commandhandler_retry_delay_seconds"

	// CommandHandlerMaxRetriesReachedMetric tracks when max retries are exhausted.
	//
	// Labels:
	//   - command_type: Type of command that exhausted retries
	//   - final_error_type: Error type that caused final failure
	//
	// Cardinality: O(command_types × error_types)
	// Expected: ~6 commands × 3 error types = ~18 series
	//
	// Use cases:
	//   - Alert on retry exhaustion: increase(commandhandler_max_retries_reached_total[5m]) > 0
	//   - Identify problematic commands: rate(commandhandler_max_retries_reached_total[1h]) by (command_type)
	CommandHandlerMaxRetriesReachedMetric = "commandhandler_max_retries_reached_total"

	// CommandHandlerComponentDurationMetric tracks component-level timing in command handlers.
	//
	// Labels:
	//   - command_type: Type of command (e.g., "LendBookCopy")
	//   - component: Processing phase (query, unmarshal, decide, append)
	//   - status: Execution status (success, error, canceled, timeout)
	//
	// Components:
	//   - query: EventStore.Query execution time
	//   - unmarshal: DomainEventsFrom deserialization time
	//   - decide: Business logic execution time
	//   - append: EventStore.Append execution time
	CommandHandlerComponentDurationMetric = "commandhandler_component_duration_seconds"

	// QueryHandlerComponentDurationMetric tracks component-level timing in query handlers.
	//
	// Labels:
	//   - query_type: Type of query (e.g., "BooksLentByReader")
	//   - component: Processing phase (query, unmarshal, projection)
	//   - status: Execution status (success, error, canceled, timeout)
	//
	// Components:
	//   - query: EventStore.Query execution time
	//   - unmarshal: DomainEventsFrom deserialization time
	//   - projection: Business logic execution time
	QueryHandlerComponentDurationMetric = "queryhandler_component_duration_seconds"

	// StatusSuccess indicates successful command completion.
	StatusSuccess = "success"

	// StatusError indicates a command processing error.
	StatusError = "error"

	// StatusIdempotent indicates no state change was needed.
	StatusIdempotent = "idempotent"

	// StatusCanceled indicates the operation was canceled due to context cancellation.
	StatusCanceled = "canceled"

	// StatusTimeout indicates the operation timed out due to context deadline exceeded.
	StatusTimeout = "timeout"

	// StatusConcurrencyConflict indicates the operation failed due to optimistic concurrency control.
	StatusConcurrencyConflict = "concurrency_conflict"

	// ComponentQuery identifies the query phase in timing metrics.
	ComponentQuery = "query"

	// ComponentUnmarshal identifies the unmarshal phase in timing metrics.
	ComponentUnmarshal = "unmarshal"

	// ComponentDecide identifies the decide phase in timing metrics.
	ComponentDecide = "decide"

	// ComponentAppend identifies the append phase in timing metrics.
	ComponentAppend = "append"

	// ComponentProjection identifies the projection phase in timing metrics.
	ComponentProjection = "projection"

	// ComponentSnapshotLoad identifies the snapshot loading phase in snapshot-aware query handlers.
	ComponentSnapshotLoad = "snapshot_load"

	// ComponentFilterReopen identifies the filter reopening phase in snapshot-aware query handlers.
	ComponentFilterReopen = "filter_reopen"

	// ComponentIncrementalQuery identifies the incremental query phase in snapshot-aware query handlers.
	ComponentIncrementalQuery = "incremental_query"

	// ComponentSnapshotDeserialize identifies the snapshot deserialization phase in snapshot-aware query handlers.
	ComponentSnapshotDeserialize = "snapshot_deserialize"

	// ComponentSnapshotSave identifies the asynchronous snapshot saving phase in snapshot-aware query handlers.
	ComponentSnapshotSave = "snapshot_save"

	// ComponentIncrementalProjection identifies the incremental projection phase in snapshot-aware query handlers.
	ComponentIncrementalProjection = "incremental_projection"

	// LogMsgCommandStarted is logged when command processing begins.
	LogMsgCommandStarted = "command handler started"

	// LogMsgCommandCompleted is logged when command processing succeeds.
	LogMsgCommandCompleted = "command handler completed"

	// LogMsgCommandFailed is logged when command processing fails.
	LogMsgCommandFailed = "command handler failed"

	// LogMsgQueryStarted is logged when query processing begins.
	LogMsgQueryStarted = "query handler started"

	// LogMsgQueryCompleted is logged when query processing succeeds.
	LogMsgQueryCompleted = "query handler completed"

	// LogMsgQueryFailed is logged when query processing fails.
	LogMsgQueryFailed = "query handler failed"

	// LogMsgSnapshotQuerySuccess is logged when snapshot-aware query processing succeeds.
	LogMsgSnapshotQuerySuccess = "snapshot-aware query completed"

	// LogMsgSnapshotFallback is logged when snapshot loading fails and falls back to base handler.
	LogMsgSnapshotFallback = "snapshot fallback to base handler"

	// LogMsgSnapshotHit is logged when the snapshot is successfully loaded and used for the incremental query.
	LogMsgSnapshotHit = "snapshot hit: incremental query"

	// LogMsgSnapshotMiss is logged when snapshot loading fails.
	LogMsgSnapshotMiss = "snapshot miss: falling back to base handler"

	// LogMsgSnapshotIncompatible is logged when the filter is incompatible with sequence filtering.
	LogMsgSnapshotIncompatible = "snapshot incompatible"

	// LogMsgSnapshotSaved is logged when the snapshot is successfully saved.
	LogMsgSnapshotSaved = "snapshot saved"

	// LogMsgSnapshotSaveError is logged when snapshot saving fails.
	LogMsgSnapshotSaveError = "snapshot save error"

	// LogMsgIncrementalQueryError is logged when the incremental query fails.
	LogMsgIncrementalQueryError = "incremental query error: falling back to base handler"

	// LogMsgEventConversionError is logged when event conversion fails.
	LogMsgEventConversionError = "event conversion error: falling back to base handler"

	// LogMsgSnapshotDeserializationError is logged when snapshot deserialization fails.
	LogMsgSnapshotDeserializationError = "snapshot deserialization error: falling back to base handler"

	// LogAttrCommandType identifies the command type in logs.
	LogAttrCommandType = "command_type"

	// LogAttrQueryType identifies the query type in logs.
	LogAttrQueryType = "query_type"

	// LogAttrStatus indicates the command processing status.
	LogAttrStatus = "status"

	// LogAttrDurationMS indicates the processing duration in milliseconds.
	LogAttrDurationMS = "duration_ms"

	// LogAttrBusinessOutcome classifies the business result.
	LogAttrBusinessOutcome = "business_outcome"

	// LogAttrError contains error details.
	LogAttrError = "error"

	// LogAttrSnapshotStatus indicates the snapshot operation status (hit/miss).
	LogAttrSnapshotStatus = "snapshot_status"

	// LogAttrReason indicates the reason for a fallback or failure.
	LogAttrReason = "reason"

	// LogAttrOperation indicates which snapshot operation was being performed.
	LogAttrOperation = "operation"

	// LogAttrFromSequence indicates the starting sequence number for incremental queries.
	LogAttrFromSequence = "from_sequence"

	// LogAttrToSequence indicates the ending sequence number for incremental queries.
	LogAttrToSequence = "to_sequence"

	// LogAttrEventCount indicates the number of events processed.
	LogAttrEventCount = "event_count"

	// LogAttrSequence indicates a sequence number value.
	LogAttrSequence = "sequence"

	// SpanNameCommandHandle is the tracing span name for command handling.
	SpanNameCommandHandle = "commandhandler.handle"

	// SpanNameQueryHandle is the tracing span name for query handling.
	SpanNameQueryHandle = "queryhandler.handle"

	// SnapshotReasonError indicates that the snapshot operation failed with an error.
	SnapshotReasonError = "snapshot_error"

	// SnapshotReasonMiss indicates that no snapshot was found.
	SnapshotReasonMiss = "snapshot_miss"

	// SnapshotReasonFilterIncompatible indicates that the filter is incompatible with sequence filtering.
	SnapshotReasonFilterIncompatible = "filter_incompatible"

	// SnapshotReasonIncrementalQueryError indicates that the incremental query failed.
	SnapshotReasonIncrementalQueryError = "incremental_query_error"

	// SnapshotReasonUnmarshalError indicates that the event unmarshaling failed.
	SnapshotReasonUnmarshalError = "unmarshal_error"

	// SnapshotReasonDeserializeError indicates that the snapshot deserialization failed.
	SnapshotReasonDeserializeError = "deserialize_error"

	// SnapshotReasonHit indicates that the snapshot was successfully used.
	SnapshotReasonHit = "snapshot_hit"
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

// BuildCommandLabels creates standard metric labels for command handler operations.
func BuildCommandLabels(commandType, status string) map[string]string {
	return map[string]string{
		LogAttrCommandType: commandType,
		LogAttrStatus:      status,
	}
}

// BuildQueryLabels creates standard metric labels for query handler operations.
func BuildQueryLabels(queryType, status string) map[string]string {
	return map[string]string{
		LogAttrQueryType: queryType,
		LogAttrStatus:    status,
	}
}

// BuildCommandComponentLabels creates metric labels for command handler component timing.
func BuildCommandComponentLabels(commandType, component, status string) map[string]string {
	return map[string]string{
		LogAttrCommandType: commandType,
		"component":        component,
		LogAttrStatus:      status,
	}
}

// BuildQueryComponentLabels creates metric labels for query handler component timing.
func BuildQueryComponentLabels(queryType, component, status string) map[string]string {
	return map[string]string{
		LogAttrQueryType: queryType,
		"component":      component,
		LogAttrStatus:    status,
	}
}

// BuildRetryLabels creates standard metric labels for retry operations.
func BuildRetryLabels(commandType string, attemptNumber int, errorType string) map[string]string {
	return map[string]string{
		LogAttrCommandType: commandType,
		"attempt_number":   fmt.Sprintf("%d", attemptNumber),
		"error_type":       errorType,
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

	// Record canceled operations separately
	if status == StatusCanceled {
		canceledLabels := BuildCommandLabels(commandType, StatusCanceled)
		if contextualCollector, ok := collector.(ContextualMetricsCollector); ok {
			contextualCollector.IncrementCounterContext(ctx, CommandHandlerCanceledMetric, canceledLabels)
		} else {
			collector.IncrementCounter(CommandHandlerCanceledMetric, canceledLabels)
		}
	}

	// Record timeout operations separately
	if status == StatusTimeout {
		timeoutLabels := BuildCommandLabels(commandType, StatusTimeout)
		if contextualCollector, ok := collector.(ContextualMetricsCollector); ok {
			contextualCollector.IncrementCounterContext(ctx, CommandHandlerTimeoutMetric, timeoutLabels)
		} else {
			collector.IncrementCounter(CommandHandlerTimeoutMetric, timeoutLabels)
		}
	}

	// Record concurrency conflict operations separately
	if status == StatusConcurrencyConflict {
		conflictLabels := BuildCommandLabels(commandType, StatusConcurrencyConflict)
		if contextualCollector, ok := collector.(ContextualMetricsCollector); ok {
			contextualCollector.IncrementCounterContext(ctx, CommandHandlerConcurrencyConflictMetric, conflictLabels)
		} else {
			collector.IncrementCounter(CommandHandlerConcurrencyConflictMetric, conflictLabels)
		}
	}
}

// RecordQueryMetrics is a helper function to record all relevant metrics for a query operation.
// It handles both context-aware and basic metrics collectors automatically.
func RecordQueryMetrics(
	ctx context.Context,
	collector MetricsCollector,
	queryType string,
	status string,
	duration time.Duration,
) {
	if collector == nil {
		return
	}

	labels := BuildQueryLabels(queryType, status)

	// Record duration metric
	if contextualCollector, ok := collector.(ContextualMetricsCollector); ok {
		contextualCollector.RecordDurationContext(ctx, QueryHandlerDurationMetric, duration, labels)
		contextualCollector.IncrementCounterContext(ctx, QueryHandlerCallsMetric, labels)
	} else {
		collector.RecordDuration(QueryHandlerDurationMetric, duration, labels)
		collector.IncrementCounter(QueryHandlerCallsMetric, labels)
	}

	// Record canceled operations separately
	if status == StatusCanceled {
		canceledLabels := BuildQueryLabels(queryType, StatusCanceled)
		if contextualCollector, ok := collector.(ContextualMetricsCollector); ok {
			contextualCollector.IncrementCounterContext(ctx, QueryHandlerCanceledMetric, canceledLabels)
		} else {
			collector.IncrementCounter(QueryHandlerCanceledMetric, canceledLabels)
		}
	}

	// Record timeout operations separately
	if status == StatusTimeout {
		timeoutLabels := BuildQueryLabels(queryType, StatusTimeout)
		if contextualCollector, ok := collector.(ContextualMetricsCollector); ok {
			contextualCollector.IncrementCounterContext(ctx, QueryHandlerTimeoutMetric, timeoutLabels)
		} else {
			collector.IncrementCounter(QueryHandlerTimeoutMetric, timeoutLabels)
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

// StartQuerySpan starts a distributed tracing span for query operations.
// Returns the updated context and span context, or original context and nil if tracing is disabled.
func StartQuerySpan(
	ctx context.Context,
	tracingCollector TracingCollector,
	queryType string,
) (context.Context, SpanContext) {
	if tracingCollector == nil {
		return ctx, nil
	}

	attrs := map[string]string{
		LogAttrQueryType: queryType,
	}

	return tracingCollector.StartSpan(ctx, SpanNameQueryHandle, attrs)
}

// FinishQuerySpan completes a distributed tracing span with the operation outcome.
func FinishQuerySpan(
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

// LogQueryStart logs the beginning of query processing.
func LogQueryStart(
	ctx context.Context,
	logger Logger,
	contextualLogger ContextualLogger,
	queryType string,
) {
	if contextualLogger != nil {
		contextualLogger.InfoContext(ctx, LogMsgQueryStarted, LogAttrQueryType, queryType)
	} else if logger != nil {
		logger.Info(LogMsgQueryStarted, LogAttrQueryType, queryType)
	}
}

// LogQuerySuccess logs successful query completion.
func LogQuerySuccess(
	ctx context.Context,
	logger Logger,
	contextualLogger ContextualLogger,
	queryType string,
	businessOutcome string,
	duration time.Duration,
) {
	args := []any{
		LogAttrQueryType, queryType,
		LogAttrBusinessOutcome, businessOutcome,
		LogAttrDurationMS, ToMilliseconds(duration),
	}

	if contextualLogger != nil {
		contextualLogger.InfoContext(ctx, LogMsgQueryCompleted, args...)
	} else if logger != nil {
		logger.Info(LogMsgQueryCompleted, args...)
	}
}

// LogQueryError logs query processing errors.
func LogQueryError(
	ctx context.Context,
	logger Logger,
	contextualLogger ContextualLogger,
	queryType string,
	err error,
) {
	args := []any{
		LogAttrQueryType, queryType,
		LogAttrError, err.Error(),
	}

	if contextualLogger != nil {
		contextualLogger.ErrorContext(ctx, LogMsgQueryFailed, args...)
	} else if logger != nil {
		logger.Error(LogMsgQueryFailed, args...)
	}
}

// RecordCommandComponentDuration records component-level timing metrics for command handlers.
// It handles both context-aware and basic metrics collectors automatically.
func RecordCommandComponentDuration(
	ctx context.Context,
	collector MetricsCollector,
	commandType string,
	component string,
	status string,
	duration time.Duration,
) {
	if collector == nil {
		return
	}

	labels := BuildCommandComponentLabels(commandType, component, status)

	if contextualCollector, ok := collector.(ContextualMetricsCollector); ok {
		contextualCollector.RecordDurationContext(ctx, CommandHandlerComponentDurationMetric, duration, labels)
	} else {
		collector.RecordDuration(CommandHandlerComponentDurationMetric, duration, labels)
	}
}

// RecordQueryComponentDuration records component-level timing metrics for query handlers.
// It handles both context-aware and basic metrics collectors automatically.
func RecordQueryComponentDuration(
	ctx context.Context,
	collector MetricsCollector,
	queryType string,
	component string,
	status string,
	duration time.Duration,
) {
	if collector == nil {
		return
	}

	labels := BuildQueryComponentLabels(queryType, component, status)

	if contextualCollector, ok := collector.(ContextualMetricsCollector); ok {
		contextualCollector.RecordDurationContext(ctx, QueryHandlerComponentDurationMetric, duration, labels)
	} else {
		collector.RecordDuration(QueryHandlerComponentDurationMetric, duration, labels)
	}
}

// formatDurationMS formats duration in milliseconds for span attributes.
func formatDurationMS(duration time.Duration) string {
	return fmt.Sprintf("%.2f", ToMilliseconds(duration))
}

// IsCancellationError checks if an error is due to context cancellation.
func IsCancellationError(err error) bool {
	return errors.Is(err, context.Canceled)
}

// IsTimeoutError checks if an error is due to context deadline exceeded.
func IsTimeoutError(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}

// IsConcurrencyConflictError checks if an error is due to optimistic concurrency control failure.
func IsConcurrencyConflictError(err error) bool {
	return errors.Is(err, eventstore.ErrConcurrencyConflict)
}
