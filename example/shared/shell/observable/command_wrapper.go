package observable

import (
	"context"
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
)

// CommandWrapper provides comprehensive observability instrumentation for any command handler.
// It wraps a core command handler and adds metrics, tracing, logging, and retry logic.
// The wrapper handles all infrastructure concerns while delegating business logic to the wrapped handler.
// This follows the same composition pattern as snapshot.QueryWrapper for consistency.
type CommandWrapper[C shell.Command] struct {
	coreHandler      shell.CommandHandler[C]
	commandType      string
	metricsCollector shell.MetricsCollector
	tracingCollector shell.TracingCollector
	contextualLogger shell.ContextualLogger
	logger           shell.Logger
}

// NewCommandWrapper creates a new observable wrapper around the core command handler.
// The wrapper will instrument the handler with comprehensive observability while delegating
// all business logic to the wrapped handler.
func NewCommandWrapper[C shell.Command](
	coreHandler shell.CommandHandler[C],
	opts ...CommandOption[C],
) (*CommandWrapper[C], error) {
	// Extract command type from a zero-value instance
	var zeroCommand C
	commandType := zeroCommand.CommandType()

	wrapper := &CommandWrapper[C]{
		coreHandler: coreHandler,
		commandType: commandType,
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(wrapper); err != nil {
			return nil, err
		}
	}

	return wrapper, nil
}

// Handle executes the complete command processing workflow with comprehensive observability.
// It instruments the operation with metrics, tracing, and logging while delegating the actual
// business logic to the wrapped core handler.
//
// The wrapper provides pure observability decoration, translating HandlerResult into metrics.
func (w *CommandWrapper[C]) Handle(ctx context.Context, command C) (shell.HandlerResult, error) {
	// Start command handler instrumentation
	commandStart := time.Now()
	ctx, span := shell.StartCommandSpan(ctx, w.tracingCollector, w.commandType)
	shell.LogCommandStart(ctx, w.logger, w.contextualLogger, w.commandType)

	// Delegate directly to the core handler and get an explicit result and error
	result, err := w.coreHandler.Handle(ctx, command)

	// Record retry metrics from the explicit result metadata
	w.recordRetryMetrics(ctx, result)

	if err != nil {
		w.recordCommandError(ctx, err, time.Since(commandStart), span)
		return result, err
	}

	// Determine the success type from explicit business outcome
	if result.Idempotent {
		w.recordCommandSuccess(ctx, shell.StatusIdempotent, time.Since(commandStart), span)
	} else {
		w.recordCommandSuccess(ctx, "success", time.Since(commandStart), span)
	}

	return result, nil
}

// CommandOption defines a functional option for configuring CommandWrapper.
type CommandOption[C shell.Command] func(*CommandWrapper[C]) error

// WithCommandMetrics sets the metrics collector for the CommandWrapper.
func WithCommandMetrics[C shell.Command](collector shell.MetricsCollector) CommandOption[C] {
	return func(w *CommandWrapper[C]) error {
		w.metricsCollector = collector
		return nil
	}
}

// WithCommandTracing sets the tracing collector for the CommandWrapper.
func WithCommandTracing[C shell.Command](collector shell.TracingCollector) CommandOption[C] {
	return func(w *CommandWrapper[C]) error {
		w.tracingCollector = collector
		return nil
	}
}

// WithCommandContextualLogging sets the contextual logger for the CommandWrapper.
func WithCommandContextualLogging[C shell.Command](logger shell.ContextualLogger) CommandOption[C] {
	return func(w *CommandWrapper[C]) error {
		w.contextualLogger = logger
		return nil
	}
}

// WithCommandLogging sets the basic logger for the CommandWrapper.
func WithCommandLogging[C shell.Command](logger shell.Logger) CommandOption[C] {
	return func(w *CommandWrapper[C]) error {
		w.logger = logger
		return nil
	}
}

/*** Observability helper methods ***/

// recordCommandSuccess records successful command execution with observability.
func (w *CommandWrapper[C]) recordCommandSuccess(ctx context.Context, businessOutcome string, duration time.Duration, span shell.SpanContext) {
	shell.RecordCommandMetrics(ctx, w.metricsCollector, w.commandType, businessOutcome, duration)
	shell.FinishCommandSpan(w.tracingCollector, span, businessOutcome, duration, nil)
	shell.LogCommandSuccess(ctx, w.logger, w.contextualLogger, w.commandType, businessOutcome, duration)
}

// recordCommandError records failed command execution with observability.
func (w *CommandWrapper[C]) recordCommandError(ctx context.Context, err error, duration time.Duration, span shell.SpanContext) {
	if shell.IsCancellationError(err) {
		w.recordCommandCancelled(ctx, err, duration, span)
		return
	}

	if shell.IsTimeoutError(err) {
		w.recordCommandTimeout(ctx, err, duration, span)
		return
	}

	if shell.IsConcurrencyConflictError(err) {
		w.recordCommandConcurrencyConflict(ctx, err, duration, span)
		return
	}

	shell.RecordCommandMetrics(ctx, w.metricsCollector, w.commandType, shell.StatusError, duration)
	shell.FinishCommandSpan(w.tracingCollector, span, shell.StatusError, duration, err)
	shell.LogCommandError(ctx, w.logger, w.contextualLogger, w.commandType, err)
}

// recordCommandCancelled records canceled command execution with observability.
func (w *CommandWrapper[C]) recordCommandCancelled(ctx context.Context, err error, duration time.Duration, span shell.SpanContext) {
	shell.RecordCommandMetrics(ctx, w.metricsCollector, w.commandType, shell.StatusCanceled, duration)
	shell.FinishCommandSpan(w.tracingCollector, span, shell.StatusCanceled, duration, err)
	shell.LogCommandError(ctx, w.logger, w.contextualLogger, w.commandType, err)
}

// recordCommandTimeout records timeout command execution with observability.
func (w *CommandWrapper[C]) recordCommandTimeout(ctx context.Context, err error, duration time.Duration, span shell.SpanContext) {
	shell.RecordCommandMetrics(ctx, w.metricsCollector, w.commandType, shell.StatusTimeout, duration)
	shell.FinishCommandSpan(w.tracingCollector, span, shell.StatusTimeout, duration, err)
	shell.LogCommandError(ctx, w.logger, w.contextualLogger, w.commandType, err)
}

// recordCommandConcurrencyConflict records concurrency conflict command execution with observability.
func (w *CommandWrapper[C]) recordCommandConcurrencyConflict(ctx context.Context, err error, duration time.Duration, span shell.SpanContext) {
	shell.RecordCommandMetrics(ctx, w.metricsCollector, w.commandType, shell.StatusConcurrencyConflict, duration)
	shell.FinishCommandSpan(w.tracingCollector, span, shell.StatusConcurrencyConflict, duration, err)
	shell.LogCommandError(ctx, w.logger, w.contextualLogger, w.commandType, err)
}

// recordRetryMetrics records retry execution metadata from the handler result.
func (w *CommandWrapper[C]) recordRetryMetrics(ctx context.Context, result shell.HandlerResult) {
	if w.metricsCollector == nil {
		return
	}

	// Record retry attempts and delay metrics based on the explicit metadata
	if result.RetryAttempts > 1 {
		// Record the total retry attempts for this command
		retryLabels := shell.BuildRetryLabels(w.commandType, result.RetryAttempts-1, result.LastErrorType)
		if contextualCollector, ok := w.metricsCollector.(shell.ContextualMetricsCollector); ok {
			contextualCollector.IncrementCounterContext(ctx, shell.CommandHandlerRetriesMetric, retryLabels)
		} else {
			w.metricsCollector.IncrementCounter(shell.CommandHandlerRetriesMetric, retryLabels)
		}

		// Record the total delay spent in retries
		delayLabels := map[string]string{
			shell.LogAttrCommandType: w.commandType,
		}
		if contextualCollector, ok := w.metricsCollector.(shell.ContextualMetricsCollector); ok {
			contextualCollector.RecordDurationContext(ctx, shell.CommandHandlerRetryDelayMetric, result.TotalRetryDelay, delayLabels)
		} else {
			w.metricsCollector.RecordDuration(shell.CommandHandlerRetryDelayMetric, result.TotalRetryDelay, delayLabels)
		}
	}

	// Record when max retries were exhausted
	if result.RetriesExhausted {
		exhaustedLabels := map[string]string{
			shell.LogAttrCommandType: w.commandType,
		}
		if contextualCollector, ok := w.metricsCollector.(shell.ContextualMetricsCollector); ok {
			contextualCollector.IncrementCounterContext(ctx, shell.CommandHandlerMaxRetriesReachedMetric, exhaustedLabels)
		} else {
			w.metricsCollector.IncrementCounter(shell.CommandHandlerMaxRetriesReachedMetric, exhaustedLabels)
		}
	}
}
