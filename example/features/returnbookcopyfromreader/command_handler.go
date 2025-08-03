package returnbookcopyfromreader

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
)

// EventStore defines the interface needed by the CommandHandler for event store operations.
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
		storableEvents ...eventstore.StorableEvent,
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
//
// Note: The timingCollector parameter is used for testing and benchmarking purposes only.
// In production code, this timing collection would typically not exist or be replaced
// with a proper observability/metrics collection.
func (h CommandHandler) Handle(ctx context.Context, command Command, timingCollector shell.TimingCollector) error {
	filter := h.buildEventFilter(command.BookID, command.ReaderID)

	// Query phase
	start := time.Now()
	storableEvents, maxSequenceNumber, err := h.eventStore.Query(ctx, filter)
	timingCollector.RecordQuery(time.Since(start))
	if err != nil {
		return err
	}

	// Unmarshal phase
	start = time.Now()
	history, err := shell.DomainEventsFrom(storableEvents)
	if err != nil {
		return err
	}
	timingCollector.RecordUnmarshal(time.Since(start))

	// Business logic phase - delegate to pure core function
	start = time.Now()
	eventsToAppend := Decide(history, command)
	timingCollector.RecordBusiness(time.Since(start))

	if len(eventsToAppend) == 0 {
		return nil // nothing to do
	}

	// Append phase - only if there are events to append
	var storableEventsToAppend eventstore.StorableEvents

	for _, eventToAppend := range eventsToAppend {
		uid := uuid.New()
		eventMetadata := shell.BuildEventMetadata(uid, uid, uid)

		storableEvent, marshalErr := shell.StorableEventFrom(eventToAppend, eventMetadata)
		if marshalErr != nil {
			return marshalErr
		}

		storableEventsToAppend = append(storableEventsToAppend, storableEvent)
	}

	start = time.Now()
	appendErr := h.eventStore.Append(ctx, filter, maxSequenceNumber, storableEventsToAppend...)
	timingCollector.RecordAppend(time.Since(start))
	if appendErr != nil {
		return appendErr
	}

	return nil
}

// buildEventFilter creates the filter for querying all events
// related to the specified book and reader which are relevant for this feature/use-case.
func (h CommandHandler) buildEventFilter(bookID uuid.UUID, readerID uuid.UUID) eventstore.Filter {
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
			eventstore.P("ReaderID", readerID.String()),
		).
		Finalize()
}