package postgresengine

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

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

// === Tracing Observer Pattern ===
// These observers simplify tracing span management by encapsulating lifecycle complexity.

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

// === Metrics Observer Pattern ===
// These observers simplify the metrics collection by encapsulating recording complexity.

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
	qmo.es.recordDurationMetricsContext(qmo.ctx, metricQueryDuration, duration, operationQuery, statusSuccess)

	eventCount := float64(0)
	if eventStream != nil {
		eventCount = float64(len(eventStream))
	}

	qmo.es.recordValueMetricsContext(qmo.ctx, metricEventsQueried, eventCount, operationQuery, statusSuccess)
}

// recordError records all metrics for a failed query operation.
func (qmo *queryMetricsObserver) recordError(errorType string, duration time.Duration) {
	qmo.es.recordDurationMetricsContext(qmo.ctx, metricQueryDuration, duration, operationQuery, statusError)
	qmo.es.recordErrorMetricsContext(qmo.ctx, operationQuery, errorType)
}

// recordSuccess records all metrics for a successful append operation.
func (amo *appendMetricsObserver) recordSuccess(eventCount int, duration time.Duration) {
	amo.es.recordDurationMetricsContext(amo.ctx, metricAppendDuration, duration, operationAppend, statusSuccess)
	amo.es.recordValueMetricsContext(amo.ctx, metricEventsAppended, float64(eventCount), operationAppend, statusSuccess)
}

// recordError records all metrics for a failed append operation.
func (amo *appendMetricsObserver) recordError(errorType string, duration time.Duration) {
	amo.es.recordDurationMetricsContext(amo.ctx, metricAppendDuration, duration, operationAppend, statusError)
	amo.es.recordErrorMetricsContext(amo.ctx, operationAppend, errorType)
}

// recordConcurrencyConflict records metrics for concurrency conflicts during append operations.
func (amo *appendMetricsObserver) recordConcurrencyConflict() {
	amo.es.recordConcurrencyConflictMetrics(operationAppend)
}

// === Contextual Logging Pattern ===
// These methods provide context-aware logging with automatic trace correlation when available.

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
