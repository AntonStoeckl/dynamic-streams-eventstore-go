package bookscurrentlylentbyreader

import (
	"context"
	"time"

	"github.com/google/uuid"

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

// QueryHandler orchestrates the complete query processing workflow.
// It handles infrastructure concerns like event store interactions, timing collection,
// and delegates projection logic to the pure core functions.
type QueryHandler struct {
	eventStore EventStore
}

// NewQueryHandler creates a new QueryHandler with the provided EventStore dependency.
func NewQueryHandler(eventStore EventStore) QueryHandler {
	return QueryHandler{
		eventStore: eventStore,
	}
}

// Handle executes the complete query processing workflow: Query -> Project.
// It queries the current event history and delegates projection logic to the core function.
//
// Note: The timingCollector parameter is used for testing and benchmarking purposes only.
// In production code, this timing collection would typically not exist or be replaced
// with a proper observability/metrics collection.
func (h QueryHandler) Handle(
	ctx context.Context, 
	query Query, 
	timingCollector shell.TimingCollector,
) (BooksCurrentlyLent, error) {
	filter := h.buildEventFilter(query.ReaderID)

	// Query phase
	start := time.Now()
	storableEvents, _, err := h.eventStore.Query(ctx, filter)
	timingCollector.RecordQuery(time.Since(start))
	if err != nil {
		return BooksCurrentlyLent{}, err
	}

	// Unmarshal phase
	start = time.Now()
	history, err := shell.DomainEventsFrom(storableEvents)
	if err != nil {
		return BooksCurrentlyLent{}, err
	}
	timingCollector.RecordUnmarshal(time.Since(start))

	// Projection phase - delegate to pure core function
	start = time.Now()
	result := ProjectBooksCurrentlyLent(history, query)
	timingCollector.RecordBusiness(time.Since(start))

	return result, nil
}

// buildEventFilter creates the filter for querying all events
// related to the specified reader and all book circulation events
// which are relevant for this query/use-case.
func (h QueryHandler) buildEventFilter(readerID uuid.UUID) eventstore.Filter {
	return eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			core.BookCopyAddedToCirculationEventType,
			core.BookCopyRemovedFromCirculationEventType,
			core.BookCopyLentToReaderEventType,
			core.BookCopyReturnedByReaderEventType,
		).
		AndAnyPredicateOf(
			eventstore.P("ReaderID", readerID.String()),
		).
		Finalize()
}