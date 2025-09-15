package removedbooks

import (
	"context"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
)

// QueryHandler orchestrates the complete query processing workflow.
// It handles event store interactions and delegates projection logic to the pure core functions.
type QueryHandler struct {
	eventStore shell.QueriesEvents
}

// NewQueryHandler creates a new QueryHandler with the provided EventStore dependency.
func NewQueryHandler(eventStore shell.QueriesEvents) QueryHandler {
	return QueryHandler{
		eventStore: eventStore,
	}
}

// Handle executes the complete query processing workflow: Query -> Project.
// It queries the current event history and delegates projection logic to the core function.
func (h QueryHandler) Handle(ctx context.Context, query Query) (RemovedBooks, error) {
	filter := BuildEventFilter()

	// Use eventual consistency for pure query handlers - they can tolerate slightly
	// stale data in exchange for better performance and reduced primary database load
	ctx = eventstore.WithEventualConsistency(ctx)

	// Query phase
	storableEvents, maxSeq, err := h.eventStore.Query(ctx, filter)
	if err != nil {
		return RemovedBooks{}, err
	}

	// Unmarshal phase
	history, err := shell.DomainEventsFrom(storableEvents)
	if err != nil {
		return RemovedBooks{}, err
	}

	// Projection phase
	result := Project(history, query, maxSeq)

	return result, nil
}
