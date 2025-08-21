package booksincirculation

import (
	"context"
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
)

// EventStore defines the interface needed by the QueryHandler for event store operations.
type EventStore interface {
	Query(ctx context.Context, filter eventstore.Filter) (
		eventstore.StorableEvents,
		eventstore.MaxSequenceNumberUint,
		error,
	)
}

const (
	queryType = "BooksInCirculation"
)

// QueryHandler orchestrates the complete query processing workflow.
// It handles infrastructure concerns like event store interactions, observability instrumentation,
// and delegates projection logic to the pure core functions.
type QueryHandler struct {
	eventStore       EventStore
	metricsCollector shell.MetricsCollector
	tracingCollector shell.TracingCollector
	contextualLogger shell.ContextualLogger
	logger           shell.Logger
}

// NewQueryHandler creates a new QueryHandler with the provided EventStore dependency and options.
func NewQueryHandler(eventStore EventStore, opts ...Option) (QueryHandler, error) {
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
func (h QueryHandler) Handle(ctx context.Context) (BooksInCirculation, error) {
	// Start query handler instrumentation
	queryStart := time.Now()
	ctx, span := shell.StartQuerySpan(ctx, h.tracingCollector, queryType)
	shell.LogQueryStart(ctx, h.logger, h.contextualLogger, queryType)

	filter := BuildEventFilter()

	// Query phase
	queryPhaseStart := time.Now()
	storableEvents, maxSeq, err := h.eventStore.Query(ctx, filter)
	queryPhaseDuration := time.Since(queryPhaseStart)
	if err != nil {
		h.recordComponentTiming(ctx, shell.ComponentQuery, shell.StatusError, queryPhaseDuration)
		h.recordQueryError(ctx, err, time.Since(queryStart), span)
		return BooksInCirculation{}, err
	}
	h.recordComponentTiming(ctx, shell.ComponentQuery, shell.StatusSuccess, queryPhaseDuration)

	// Unmarshal phase
	unmarshalStart := time.Now()
	history, err := shell.DomainEventsFrom(storableEvents)
	unmarshalDuration := time.Since(unmarshalStart)
	if err != nil {
		h.recordComponentTiming(ctx, shell.ComponentUnmarshal, shell.StatusError, unmarshalDuration)
		h.recordQueryError(ctx, err, time.Since(queryStart), span)
		return BooksInCirculation{}, err
	}
	h.recordComponentTiming(ctx, shell.ComponentUnmarshal, shell.StatusSuccess, unmarshalDuration)

	// Projection phase - delegate to a pure core function with sequence tracking
	projectionStart := time.Now()
	result := ProjectBooksInCirculation(history, maxSeq)
	projectionDuration := time.Since(projectionStart)
	h.recordComponentTiming(ctx, shell.ComponentProjection, shell.StatusSuccess, projectionDuration)

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
	shell.RecordQueryMetrics(ctx, h.metricsCollector, queryType, shell.StatusSuccess, duration)
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

	shell.RecordQueryMetrics(ctx, h.metricsCollector, queryType, shell.StatusError, duration)
	shell.FinishQuerySpan(h.tracingCollector, span, shell.StatusError, duration, err)
	shell.LogQueryError(ctx, h.logger, h.contextualLogger, queryType, err)
}

// recordQueryCancelled records canceled query execution with observability.
func (h QueryHandler) recordQueryCancelled(ctx context.Context, err error, duration time.Duration, span shell.SpanContext) {
	shell.RecordQueryMetrics(ctx, h.metricsCollector, queryType, shell.StatusCanceled, duration)
	shell.FinishQuerySpan(h.tracingCollector, span, shell.StatusCanceled, duration, err)
	shell.LogQueryError(ctx, h.logger, h.contextualLogger, queryType, err)
}

// recordQueryTimeout records timeout query execution with observability.
func (h QueryHandler) recordQueryTimeout(ctx context.Context, err error, duration time.Duration, span shell.SpanContext) {
	shell.RecordQueryMetrics(ctx, h.metricsCollector, queryType, shell.StatusTimeout, duration)
	shell.FinishQuerySpan(h.tracingCollector, span, shell.StatusTimeout, duration, err)
	shell.LogQueryError(ctx, h.logger, h.contextualLogger, queryType, err)
}

// recordComponentTiming records component-level timing metrics.
func (h QueryHandler) recordComponentTiming(ctx context.Context, component string, status string, duration time.Duration) {
	shell.RecordQueryComponentDuration(ctx, h.metricsCollector, queryType, component, status, duration)
}
