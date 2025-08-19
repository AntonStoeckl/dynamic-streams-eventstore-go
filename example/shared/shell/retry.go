package shell

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

const (
	defaultMaxAttempts  = 6
	defaultBaseDelay    = 10 * time.Millisecond
	defaultJitterFactor = 0.3
)

var (
	// ErrNilMetricsCollector is returned when a nil metrics collector is provided to WithMetrics.
	ErrNilMetricsCollector = errors.New("metrics collector must not be nil")

	// ErrEmptyCommandType is returned when an empty command type is provided to WithMetrics.
	ErrEmptyCommandType = errors.New("command type must not be empty")

	// ErrInvalidMaxAttempts is returned when max attempts are not positive.
	ErrInvalidMaxAttempts = errors.New("max attempts must be positive")

	// ErrNegativeBaseDelay is returned when the base delay is negative.
	ErrNegativeBaseDelay = errors.New("base delay must not be negative")

	// ErrInvalidJitterFactor is returned when the jitter factor is not between 0.0 and 1.0.
	ErrInvalidJitterFactor = errors.New("jitter factor must be between 0.0 and 1.0")
)

// RetryableFunc represents a function that can be retried.
type RetryableFunc func(ctx context.Context) error

// retryConfig holds configuration for exponential backoff retry logic.
type retryConfig struct {
	maxAttempts      int
	baseDelay        time.Duration
	jitterFactor     float64
	metricsCollector MetricsCollector
	commandType      string
}

// RetryWithExponentialBackoff implements optimistic concurrency retry logic.
// It executes the provided function with exponential backoff retry logic,
// retrying only on retryable errors up to maxAttempts times.
//
// Retry Schedule (default): 0 ms, 10 ms, 20 ms, 40 ms, 80 ms (with 30% jitter)
// Total Duration: ~ 200 ms worst case
// Use Case: EventStore concurrency conflicts in high-load scenarios
//
// Only ErrConcurrencyConflict is retried - all other errors fail fast.
func RetryWithExponentialBackoff(
	ctx context.Context,
	fn RetryableFunc,
	options ...RetryOption,
) error {
	config := &retryConfig{
		maxAttempts:  defaultMaxAttempts,
		baseDelay:    defaultBaseDelay,
		jitterFactor: defaultJitterFactor,
	}

	for _, option := range options {
		if err := option(config); err != nil {
			return err
		}
	}

	var lastErr error

	for attempt := 0; attempt < config.maxAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff: baseDelay * 2^(attempt-1)
			delay := config.baseDelay * time.Duration(1<<(attempt-1))

			// Add jitter to prevent thundering herd
			jitter := rand.Float64() * float64(delay) * config.jitterFactor //nolint:gosec //math/rand is sufficient for jitter
			backoffDelay := delay + time.Duration(jitter)

			// Record retry delay metric
			recordRetryDelayMetric(ctx, config, attempt, backoffDelay)

			select {
			case <-time.After(backoffDelay):
				// Continue with retry
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			return nil // Success
		}

		// Check if the error is retryable
		if !isRetryableError(lastErr) {
			return lastErr // Permanent failure
		}

		// Record retry attempt metric (only for retryable errors that will be retried)
		recordRetryAttemptMetric(ctx, attempt, config, lastErr)
	}

	recordMaxRetriesReachedMetric(ctx, config, lastErr)

	return lastErr // Max attempts reached
}

// recordRetryDelayMetric records the actual backoff delay before each retry attempt.
func recordRetryDelayMetric(ctx context.Context, config *retryConfig, attempt int, backoffDelay time.Duration) {
	if config.metricsCollector != nil {
		delayLabels := map[string]string{
			LogAttrCommandType: config.commandType,
			"attempt_number":   fmt.Sprintf("%d", attempt),
		}

		if contextualCollector, ok := config.metricsCollector.(ContextualMetricsCollector); ok {
			contextualCollector.RecordDurationContext(ctx, CommandHandlerRetryDelayMetric, backoffDelay, delayLabels)
		} else {
			config.metricsCollector.RecordDuration(CommandHandlerRetryDelayMetric, backoffDelay, delayLabels)
		}
	}
}

// recordRetryAttemptMetric tracks retry attempts by command type, attempt number, and error type.
func recordRetryAttemptMetric(ctx context.Context, attempt int, config *retryConfig, lastErr error) {
	if attempt < config.maxAttempts-1 && config.metricsCollector != nil {
		retryLabels := BuildRetryLabels(config.commandType, attempt+1, getErrorType(lastErr))

		if contextualCollector, ok := config.metricsCollector.(ContextualMetricsCollector); ok {
			contextualCollector.IncrementCounterContext(ctx, CommandHandlerRetriesMetric, retryLabels)
		} else {
			config.metricsCollector.IncrementCounter(CommandHandlerRetriesMetric, retryLabels)
		}
	}
}

// recordMaxRetriesReachedMetric tracks when retry exhaustion occurs with the final error type.
func recordMaxRetriesReachedMetric(ctx context.Context, config *retryConfig, lastErr error) {
	if config.metricsCollector != nil {
		maxRetriesLabels := map[string]string{
			LogAttrCommandType: config.commandType,
			"final_error_type": getErrorType(lastErr),
		}

		if contextualCollector, ok := config.metricsCollector.(ContextualMetricsCollector); ok {
			contextualCollector.IncrementCounterContext(ctx, CommandHandlerMaxRetriesReachedMetric, maxRetriesLabels)
		} else {
			config.metricsCollector.IncrementCounter(CommandHandlerMaxRetriesReachedMetric, maxRetriesLabels)
		}
	}
}

// isRetryableError determines if an error should be retried.
// Currently, only concurrency conflicts are considered retryable.
//
// A context.DeadlineExceeded is NOT retryable - retrying timeouts during overload creates cascade failures.
// Timeout errors should fail fast to provide clear signals about system capacity issues.
// Alternative resilience: circuit breakers, load shedding, differentiated timeouts (not retry).
// Other database errors are NOT retryable for now (baby steps).
func isRetryableError(err error) bool {
	// Concurrency conflicts are always retryable
	if errors.Is(err, eventstore.ErrConcurrencyConflict) {
		return true
	}

	return false
}

// getErrorType extracts a string representation of the error type for metrics labeling.
func getErrorType(err error) string {
	if err == nil {
		return "none"
	}
	if errors.Is(err, eventstore.ErrConcurrencyConflict) {
		return "concurrency_conflict"
	}
	if errors.Is(err, context.Canceled) {
		return "context_canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "context_deadline_exceeded"
	}

	return "other"
}

// RetryOption configures retry behavior using the functional options pattern.
type RetryOption func(*retryConfig) error

// WithMaxAttempts sets the maximum number of retry attempts.
func WithMaxAttempts(attempts int) RetryOption {
	return func(config *retryConfig) error {
		if attempts <= 0 {
			return ErrInvalidMaxAttempts
		}

		config.maxAttempts = attempts

		return nil
	}
}

// WithBaseDelay sets the base delay for exponential backoff.
// Actual delays: baseDelay, baseDelay*2, baseDelay*4, baseDelay*8, etc.
func WithBaseDelay(delay time.Duration) RetryOption {
	return func(config *retryConfig) error {
		if delay < 0 {
			return ErrNegativeBaseDelay
		}

		config.baseDelay = delay

		return nil
	}
}

// WithJitterFactor sets the jitter factor to prevent thundering herd problems.
// Jitter is added as a percentage of the calculated backoff delay.
// Valid range: 0.0 (no jitter) to 1.0 (100% jitter).
func WithJitterFactor(factor float64) RetryOption {
	return func(config *retryConfig) error {
		if factor < 0.0 || factor > 1.0 {
			return ErrInvalidJitterFactor
		}

		config.jitterFactor = factor

		return nil
	}
}

// WithMetrics sets the metrics collector for retry instrumentation.
// Requires commandType to properly label metrics.
func WithMetrics(collector MetricsCollector, commandType string) RetryOption {
	return func(config *retryConfig) error {
		if collector == nil {
			return ErrNilMetricsCollector
		}

		if commandType == "" {
			return ErrEmptyCommandType
		}

		config.metricsCollector = collector
		config.commandType = commandType

		return nil
	}
}
