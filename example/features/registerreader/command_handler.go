package registerreader

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
)

const (
	commandType = "RegisterReader"
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
	eventStore       EventStore
	metricsCollector shell.MetricsCollector
	tracingCollector shell.TracingCollector
	contextualLogger shell.ContextualLogger
	logger           shell.Logger
}

// NewCommandHandler creates a new CommandHandler with the provided EventStore dependency and options.
func NewCommandHandler(eventStore EventStore, opts ...Option) (CommandHandler, error) {
	h := CommandHandler{
		eventStore: eventStore,
	}

	for _, opt := range opts {
		if err := opt(&h); err != nil {
			return CommandHandler{}, err
		}
	}

	return h, nil
}

// Handle executes the complete command processing workflow: Query -> Decide -> Append.
// It queries the current event history, delegates business logic to the core Decide function,
// and appends any resulting events with comprehensive observability instrumentation.
func (h CommandHandler) Handle(ctx context.Context, command Command) error {
	// Start command handler instrumentation
	commandStart := time.Now()
	ctx, span := shell.StartCommandSpan(ctx, h.tracingCollector, commandType)
	shell.LogCommandStart(ctx, h.logger, h.contextualLogger, commandType)

	filter := h.buildEventFilter(command.ReaderID)

	// Query phase
	storableEvents, maxSequenceNumber, err := h.eventStore.Query(ctx, filter)
	if err != nil {
		h.recordCommandError(ctx, err, time.Since(commandStart), span)
		return err
	}

	// Unmarshal phase
	history, err := shell.DomainEventsFrom(storableEvents)
	if err != nil {
		h.recordCommandError(ctx, err, time.Since(commandStart), span)
		return err
	}

	// Business logic phase - delegate to pure core function
	result := Decide(history, command)

	if !result.HasEventToAppend() {
		h.recordCommandSuccess(ctx, result.Outcome, time.Since(commandStart), span)
		return nil // nothing to do
	}

	// Append phase - single event to append
	uid := uuid.New()
	eventMetadata := shell.BuildEventMetadata(uid, uid, uid)

	storableEvent, marshalErr := shell.StorableEventFrom(result.Event, eventMetadata)
	if marshalErr != nil {
		h.recordCommandError(ctx, marshalErr, time.Since(commandStart), span)
		return marshalErr
	}

	appendErr := h.eventStore.Append(ctx, filter, maxSequenceNumber, storableEvent)
	if appendErr != nil {
		h.recordCommandError(ctx, appendErr, time.Since(commandStart), span)
		return appendErr
	}

	h.recordCommandSuccess(ctx, result.Outcome, time.Since(commandStart), span)

	return nil
}

// buildEventFilter creates the filter for querying all events
// related to the specified reader which are relevant for this feature/use-case.
func (h CommandHandler) buildEventFilter(readerID uuid.UUID) eventstore.Filter {
	return eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			core.ReaderRegisteredEventType,
			core.ReaderContractCanceledEventType,
		).
		AndAnyPredicateOf(
			eventstore.P("ReaderID", readerID.String()),
		).
		Finalize()
}

/*** Command Handler Options and helper methods for observability ***/

// Option defines a functional option for configuring CommandHandler.
type Option func(*CommandHandler) error

// WithMetrics sets the metrics collector for the CommandHandler.
func WithMetrics(collector shell.MetricsCollector) Option {
	return func(h *CommandHandler) error {
		h.metricsCollector = collector
		return nil
	}
}

// WithTracing sets the tracing collector for the CommandHandler.
func WithTracing(collector shell.TracingCollector) Option {
	return func(h *CommandHandler) error {
		h.tracingCollector = collector
		return nil
	}
}

// WithContextualLogging sets the contextual logger for the CommandHandler.
func WithContextualLogging(logger shell.ContextualLogger) Option {
	return func(h *CommandHandler) error {
		h.contextualLogger = logger
		return nil
	}
}

// WithLogging sets the basic logger for the CommandHandler.
func WithLogging(logger shell.Logger) Option {
	return func(h *CommandHandler) error {
		h.logger = logger
		return nil
	}
}

// recordCommandSuccess records successful command execution with observability.
func (h CommandHandler) recordCommandSuccess(ctx context.Context, businessOutcome string, duration time.Duration, span shell.SpanContext) {
	shell.RecordCommandMetrics(ctx, h.metricsCollector, commandType, businessOutcome, duration)
	shell.FinishCommandSpan(h.tracingCollector, span, businessOutcome, duration, nil)
	shell.LogCommandSuccess(ctx, h.logger, h.contextualLogger, commandType, businessOutcome, duration)
}

// recordCommandError records failed command execution with observability.
func (h CommandHandler) recordCommandError(ctx context.Context, err error, duration time.Duration, span shell.SpanContext) {
	if shell.IsCancellationError(err) {
		h.recordCommandCancelled(ctx, err, duration, span)
		return
	}

	if shell.IsTimeoutError(err) {
		h.recordCommandTimeout(ctx, err, duration, span)
		return
	}

	shell.RecordCommandMetrics(ctx, h.metricsCollector, commandType, shell.StatusError, duration)
	shell.FinishCommandSpan(h.tracingCollector, span, shell.StatusError, duration, err)
	shell.LogCommandError(ctx, h.logger, h.contextualLogger, commandType, err)
}

// recordCommandCancelled records canceled command execution with observability.
func (h CommandHandler) recordCommandCancelled(ctx context.Context, err error, duration time.Duration, span shell.SpanContext) {
	shell.RecordCommandMetrics(ctx, h.metricsCollector, commandType, shell.StatusCanceled, duration)
	shell.FinishCommandSpan(h.tracingCollector, span, shell.StatusCanceled, duration, err)
	shell.LogCommandError(ctx, h.logger, h.contextualLogger, commandType, err)
}

// recordCommandTimeout records timeout command execution with observability.
func (h CommandHandler) recordCommandTimeout(ctx context.Context, err error, duration time.Duration, span shell.SpanContext) {
	shell.RecordCommandMetrics(ctx, h.metricsCollector, commandType, shell.StatusTimeout, duration)
	shell.FinishCommandSpan(h.tracingCollector, span, shell.StatusTimeout, duration, err)
	shell.LogCommandError(ctx, h.logger, h.contextualLogger, commandType, err)
}
