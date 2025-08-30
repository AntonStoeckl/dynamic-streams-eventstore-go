package addbookcopy

import (
	"context"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
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
// It handles only business logic: Query → Unmarshal → Decide → Append.
// All observability concerns are handled by the external observable wrapper.
type CommandHandler struct {
	eventStore EventStore
}

// NewCommandHandler creates a new CommandHandler with the provided EventStore dependency.
func NewCommandHandler(eventStore EventStore) CommandHandler {
	return CommandHandler{
		eventStore: eventStore,
	}
}

// Handle executes the complete command processing workflow: Query → Unmarshal → Decide → Append.
// It implements retry logic for concurrency conflicts and returns explicit HandlerResult.
func (h CommandHandler) Handle(ctx context.Context, command Command) (shell.HandlerResult, error) {
	var isIdempotent bool

	// Execute command with retry logic
	retryMetrics, err := shell.RetryWithExponentialBackoff(ctx, func(retryCtx context.Context) error {
		idempotent, execErr := h.executeCommand(retryCtx, command)
		isIdempotent = idempotent

		return execErr
	})

	// Build HandlerResult with business outcomes and retry metadata
	if isIdempotent {
		return shell.NewIdempotentResult(retryMetrics), err
	}

	if err != nil {
		return shell.NewErrorResult(retryMetrics), err
	}

	return shell.NewSuccessResult(retryMetrics), nil
}

// executeCommand contains the core command processing logic that can be retried.
func (h CommandHandler) executeCommand(ctx context.Context, command Command) (bool, error) {
	filter := BuildEventFilter(command.BookID)

	// Ensure strong consistency for command handlers - they need to see their own writes
	// in the read-check-write pattern to avoid concurrency conflicts from replica lag
	ctx = eventstore.WithStrongConsistency(ctx)

	// Query phase
	storableEvents, maxSequenceNumber, err := h.eventStore.Query(ctx, filter)
	if err != nil {
		return false, err
	}

	// Unmarshal phase
	history, err := shell.DomainEventsFrom(storableEvents)
	if err != nil {
		return false, err
	}

	// Business logic phase - delegate to pure core function
	result := Decide(history, command)

	if !result.HasEventToAppend() {
		return true, nil // Idempotent operation - nothing to do
	}

	// Append phase - single event to append
	uid := uuid.New()
	eventMetadata := shell.BuildEventMetadata(uid, uid, uid)

	storableEvent, marshalErr := shell.StorableEventFrom(result.Event, eventMetadata)
	if marshalErr != nil {
		return false, marshalErr
	}

	appendErr := h.eventStore.Append(ctx, filter, maxSequenceNumber, storableEvent)
	if appendErr != nil {
		return false, appendErr
	}

	return false, result.HasError()
}
