package lendbookcopytoreader

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

// CommandHandler orchestrates the complete command processing workflow with pure business logic and retry.
// It handles the core event sourcing workflow: Query -> Unmarshal -> Decide -> Append.
// External wrappers handle all observability concerns.
type CommandHandler struct {
	eventStore   EventStore
	retryOptions []shell.RetryOption
}

// Option configures a CommandHandler.
type Option func(*CommandHandler)

// WithRetryOptions sets a custom retry configuration for the handler.
func WithRetryOptions(opts ...shell.RetryOption) Option {
	return func(h *CommandHandler) {
		h.retryOptions = opts
	}
}

// NewCommandHandler creates a new CommandHandler with optional configuration.
// Signature is backward compatible - existing calls work unchanged.
func NewCommandHandler(eventStore EventStore, opts ...Option) CommandHandler {
	handler := CommandHandler{
		eventStore: eventStore,
		// retryOptions defaults to nil (will use retry defaults)
	}

	for _, opt := range opts {
		opt(&handler)
	}

	return handler
}

// Handle executes the complete command processing workflow with retry logic.
// It delegates business logic to executeCommand and handles retry with exponential backoff.
// Returns HandlerResult containing business outcomes and execution metadata for observability.
//
// Resilience: Implements exponential backoff retry logic for concurrency conflicts.
func (h CommandHandler) Handle(ctx context.Context, command Command) (shell.HandlerResult, error) {
	var isIdempotent bool

	// Execute command with retry logic (no observability - that's in the wrapper)
	retryMetrics, err := shell.RetryWithExponentialBackoff(ctx, func(retryCtx context.Context) error {
		idempotent, execErr := h.executeCommand(retryCtx, command)
		isIdempotent = idempotent

		return execErr
	}, h.retryOptions...)

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
	filter := BuildEventFilter(command.BookID, command.ReaderID)

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
		return true, nil // Idempotent success - no event to append
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
