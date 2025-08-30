package shell

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

// Default retry configuration values.
const (
	defaultMaxAttempts  = 6
	defaultBaseDelay    = 10 * time.Millisecond
	defaultJitterFactor = 0.3
)

// Error type constants for consistent error classification across retry and handler results.
const (
	ErrorTypeNone                    = "none"
	ErrorTypeConcurrencyConflict     = "concurrency_conflict"
	ErrorTypeContextCanceled         = "context_canceled"
	ErrorTypeContextDeadlineExceeded = "context_deadline_exceeded"
	ErrorTypeOther                   = "other"
)

var (
	// ErrInvalidMaxAttempts is returned when max attempts are not positive.
	ErrInvalidMaxAttempts = errors.New("max attempts must be positive")

	// ErrNegativeBaseDelay is returned when the base delay is negative.
	ErrNegativeBaseDelay = errors.New("base delay must not be negative")

	// ErrInvalidJitterFactor is returned when the jitter factor is not between 0.0 and 1.0.
	ErrInvalidJitterFactor = errors.New("jitter factor must be between 0.0 and 1.0")
)

// RetryableFunc represents a function that can be retried.
type RetryableFunc func(ctx context.Context) error

// RetryMetrics contains execution metrics from retry operations.
type RetryMetrics struct {
	// Attempts is the total number of attempts made (1 for no retries, 2+ for retries).
	Attempts int

	// TotalDelay is the cumulative time spent in retry backoff delays.
	TotalDelay time.Duration

	// LastErrorType describes the type of the final error encountered.
	LastErrorType string

	// RetriesExhausted indicates whether max retry attempts were exhausted with a retryable error.
	RetriesExhausted bool
}

// retryConfig holds configuration for exponential backoff retry logic.
type retryConfig struct {
	maxAttempts  int
	baseDelay    time.Duration
	jitterFactor float64
}

// RetryWithExponentialBackoff implements optimistic concurrency retry logic.
// It executes the provided function with exponential backoff retry logic,
// retrying only on retryable errors up to maxAttempts times.
//
// Returns retry metadata along with any error, allowing callers to access
// execution information (attempts, delays, error types) for observability.
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
) (RetryMetrics, error) {
	config := &retryConfig{
		maxAttempts:  defaultMaxAttempts,
		baseDelay:    defaultBaseDelay,
		jitterFactor: defaultJitterFactor,
	}

	for _, option := range options {
		if err := option(config); err != nil {
			return RetryMetrics{
				Attempts:         0,              // No attempts were made
				TotalDelay:       0,              // No delay occurred
				LastErrorType:    ErrorTypeOther, // Configuration error
				RetriesExhausted: false,          // Config error, no retries attempted
			}, err
		}
	}

	var lastErr error
	var totalDelay time.Duration
	var attempts int

	for attempt := 0; attempt < config.maxAttempts; attempt++ {
		attempts = attempt + 1

		if attempt > 0 {
			// Exponential backoff: baseDelay * 2^(attempt-1)
			delay := config.baseDelay * time.Duration(1<<(attempt-1))

			// Add jitter to prevent thundering herd
			jitter := rand.Float64() * float64(delay) * config.jitterFactor //nolint:gosec //math/rand is sufficient for jitter
			backoffDelay := delay + time.Duration(jitter)

			// Accumulate total delay for metadata
			totalDelay += backoffDelay

			select {
			case <-time.After(backoffDelay):
				// Continue with retry
			case <-ctx.Done():
				return RetryMetrics{
					Attempts:         attempts,
					TotalDelay:       totalDelay,
					LastErrorType:    getErrorType(ctx.Err()),
					RetriesExhausted: false, // Context canceled, not exhausted
				}, ctx.Err()
			}
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			break // Success
		}

		// Check if the error is retryable
		if !isRetryableError(lastErr) {
			break // Permanent failure
		}
	}

	// Calculate if retries were exhausted
	retriesExhausted := attempts == config.maxAttempts && lastErr != nil && isRetryableError(lastErr)

	return RetryMetrics{
		Attempts:         attempts,
		TotalDelay:       totalDelay,
		LastErrorType:    getErrorType(lastErr),
		RetriesExhausted: retriesExhausted,
	}, lastErr
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
	switch {
	case err == nil:
		return ErrorTypeNone
	case errors.Is(err, eventstore.ErrConcurrencyConflict):
		return ErrorTypeConcurrencyConflict
	case errors.Is(err, context.Canceled):
		return ErrorTypeContextCanceled
	case errors.Is(err, context.DeadlineExceeded):
		return ErrorTypeContextDeadlineExceeded
	default:
		return ErrorTypeOther
	}
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
