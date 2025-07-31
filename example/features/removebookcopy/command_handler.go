package removebookcopy

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"

	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"
)

// EventStore defines the interface needed by the CommandHandler for event store operations
type EventStore interface {
	Query(ctx context.Context, filter eventstore.Filter) (
		eventstore.StorableEvents,
		eventstore.MaxSequenceNumberUint,
		error,
	)
	Append(
		ctx context.Context,
		filter eventstore.Filter,
		expectedMaxSequenceNumber eventstore.MaxSequenceNumberUint,
		event eventstore.StorableEvent,
		events ...eventstore.StorableEvent,
	) error
}

// CommandHandler orchestrates the complete command processing workflow.
// It handles infrastructure concerns like event store interactions, timing collection,
// and delegates business logic decisions to the pure core functions.
type CommandHandler struct {
	eventStore EventStore
}

// NewCommandHandler creates a new CommandHandler with the provided EventStore dependency.
func NewCommandHandler(eventStore EventStore) CommandHandler {
	return CommandHandler{
		eventStore: eventStore,
	}
}

// Handle executes the complete command processing workflow: Query -> Decide -> Append.
// It queries the current event history, delegates business logic to the core Decide function,
// and appends any resulting events while collecting detailed timing measurements.
func (h CommandHandler) Handle(ctx context.Context, command Command, timingCollector TimingCollector) error {
	filter := h.buildEventFilter(command.BookID)

	// Query phase
	start := time.Now()
	storableEvents, maxSequenceNumber, err := h.eventStore.Query(ctx, filter)
	timingCollector.RecordQuery(time.Since(start))
	if err != nil {
		return err
	}

	// Unmarshal phase
	start = time.Now()
	domainEvents, err := shell.DomainEventsFrom(storableEvents)
	if err != nil {
		return err
	}
	timingCollector.RecordUnmarshal(time.Since(start))

	// Business logic phase - delegate to pure core function
	start = time.Now()
	eventsToAppend := Decide(domainEvents, command)
	timingCollector.RecordBusiness(time.Since(start))

	// Append phase - only if there are events to append
	if len(eventsToAppend) > 0 {
		for _, domainEvent := range eventsToAppend {
			storableEvent, marshalErr := shell.StorableEventFrom(domainEvent, shell.EventMetadata{})
			if marshalErr != nil {
				return marshalErr
			}

			start = time.Now()
			appendErr := h.eventStore.Append(ctx, filter, maxSequenceNumber, storableEvent)
			timingCollector.RecordAppend(time.Since(start))
			if appendErr != nil {
				return appendErr
			}
		}
	}

	return nil
}

// buildEventFilter creates the filter for querying all events related to the specified book
func (h CommandHandler) buildEventFilter(bookID uuid.UUID) eventstore.Filter {
	return eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			core.BookCopyAddedToCirculationEventType,
			core.BookCopyRemovedFromCirculationEventType,
			core.BookCopyLentToReaderEventType,
			core.BookCopyReturnedByReaderEventType,
		).
		AndAnyPredicateOf(
			eventstore.P("BookID", bookID.String()),
		).
		Finalize()
}
