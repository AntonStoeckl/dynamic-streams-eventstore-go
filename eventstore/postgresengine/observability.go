package postgresengine

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

// ===== INSTRUMENTATION CONTEXT TYPES =====
// Encapsulate all instrumentation context for managing observability across operation phases

// queryInstrumentation encapsulates all instrumentation context for query operations.
type queryInstrumentation struct {
	tracer      *queryTracingObserver
	metrics     *queryMetricsObserver
	methodStart time.Time
}

// appendInstrumentation encapsulates all instrumentation context for append operations.
type appendInstrumentation struct {
	tracer      *appendTracingObserver
	metrics     *appendMetricsObserver
	methodStart time.Time
}

// ===== OBSERVABILITY CONSTANTS =====
// Constants for structured logging, metrics, tracing, and error classification

const (
	// Structured logging messages.
	logMsgBuildSelectQueryFailed   = "failed to build select query"
	logMsgDBQueryFailed            = "database query execution failed"
	logMsgCloseRowsFailed          = "failed to close database rows"
	logMsgScanRowFailed            = "failed to scan database row"
	logMsgBuildStorableEventFailed = "failed to build storable event from database row"
	logMsgBuildInsertQueryFailed   = "failed to build insert query"
	logMsgDBExecFailed             = "database execution failed during event append"
	logMsgRowsAffectedFailed       = "failed to get rows affected count"
	logMsgSingleEventSQLFailed     = "failed to convert single event insert statement to SQL"
	logMsgMultiEventSQLFailed      = "failed to convert multiple events insert statement to SQL"
	logMsgQueryCompleted           = "query completed"
	logMsgEventsAppended           = "events appended"
	logMsgConcurrencyConflict      = "concurrency conflict detected"
	logMsgSQLExecuted              = "executed sql for: "
	logMsgOperation                = "eventstore operation: "

	// Structured logging attribute names.
	logAttrError            = "error"
	logAttrQuery            = "query"
	logAttrEventType        = "event_type"
	logAttrEventCount       = "event_count"
	logAttrDurationMS       = "duration_ms"
	logAttrExpectedEvents   = "expected_events"
	logAttrRowsAffected     = "rows_affected"
	logAttrExpectedSequence = "expected_sequence"
	logActionQuery          = "query"
	logActionAppend         = "append"

	// OpenTelemetry-compatible metrics names.

	// Method-level metrics (complete operation timing).
	metricQueryMethodDuration  = "eventstore_query_method_duration_seconds"
	metricAppendMethodDuration = "eventstore_append_method_duration_seconds"

	// SQL-level metrics (database execution only).
	metricQuerySQLDuration  = "eventstore_query_sql_duration_seconds"
	metricAppendSQLDuration = "eventstore_append_sql_duration_seconds"

	// Component-level metrics (Phase 2).
	metricComponentDuration = "eventstore_component_duration_seconds"

	// Other existing metrics.
	metricEventsQueried        = "eventstore_events_queried_total"
	metricEventsAppended       = "eventstore_events_appended_total"
	metricConcurrencyConflicts = "eventstore_concurrency_conflicts_total"
	metricDatabaseErrors       = "eventstore_database_errors_total"

	// Method-level metrics (separate from the SQL operation metrics above).
	metricQueryMethodCalls  = "eventstore_query_method_calls_total"
	metricAppendMethodCalls = "eventstore_append_method_calls_total"
	metricQueryCanceled     = "eventstore_query_canceled_total"
	metricAppendCanceled    = "eventstore_append_canceled_total"
	metricQueryTimeout      = "eventstore_query_timeout_total"
	metricAppendTimeout     = "eventstore_append_timeout_total"

	// Shared operation constants for metrics and tracing.
	operationQuery  = "query"
	operationAppend = "append"

	// Shared status constants for metrics and tracing.
	statusSuccess  = "success"
	statusError    = "error"
	statusCanceled = "canceled"
	statusTimeout  = "timeout"

	// Component names for hierarchical timing (Phase 2).
	componentQueryBuild       = "query_build"
	componentSQLExecution     = "sql_execution"
	componentResultProcessing = "result_processing"

	// Error type classification for metrics and tracing.
	errorTypeBuildQuery          = "build_query"
	errorTypeDatabaseQuery       = "database_query"
	errorTypeScanResults         = "scan_results"
	errorTypeDatabaseExec        = "database_exec"
	errorTypeConcurrencyConflict = "concurrency_conflict"
	errorTypeRowScan             = "row_scan"
	errorTypeBuildStorableEvent  = "build_storable_event"
	errorTypeRowsAffected        = "rows_affected"
	errorTypeBuildSingleEventSQL = "build_single_event_sql"
	errorTypeBuildMultiEventSQL  = "build_multi_event_sql"
	errorTypeCancelled           = "cancelled"
	errorTypeTimeout             = "timeout"

	// Distributed tracing span names.
	spanNameQuery  = "eventstore.query"
	spanNameAppend = "eventstore.append"

	// Distributed tracing span attribute names.
	spanAttrOperation    = "operation"
	spanAttrEventCount   = "event_count"
	spanAttrMaxSequence  = "max_sequence"
	spanAttrDurationMS   = "duration_ms"
	spanAttrRowsAffected = "rows_affected"
	spanAttrErrorType    = "error_type"
	spanAttrEventType    = "event_type"
	spanAttrExpectedSeq  = "expected_seq"
)

// ===== BASIC LOGGING UTILITIES =====
// Low-level logging functions that don't require context awareness

// logQueryWithDuration logs SQL queries with execution time at debug level if the logger is configured.
func (es *EventStore) logQueryWithDuration(
	sqlQuery string,
	action string,
	duration time.Duration,
) {
	if es.logger != nil {
		es.logger.Debug(logMsgSQLExecuted+action, logAttrDurationMS, es.toMilliseconds(duration), logAttrQuery, sqlQuery)
	}
}

// logOperation logs operational information at info level if the logger is configured.
func (es *EventStore) logOperation(action string, args ...any) {
	if es.logger != nil {
		es.logger.Info(logMsgOperation+action, args...)
	}
}

// logError logs error information at the error level if the logger is configured.
func (es *EventStore) logError(
	message string,
	err error,
	args ...any,
) {
	if es.logger != nil {
		allArgs := []any{logAttrError, err.Error()}
		allArgs = append(allArgs, args...)
		es.logger.Error(message, allArgs...)
	}
}

// toMilliseconds converts a time.Duration to float64 milliseconds with 3 decimal places.
func (es *EventStore) toMilliseconds(d time.Duration) float64 {
	return math.Round(float64(d.Nanoseconds())/1e6*1000) / 1000
}

// ===== CONTEXTUAL LOGGING PATTERN =====
// Context-aware logging with automatic trace correlation when available

// logQueryWithDurationContext logs SQL queries with execution time and context correlation.
func (es *EventStore) logQueryWithDurationContext(
	ctx context.Context,
	sqlQuery string,
	action string,
	duration time.Duration,
) {
	if es.contextualLogger != nil {
		es.contextualLogger.DebugContext(ctx, logMsgSQLExecuted+action, logAttrDurationMS, es.toMilliseconds(duration), logAttrQuery, sqlQuery)
	}
}

// logOperationContext logs operational information with context correlation.
func (es *EventStore) logOperationContext(ctx context.Context, action string, args ...any) {
	if es.contextualLogger != nil {
		es.contextualLogger.InfoContext(ctx, logMsgOperation+action, args...)
	}
}

// logErrorContext logs error information with context correlation.
func (es *EventStore) logErrorContext(
	ctx context.Context,
	message string,
	err error,
	args ...any,
) {
	if es.contextualLogger != nil {
		allArgs := []any{logAttrError, err.Error()}
		allArgs = append(allArgs, args...)
		es.contextualLogger.ErrorContext(ctx, message, allArgs...)
	}
}

// ===== BASIC METRICS UTILITIES =====
// Low-level metrics recording functions for different metric types

// recordErrorMetrics records error metricsCollector if the metricsCollector collector is configured.
func (es *EventStore) recordErrorMetrics(operation, errorType string) {
	if es.metricsCollector != nil {
		labels := map[string]string{
			spanAttrOperation: operation,
			"status":          statusError,
			spanAttrErrorType: errorType,
		}
		es.metricsCollector.IncrementCounter(metricDatabaseErrors, labels)
	}
}

// recordErrorMetricsContext records error metricsCollector with context if the collector supports it.
func (es *EventStore) recordErrorMetricsContext(ctx context.Context, operation, errorType string) {
	if es.metricsCollector != nil {
		labels := map[string]string{
			spanAttrOperation: operation,
			"status":          statusError,
			spanAttrErrorType: errorType,
		}

		// Use context-aware method if available
		if contextualCollector, ok := es.metricsCollector.(eventstore.ContextualMetricsCollector); ok {
			contextualCollector.IncrementCounterContext(ctx, metricDatabaseErrors, labels)
		} else {
			es.metricsCollector.IncrementCounter(metricDatabaseErrors, labels)
		}
	}
}

// recordDurationMetricsContext records duration metricsCollector with context if the collector supports it.
func (es *EventStore) recordDurationMetricsContext(
	ctx context.Context,
	metricName string,
	duration time.Duration,
	operation, status string,
) {
	if es.metricsCollector != nil {
		labels := map[string]string{
			spanAttrOperation: operation,
			"status":          status,
		}

		// Use context-aware method if available
		if contextualCollector, ok := es.metricsCollector.(eventstore.ContextualMetricsCollector); ok {
			contextualCollector.RecordDurationContext(ctx, metricName, duration, labels)
		} else {
			es.metricsCollector.RecordDuration(metricName, duration, labels)
		}
	}
}

// recordValueMetricsContext records value metricsCollector with context if the collector supports it.
func (es *EventStore) recordValueMetricsContext(
	ctx context.Context,
	metricName string,
	value float64,
	operation,
	status string,
) {
	if es.metricsCollector != nil {
		labels := map[string]string{
			spanAttrOperation: operation,
			"status":          status,
		}

		// Use context-aware method if available
		if contextualCollector, ok := es.metricsCollector.(eventstore.ContextualMetricsCollector); ok {
			contextualCollector.RecordValueContext(ctx, metricName, value, labels)
		} else {
			es.metricsCollector.RecordValue(metricName, value, labels)
		}
	}
}

// recordConcurrencyConflictMetrics records concurrency conflict metricsCollector if the metricsCollector collector is configured.
func (es *EventStore) recordConcurrencyConflictMetrics(operation string) {
	if es.metricsCollector != nil {
		labels := map[string]string{
			spanAttrOperation: operation,
			"conflict_type":   "concurrency",
		}
		es.metricsCollector.IncrementCounter(metricConcurrencyConflicts, labels)
	}
}

// recordCancelledMetricsContext records canceled operation metrics with context if the collector supports it.
func (es *EventStore) recordCancelledMetricsContext(ctx context.Context, operation string) {
	if es.metricsCollector != nil {
		labels := map[string]string{
			spanAttrOperation: operation,
			"status":          statusCanceled,
		}

		var metricName string
		switch operation {
		case operationQuery:
			metricName = metricQueryCanceled
		case operationAppend:
			metricName = metricAppendCanceled
		default:
			return
		}

		if contextualCollector, ok := es.metricsCollector.(eventstore.ContextualMetricsCollector); ok {
			contextualCollector.IncrementCounterContext(ctx, metricName, labels)
		} else {
			es.metricsCollector.IncrementCounter(metricName, labels)
		}
	}
}

// recordTimeoutMetricsContext records timeout operation metrics with context if the collector supports it.
func (es *EventStore) recordTimeoutMetricsContext(ctx context.Context, operation string) {
	if es.metricsCollector != nil {
		labels := map[string]string{
			spanAttrOperation: operation,
			"status":          statusTimeout,
		}

		var metricName string
		switch operation {
		case operationQuery:
			metricName = metricQueryTimeout
		case operationAppend:
			metricName = metricAppendTimeout
		default:
			return
		}

		if contextualCollector, ok := es.metricsCollector.(eventstore.ContextualMetricsCollector); ok {
			contextualCollector.IncrementCounterContext(ctx, metricName, labels)
		} else {
			es.metricsCollector.IncrementCounter(metricName, labels)
		}
	}
}

// ===== BASIC TRACING UTILITIES =====
// Low-level tracing span management functions

// startTraceSpan starts a tracing span if the tracing collector is configured.
func (es *EventStore) startTraceSpan(
	ctx context.Context,
	operation string,
	attrs map[string]string,
) (context.Context, SpanContext) {
	if es.tracingCollector != nil {
		return es.tracingCollector.StartSpan(ctx, operation, attrs)
	}

	return ctx, nil
}

// finishTraceSpan finishes a tracing span if the tracing collector is configured.
func (es *EventStore) finishTraceSpan(
	spanCtx SpanContext,
	status string,
	attrs map[string]string,
) {
	if es.tracingCollector != nil && spanCtx != nil {
		es.tracingCollector.FinishSpan(spanCtx, status, attrs)
	}
}

// startQuerySpan starts a tracing span for query operations.
func (es *EventStore) startQuerySpan(ctx context.Context) (context.Context, SpanContext) {
	spanAttrs := map[string]string{
		spanAttrOperation: operationQuery,
	}

	return es.startTraceSpan(ctx, spanNameQuery, spanAttrs)
}

// finishQuerySpanSuccess finishes a successful query span with results.
func (es *EventStore) finishQuerySpanSuccess(
	span SpanContext,
	eventStream eventstore.StorableEvents,
	maxSequenceNumber eventstore.MaxSequenceNumberUint,
	duration time.Duration,
) {
	if span != nil {
		span.SetStatus(statusSuccess)
		if eventStream != nil {
			span.AddAttribute(spanAttrEventCount, fmt.Sprintf("%d", len(eventStream)))
		}
		span.AddAttribute(spanAttrMaxSequence, fmt.Sprintf("%d", maxSequenceNumber))
		span.AddAttribute(spanAttrDurationMS, fmt.Sprintf("%.2f", float64(duration.Nanoseconds())/1e6))
	}

	attrs := map[string]string{
		spanAttrMaxSequence: fmt.Sprintf("%d", maxSequenceNumber),
	}

	if eventStream != nil {
		attrs[spanAttrEventCount] = fmt.Sprintf("%d", len(eventStream))
	}

	es.finishTraceSpan(span, statusSuccess, attrs)
}

// finishQuerySpanError finishes a query span with error details.
func (es *EventStore) finishQuerySpanError(
	span SpanContext,
	errorType string,
	duration time.Duration,
) {
	if span != nil {
		span.SetStatus(statusError)
		span.AddAttribute(spanAttrErrorType, errorType)

		if duration > 0 {
			span.AddAttribute(spanAttrDurationMS, fmt.Sprintf("%.2f", float64(duration.Nanoseconds())/1e6))
		}
	}

	es.finishTraceSpan(span, statusError, map[string]string{spanAttrErrorType: errorType})
}

// startAppendSpan starts a tracing span for append operations.
func (es *EventStore) startAppendSpan(
	ctx context.Context,
	allEvents eventstore.StorableEvents,
	expectedMaxSequenceNumber eventstore.MaxSequenceNumberUint,
) (context.Context, SpanContext) {
	spanAttrs := map[string]string{
		spanAttrOperation:   operationAppend,
		spanAttrEventCount:  fmt.Sprintf("%d", len(allEvents)),
		spanAttrExpectedSeq: fmt.Sprintf("%d", expectedMaxSequenceNumber),
	}

	if len(allEvents) > 0 {
		spanAttrs[spanAttrEventType] = allEvents[0].EventType
	}

	return es.startTraceSpan(ctx, spanNameAppend, spanAttrs)
}

// finishAppendSpanSuccess finishes a successful append span with results.
func (es *EventStore) finishAppendSpanSuccess(
	span SpanContext,
	rowsAffected int64,
	duration time.Duration,
) {
	if span != nil {
		span.SetStatus(statusSuccess)
		span.AddAttribute(spanAttrRowsAffected, fmt.Sprintf("%d", rowsAffected))
		span.AddAttribute(spanAttrDurationMS, fmt.Sprintf("%.2f", float64(duration.Nanoseconds())/1e6))
	}

	es.finishTraceSpan(span, statusSuccess, map[string]string{
		spanAttrRowsAffected: fmt.Sprintf("%d", rowsAffected),
	})
}

// finishAppendSpanError finishes an append span with error details.
func (es *EventStore) finishAppendSpanError(
	span SpanContext,
	errorType string,
	additionalAttrs map[string]string,
) {
	if span != nil {
		span.SetStatus(statusError)
		span.AddAttribute(spanAttrErrorType, errorType)
		for key, value := range additionalAttrs {
			span.AddAttribute(key, value)
		}
	}

	attrs := map[string]string{spanAttrErrorType: errorType}
	for key, value := range additionalAttrs {
		attrs[key] = value
	}

	es.finishTraceSpan(span, statusError, attrs)
}

// ===== TRACING OBSERVER PATTERN =====
// These observers simplify tracing span management by encapsulating lifecycle complexity

// queryTracingObserver encapsulates tracing span lifecycle management for query operations.
type queryTracingObserver struct {
	es   *EventStore
	span SpanContext
}

// appendTracingObserver encapsulates tracing span lifecycle management for append operations.
type appendTracingObserver struct {
	es   *EventStore
	span SpanContext
}

// startQueryTracing creates a new tracing observer for query operations.
func (es *EventStore) startQueryTracing(ctx context.Context) (*queryTracingObserver, context.Context) {
	newCtx, span := es.startQuerySpan(ctx)

	return &queryTracingObserver{
		es:   es,
		span: span,
	}, newCtx
}

// startAppendTracing creates a new tracing observer for append operations.
func (es *EventStore) startAppendTracing(
	ctx context.Context,
	events eventstore.StorableEvents,
	expectedMaxSequenceNumber eventstore.MaxSequenceNumberUint,
) (*appendTracingObserver, context.Context) {

	newCtx, span := es.startAppendSpan(ctx, events, expectedMaxSequenceNumber)

	return &appendTracingObserver{
		es:   es,
		span: span,
	}, newCtx
}

// finishError completes the query tracing span with error details.
func (qto *queryTracingObserver) finishError(errorType string, duration time.Duration) {
	if qto.span == nil {
		return
	}

	qto.es.finishQuerySpanError(qto.span, errorType, duration)
}

// finishCancelled completes the query tracing span with cancellation details.
func (qto *queryTracingObserver) finishCancelled(duration time.Duration) {
	if qto.span == nil {
		return
	}

	qto.es.finishQuerySpanError(qto.span, errorTypeCancelled, duration)
}

// finishTimeout completes the query tracing span with timeout details.
func (qto *queryTracingObserver) finishTimeout(duration time.Duration) {
	if qto.span == nil {
		return
	}

	qto.es.finishQuerySpanError(qto.span, errorTypeTimeout, duration)
}

// finishSuccess completes the query tracing span for successful operations.
func (qto *queryTracingObserver) finishSuccess(
	eventStream eventstore.StorableEvents,
	maxSequenceNumber eventstore.MaxSequenceNumberUint,
	duration time.Duration,
) {
	if qto.span == nil {
		return
	}

	qto.es.finishQuerySpanSuccess(qto.span, eventStream, maxSequenceNumber, duration)
}

// finishError completes the append operation's tracing span with error details.
func (ato *appendTracingObserver) finishError(errorType string, duration time.Duration) {
	if ato.span == nil {
		return
	}

	// For append operations, we may need additional attributes
	var attrs map[string]string
	if duration > 0 {
		attrs = map[string]string{
			spanAttrDurationMS: ato.formatDuration(duration),
		}
	}

	ato.es.finishAppendSpanError(ato.span, errorType, attrs)
}

// finishCancelled completes the append operation's tracing span with cancellation details.
func (ato *appendTracingObserver) finishCancelled(duration time.Duration) {
	if ato.span == nil {
		return
	}

	var attrs map[string]string
	if duration > 0 {
		attrs = map[string]string{
			spanAttrDurationMS: ato.formatDuration(duration),
		}
	}

	ato.es.finishAppendSpanError(ato.span, errorTypeCancelled, attrs)
}

// finishTimeout completes the append operation's tracing span with timeout details.
func (ato *appendTracingObserver) finishTimeout(duration time.Duration) {
	if ato.span == nil {
		return
	}

	var attrs map[string]string
	if duration > 0 {
		attrs = map[string]string{
			spanAttrDurationMS: ato.formatDuration(duration),
		}
	}

	ato.es.finishAppendSpanError(ato.span, errorTypeTimeout, attrs)
}

// finishErrorWithAttrs completes the append operation's tracing span with error details and additional attributes.
func (ato *appendTracingObserver) finishErrorWithAttrs(errorType string, attrs map[string]string) {
	if ato.span == nil {
		return
	}

	ato.es.finishAppendSpanError(ato.span, errorType, attrs)
}

// finishSuccess completes the append operation's tracing span for successful operations.
func (ato *appendTracingObserver) finishSuccess(rowsAffected int64, duration time.Duration) {
	if ato.span == nil {
		return
	}

	ato.es.finishAppendSpanSuccess(ato.span, rowsAffected, duration)
}

// formatDuration formats duration for span attributes using the EventStore's helper.
func (ato *appendTracingObserver) formatDuration(duration time.Duration) string {
	return fmt.Sprintf("%.2f", ato.es.toMilliseconds(duration))
}

// ===== METRICS OBSERVER PATTERN =====
// These observers simplify the metrics collection by encapsulating recording complexity

// queryMetricsObserver encapsulates the metrics collection for query operations.
type queryMetricsObserver struct {
	es  *EventStore
	ctx context.Context
}

// appendMetricsObserver encapsulates the metrics collection for append operations.
type appendMetricsObserver struct {
	es  *EventStore
	ctx context.Context
}

// startQueryMetrics creates a new metrics observer for query operations.
func (es *EventStore) startQueryMetrics(ctx context.Context) *queryMetricsObserver {
	return &queryMetricsObserver{
		es:  es,
		ctx: ctx,
	}
}

// startAppendMetrics creates a new metrics observer for append operations.
func (es *EventStore) startAppendMetrics(ctx context.Context) *appendMetricsObserver {
	return &appendMetricsObserver{
		es:  es,
		ctx: ctx,
	}
}

// recordSuccess records all metrics for a successful query operation.
func (qmo *queryMetricsObserver) recordSuccess(eventStream eventstore.StorableEvents, duration time.Duration) {
	qmo.es.recordDurationMetricsContext(qmo.ctx, metricQuerySQLDuration, duration, operationQuery, statusSuccess)

	eventCount := float64(0)
	if eventStream != nil {
		eventCount = float64(len(eventStream))
	}

	qmo.es.recordValueMetricsContext(qmo.ctx, metricEventsQueried, eventCount, operationQuery, statusSuccess)
}

// recordMethodSuccess records method-level timing for successful query operations.
func (qmo *queryMetricsObserver) recordMethodSuccess(eventStream eventstore.StorableEvents, duration time.Duration) {
	qmo.es.recordDurationMetricsContext(qmo.ctx, metricQueryMethodDuration, duration, operationQuery, statusSuccess)

	eventCount := float64(0)
	if eventStream != nil {
		eventCount = float64(len(eventStream))
	}

	qmo.es.recordValueMetricsContext(qmo.ctx, metricEventsQueried, eventCount, operationQuery, statusSuccess)
}

// recordComponentSuccess records component-level timing (Phase 2).
func (qmo *queryMetricsObserver) recordComponentSuccess(component string, duration time.Duration) {
	if qmo.es.metricsCollector == nil {
		return
	}

	labels := map[string]string{
		spanAttrOperation: operationQuery,
		"component":       component,
		"status":          statusSuccess,
	}

	if contextualCollector, ok := qmo.es.metricsCollector.(eventstore.ContextualMetricsCollector); ok {
		contextualCollector.RecordDurationContext(qmo.ctx, metricComponentDuration, duration, labels)
	} else {
		qmo.es.metricsCollector.RecordDuration(metricComponentDuration, duration, labels)
	}
}

// recordError records all metrics for a failed query operation.
func (qmo *queryMetricsObserver) recordError(errorType string, duration time.Duration) {
	qmo.es.recordDurationMetricsContext(qmo.ctx, metricQuerySQLDuration, duration, operationQuery, statusError)
	qmo.es.recordDurationMetricsContext(qmo.ctx, metricQueryMethodDuration, duration, operationQuery, statusError)
	qmo.es.recordErrorMetricsContext(qmo.ctx, operationQuery, errorType)
}

// recordCanceled records all metrics for a canceled query operation.
func (qmo *queryMetricsObserver) recordCanceled(duration time.Duration) {
	qmo.es.recordDurationMetricsContext(qmo.ctx, metricQuerySQLDuration, duration, operationQuery, statusCanceled)
	qmo.es.recordDurationMetricsContext(qmo.ctx, metricQueryMethodDuration, duration, operationQuery, statusCanceled)
	qmo.es.recordCancelledMetricsContext(qmo.ctx, operationQuery)
}

// recordTimeout records all metrics for a timeout query operation.
func (qmo *queryMetricsObserver) recordTimeout(duration time.Duration) {
	qmo.es.recordDurationMetricsContext(qmo.ctx, metricQuerySQLDuration, duration, operationQuery, statusTimeout)
	qmo.es.recordDurationMetricsContext(qmo.ctx, metricQueryMethodDuration, duration, operationQuery, statusTimeout)
	qmo.es.recordTimeoutMetricsContext(qmo.ctx, operationQuery)
}

// recordSuccess records all metrics for a successful append operation.
func (amo *appendMetricsObserver) recordSuccess(eventCount int, duration time.Duration) {
	amo.es.recordDurationMetricsContext(amo.ctx, metricAppendSQLDuration, duration, operationAppend, statusSuccess)
	amo.es.recordValueMetricsContext(amo.ctx, metricEventsAppended, float64(eventCount), operationAppend, statusSuccess)
}

// recordMethodSuccess records method-level timing for successful append operations.
func (amo *appendMetricsObserver) recordMethodSuccess(eventCount int, duration time.Duration) {
	amo.es.recordDurationMetricsContext(amo.ctx, metricAppendMethodDuration, duration, operationAppend, statusSuccess)
	amo.es.recordValueMetricsContext(amo.ctx, metricEventsAppended, float64(eventCount), operationAppend, statusSuccess)
}

// recordComponentSuccess records component-level timing (Phase 2).
func (amo *appendMetricsObserver) recordComponentSuccess(component string, duration time.Duration) {
	if amo.es.metricsCollector == nil {
		return
	}

	labels := map[string]string{
		spanAttrOperation: operationAppend,
		"component":       component,
		"status":          statusSuccess,
	}

	if contextualCollector, ok := amo.es.metricsCollector.(eventstore.ContextualMetricsCollector); ok {
		contextualCollector.RecordDurationContext(amo.ctx, metricComponentDuration, duration, labels)
	} else {
		amo.es.metricsCollector.RecordDuration(metricComponentDuration, duration, labels)
	}
}

// recordError records all metrics for a failed append operation.
func (amo *appendMetricsObserver) recordError(errorType string, duration time.Duration) {
	amo.es.recordDurationMetricsContext(amo.ctx, metricAppendSQLDuration, duration, operationAppend, statusError)
	amo.es.recordDurationMetricsContext(amo.ctx, metricAppendMethodDuration, duration, operationAppend, statusError)
	amo.es.recordErrorMetricsContext(amo.ctx, operationAppend, errorType)
}

// recordCanceled records all metrics for a canceled append operation.
func (amo *appendMetricsObserver) recordCanceled(duration time.Duration) {
	amo.es.recordDurationMetricsContext(amo.ctx, metricAppendSQLDuration, duration, operationAppend, statusCanceled)
	amo.es.recordDurationMetricsContext(amo.ctx, metricAppendMethodDuration, duration, operationAppend, statusCanceled)
	amo.es.recordCancelledMetricsContext(amo.ctx, operationAppend)
}

// recordTimeout records all metrics for a timeout append operation.
func (amo *appendMetricsObserver) recordTimeout(duration time.Duration) {
	amo.es.recordDurationMetricsContext(amo.ctx, metricAppendSQLDuration, duration, operationAppend, statusTimeout)
	amo.es.recordDurationMetricsContext(amo.ctx, metricAppendMethodDuration, duration, operationAppend, statusTimeout)
	amo.es.recordTimeoutMetricsContext(amo.ctx, operationAppend)
}

// recordConcurrencyConflict records metrics for concurrency conflicts during append operations.
func (amo *appendMetricsObserver) recordConcurrencyConflict() {
	amo.es.recordConcurrencyConflictMetrics(operationAppend)
}

// ===== INSTRUMENTATION SETUP & COMPLETION =====
// High-level instrumentation functions that set up and complete observability context

// setupQueryInstrumentation initializes timing, tracing, and metrics for query operations.
func (es *EventStore) setupQueryInstrumentation(ctx context.Context) (queryInstrumentation, context.Context) {
	// Method-level timing
	methodStart := time.Now()

	tracer, ctx := es.startQueryTracing(ctx)
	metrics := es.startQueryMetrics(ctx)

	// Record method call counter (separate from SQL operation metrics)
	if es.metricsCollector != nil {
		labels := map[string]string{
			spanAttrOperation: operationQuery,
		}
		if contextualCollector, ok := es.metricsCollector.(eventstore.ContextualMetricsCollector); ok {
			contextualCollector.IncrementCounterContext(ctx, metricQueryMethodCalls, labels)
		} else {
			es.metricsCollector.IncrementCounter(metricQueryMethodCalls, labels)
		}
	}

	return queryInstrumentation{
		tracer:      tracer,
		metrics:     metrics,
		methodStart: methodStart,
	}, ctx
}

// setupAppendInstrumentation initializes timing, tracing, and metrics for append operations.
func (es *EventStore) setupAppendInstrumentation(
	ctx context.Context,
	storableEvents eventstore.StorableEvents,
	expectedMaxSequenceNumber eventstore.MaxSequenceNumberUint,
) (appendInstrumentation, context.Context) {
	// Method-level timing
	methodStart := time.Now()

	tracer, ctx := es.startAppendTracing(ctx, storableEvents, expectedMaxSequenceNumber)
	metrics := es.startAppendMetrics(ctx)

	// Record method call counter (separate from SQL operation metrics)
	if es.metricsCollector != nil {
		labels := map[string]string{
			spanAttrOperation: operationAppend,
		}
		if contextualCollector, ok := es.metricsCollector.(eventstore.ContextualMetricsCollector); ok {
			contextualCollector.IncrementCounterContext(ctx, metricAppendMethodCalls, labels)
		} else {
			es.metricsCollector.IncrementCounter(metricAppendMethodCalls, labels)
		}
	}

	return appendInstrumentation{
		tracer:      tracer,
		metrics:     metrics,
		methodStart: methodStart,
	}, ctx
}

// completeQuerySuccess handles final logging, metrics, and tracing for successful query operations.
func (es *EventStore) completeQuerySuccess(
	ctx context.Context,
	eventStream eventstore.StorableEvents,
	maxSequenceNumber eventstore.MaxSequenceNumberUint,
	sqlDuration time.Duration,
	inst queryInstrumentation,
) {
	// Method completion timing
	methodDuration := time.Since(inst.methodStart)

	es.logOperation(logMsgQueryCompleted, logAttrEventCount, len(eventStream), logAttrDurationMS, es.toMilliseconds(methodDuration))
	es.logOperationContext(ctx, logMsgQueryCompleted, logAttrEventCount, len(eventStream), logAttrDurationMS, es.toMilliseconds(methodDuration))

	// Record both SQL-level (existing) and method-level (new) timing
	inst.metrics.recordSuccess(eventStream, sqlDuration)
	inst.metrics.recordMethodSuccess(eventStream, methodDuration)

	inst.tracer.finishSuccess(eventStream, maxSequenceNumber, methodDuration)
}

// completeAppendSuccess handles final logging, metrics, and tracing for successful append operations.
func (es *EventStore) completeAppendSuccess(
	ctx context.Context,
	eventCount int,
	rowsAffected int64,
	sqlDuration time.Duration,
	inst appendInstrumentation,
) {
	// Method completion timing
	methodDuration := time.Since(inst.methodStart)

	es.logOperation(logMsgEventsAppended, logAttrEventCount, eventCount, logAttrDurationMS, es.toMilliseconds(methodDuration))
	es.logOperationContext(ctx, logMsgEventsAppended, logAttrEventCount, eventCount, logAttrDurationMS, es.toMilliseconds(methodDuration))

	// Record both SQL-level (existing) and method-level (new) timing
	inst.metrics.recordSuccess(eventCount, sqlDuration)
	inst.metrics.recordMethodSuccess(eventCount, methodDuration)

	inst.tracer.finishSuccess(rowsAffected, methodDuration)
}
