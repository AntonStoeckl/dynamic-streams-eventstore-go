package bookscurrentlylentbyreader

import (
	"context"

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
func (h QueryHandler) Handle(ctx context.Context, query Query) (BooksCurrentlyLent, error) {
	filter := h.buildEventFilter(query.ReaderID)

	// Query phase
	storableEvents, _, err := h.eventStore.Query(ctx, filter)
	if err != nil {
		return BooksCurrentlyLent{}, err
	}

	// Unmarshal phase
	history, err := shell.DomainEventsFrom(storableEvents)
	if err != nil {
		return BooksCurrentlyLent{}, err
	}

	// Projection phase - delegate to pure core function
	result := ProjectBooksCurrentlyLent(history, query)

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