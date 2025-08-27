package booksincirculation

import (
	"context"
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
)

// QueryHandler orchestrates the complete query processing workflow.
// It handles infrastructure concerns like event store interactions, observability instrumentation,
// and delegates projection logic to the pure core functions.
type QueryHandler struct {
	eventStore       shell.QueriesEvents
	metricsCollector shell.MetricsCollector
	tracingCollector shell.TracingCollector
	contextualLogger shell.ContextualLogger
	logger           shell.Logger
}

// NewQueryHandler creates a new QueryHandler with the provided EventStore dependency and options.
func NewQueryHandler(eventStore shell.QueriesEvents, opts ...Option) (QueryHandler, error) {
	h := QueryHandler{
		eventStore: eventStore,
	}

	for _, opt := range opts {
		if err := opt(&h); err != nil {
			return QueryHandler{}, err
		}
	}

	return h, nil
}

// Handle executes the complete query processing workflow: Query -> Project.
// It queries the current event history, delegates projection logic to the core function,
// and instruments the operation with comprehensive observability.
func (h QueryHandler) Handle(ctx context.Context, query Query) (BooksInCirculation, error) {
	// Start query handler instrumentation
	queryStart := time.Now()
	ctx, span := shell.StartQuerySpan(ctx, h.tracingCollector, queryType)
	shell.LogQueryStart(ctx, h.logger, h.contextualLogger, queryType)

	filter := BuildEventFilter()

	// Query phase
	storableEvents, maxSeq, err := h.executeQuery(ctx, filter)
	if err != nil {
		h.recordQueryError(ctx, err, time.Since(queryStart), span)
		return BooksInCirculation{}, err
	}

	// Unmarshal phase
	history, err := h.executeUnmarshal(ctx, storableEvents)
	if err != nil {
		h.recordQueryError(ctx, err, time.Since(queryStart), span)
		return BooksInCirculation{}, err
	}

	// Projection phase
	result := h.executeProjection(ctx, history, query, maxSeq)

	h.recordQuerySuccess(ctx, time.Since(queryStart), span)

	return result, nil
}

/*** Query Handler Options and helper methods for observability ***/

// Option defines a functional option for configuring QueryHandler.
type Option func(*QueryHandler) error

// WithMetrics sets the metrics collector for the QueryHandler.
func WithMetrics(collector shell.MetricsCollector) Option {
	return func(h *QueryHandler) error {
		h.metricsCollector = collector
		return nil
	}
}

// WithTracing sets the tracing collector for the QueryHandler.
func WithTracing(collector shell.TracingCollector) Option {
	return func(h *QueryHandler) error {
		h.tracingCollector = collector
		return nil
	}
}

// WithContextualLogging sets the contextual logger for the QueryHandler.
func WithContextualLogging(logger shell.ContextualLogger) Option {
	return func(h *QueryHandler) error {
		h.contextualLogger = logger
		return nil
	}
}

// WithLogging sets the basic logger for the QueryHandler.
func WithLogging(logger shell.Logger) Option {
	return func(h *QueryHandler) error {
		h.logger = logger
		return nil
	}
}

// recordQuerySuccess records successful query execution with observability.
func (h QueryHandler) recordQuerySuccess(ctx context.Context, duration time.Duration, span shell.SpanContext) {
	shell.RecordQueryMetrics(ctx, h.metricsCollector, queryType, shell.StatusSuccess, duration, "")
	shell.FinishQuerySpan(h.tracingCollector, span, shell.StatusSuccess, duration, nil)
	shell.LogQuerySuccess(ctx, h.logger, h.contextualLogger, queryType, shell.StatusSuccess, duration)
}

// recordQueryError records failed query execution with observability.
func (h QueryHandler) recordQueryError(ctx context.Context, err error, duration time.Duration, span shell.SpanContext) {
	if shell.IsCancellationError(err) {
		h.recordQueryCancelled(ctx, err, duration, span)
		return
	}

	if shell.IsTimeoutError(err) {
		h.recordQueryTimeout(ctx, err, duration, span)
		return
	}

	shell.RecordQueryMetrics(ctx, h.metricsCollector, queryType, shell.StatusError, duration, "")
	shell.FinishQuerySpan(h.tracingCollector, span, shell.StatusError, duration, err)
	shell.LogQueryError(ctx, h.logger, h.contextualLogger, queryType, err)
}

// recordQueryCancelled records canceled query execution with observability.
func (h QueryHandler) recordQueryCancelled(ctx context.Context, err error, duration time.Duration, span shell.SpanContext) {
	shell.RecordQueryMetrics(ctx, h.metricsCollector, queryType, shell.StatusCanceled, duration, "")
	shell.FinishQuerySpan(h.tracingCollector, span, shell.StatusCanceled, duration, err)
	shell.LogQueryError(ctx, h.logger, h.contextualLogger, queryType, err)
}

// recordQueryTimeout records timeout query execution with observability.
func (h QueryHandler) recordQueryTimeout(ctx context.Context, err error, duration time.Duration, span shell.SpanContext) {
	shell.RecordQueryMetrics(ctx, h.metricsCollector, queryType, shell.StatusTimeout, duration, "")
	shell.FinishQuerySpan(h.tracingCollector, span, shell.StatusTimeout, duration, err)
	shell.LogQueryError(ctx, h.logger, h.contextualLogger, queryType, err)
}

// executeQuery handles the query phase with proper observability.
func (h QueryHandler) executeQuery(
	ctx context.Context,
	filter eventstore.Filter,
) (eventstore.StorableEvents, eventstore.MaxSequenceNumberUint, error) {

	// Use eventual consistency for pure query handlers - they can tolerate slightly
	// stale data in exchange for better performance and reduced primary database load
	ctx = eventstore.WithEventualConsistency(ctx)

	queryPhaseStart := time.Now()
	storableEvents, maxSeq, err := h.eventStore.Query(ctx, filter)
	queryPhaseDuration := time.Since(queryPhaseStart)

	if err != nil {
		h.recordComponentTiming(ctx, shell.ComponentQuery, shell.StatusError, queryPhaseDuration)
		return nil, 0, err
	}

	h.recordComponentTiming(ctx, shell.ComponentQuery, shell.StatusSuccess, queryPhaseDuration)

	return storableEvents, maxSeq, nil
}

// executeUnmarshal handles the unmarshal phase with proper observability.
func (h QueryHandler) executeUnmarshal(
	ctx context.Context,
	storableEvents eventstore.StorableEvents,
) (core.DomainEvents, error) {

	unmarshalStart := time.Now()
	history, err := shell.DomainEventsFrom(storableEvents)
	unmarshalDuration := time.Since(unmarshalStart)

	if err != nil {
		h.recordComponentTiming(ctx, shell.ComponentUnmarshal, shell.StatusError, unmarshalDuration)
		return nil, err
	}

	h.recordComponentTiming(ctx, shell.ComponentUnmarshal, shell.StatusSuccess, unmarshalDuration)

	return history, nil
}

// executeProjection handles the projection phase with proper observability.
func (h QueryHandler) executeProjection(
	ctx context.Context,
	history core.DomainEvents,
	query Query,
	maxSeq eventstore.MaxSequenceNumberUint,
) BooksInCirculation {

	projectionStart := time.Now()
	result := Project(history, query, maxSeq)
	projectionDuration := time.Since(projectionStart)

	h.recordComponentTiming(ctx, shell.ComponentProjection, shell.StatusSuccess, projectionDuration)

	return result
}

// recordComponentTiming records component-level timing metrics.
func (h QueryHandler) recordComponentTiming(ctx context.Context, component string, status string, duration time.Duration) {
	shell.RecordQueryComponentDuration(ctx, h.metricsCollector, queryType, component, status, duration)
}

/*** Those methods are necessary to be able to wrap the QueryHandler with a snapshot wrapper ***/

// ExposeEventStore provides access to the EventStore for snapshot wrapper validation.
func (h QueryHandler) ExposeEventStore() shell.QueriesEvents {
	return h.eventStore
}

// ExposeMetricsCollector provides access to the MetricsCollector for snapshot wrapper observability.
func (h QueryHandler) ExposeMetricsCollector() shell.MetricsCollector {
	return h.metricsCollector
}

// ExposeTracingCollector provides access to the TracingCollector for snapshot wrapper observability.
func (h QueryHandler) ExposeTracingCollector() shell.TracingCollector {
	return h.tracingCollector
}

// ExposeContextualLogger provides access to the ContextualLogger for snapshot wrapper observability.
func (h QueryHandler) ExposeContextualLogger() shell.ContextualLogger {
	return h.contextualLogger
}

// ExposeLogger provides access to the Logger for snapshot wrapper observability.
func (h QueryHandler) ExposeLogger() shell.Logger {
	return h.logger
}
