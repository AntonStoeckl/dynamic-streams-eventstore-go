package booksincirculation

import (
	"context"
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
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
	ctx, span := shell.StartCommandSpan(ctx, h.tracingCollector, queryType)
	shell.LogCommandStart(ctx, h.logger, h.contextualLogger, queryType)

	filter := h.buildEventFilter()

	// Query phase
	storableEvents, _, err := h.eventStore.Query(ctx, filter)
	if err != nil {
		h.recordQueryError(ctx, err, time.Since(queryStart), span)
		return BooksInCirculation{}, err
	}

	// Unmarshal phase
	history, err := shell.DomainEventsFrom(storableEvents)
	if err != nil {
		h.recordQueryError(ctx, err, time.Since(queryStart), span)
		return BooksInCirculation{}, err
	}

	// Projection phase - delegate to pure core function
	result := ProjectBooksInCirculation(history)

	h.recordQuerySuccess(ctx, time.Since(queryStart), span)

	return result, nil
}

// buildEventFilter creates the filter for querying all book circulation events
// which are relevant for this query/use-case.
func (h QueryHandler) buildEventFilter() eventstore.Filter {
	return eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			core.BookCopyAddedToCirculationEventType,
			core.BookCopyRemovedFromCirculationEventType,
			core.BookCopyLentToReaderEventType,
			core.BookCopyReturnedByReaderEventType,
		).
		Finalize()
}

/*** Query Handler Options and helper methods for observability ***/

// Option defines a functional option for configuring QueryHandler.
type Option func(*QueryHandler) error

// recordQuerySuccess records successful query execution with observability.
func (h QueryHandler) recordQuerySuccess(ctx context.Context, duration time.Duration, span shell.SpanContext) {
	shell.RecordCommandMetrics(ctx, h.metricsCollector, queryType, shell.StatusSuccess, duration)
	shell.FinishCommandSpan(h.tracingCollector, span, shell.StatusSuccess, duration, nil)
	shell.LogCommandSuccess(ctx, h.logger, h.contextualLogger, queryType, shell.StatusSuccess, duration)
}

// recordQueryError records failed query execution with observability.
func (h QueryHandler) recordQueryError(ctx context.Context, err error, duration time.Duration, span shell.SpanContext) {
	shell.RecordCommandMetrics(ctx, h.metricsCollector, queryType, shell.StatusError, duration)
	shell.FinishCommandSpan(h.tracingCollector, span, shell.StatusError, duration, err)
	shell.LogCommandError(ctx, h.logger, h.contextualLogger, queryType, err)
}

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
