package shell

import "time"

// HandlerResult represents the outcome of a command handler execution.
// It captures both business outcomes (idempotency) and execution metadata (retry information)
// without coupling the handler to specific observability implementations.
type HandlerResult struct {
	// Idempotent indicates whether the operation was idempotent (no state change needed).
	// This is a first-class business outcome, not an error condition.
	Idempotent bool

	// RetryAttempts is the total number of attempts made (1 for no retries, 2+ for retries).
	RetryAttempts int

	// TotalRetryDelay is the cumulative time spent in retry backoff delays.
	// This excludes the actual execution time, only counting sleep/wait periods.
	TotalRetryDelay time.Duration

	// LastErrorType describes the type of the final error encountered during retries.
	// Values: "none" (success), "concurrency_conflict", "context_canceled", "context_deadline_exceeded", "other"
	LastErrorType string

	// RetriesExhausted indicates whether max retry attempts were reached with a retryable error.
	// This is true only when all retry attempts were exhausted for retryable errors (e.g., concurrency conflicts).
	RetriesExhausted bool
}

// NewSuccessResult creates a HandlerResult for successful operations (non-idempotent).
func NewSuccessResult(retryMetrics RetryMetrics) HandlerResult {
	return HandlerResult{
		Idempotent:       false,
		RetryAttempts:    retryMetrics.Attempts,
		TotalRetryDelay:  retryMetrics.TotalDelay,
		LastErrorType:    retryMetrics.LastErrorType,
		RetriesExhausted: retryMetrics.RetriesExhausted,
	}
}

// NewIdempotentResult creates a HandlerResult for idempotent operations.
func NewIdempotentResult(retryMetrics RetryMetrics) HandlerResult {
	return HandlerResult{
		Idempotent:       true,
		RetryAttempts:    retryMetrics.Attempts,
		TotalRetryDelay:  retryMetrics.TotalDelay,
		LastErrorType:    retryMetrics.LastErrorType,
		RetriesExhausted: retryMetrics.RetriesExhausted,
	}
}

// NewErrorResult creates a HandlerResult for failed operations.
// This is used when the handler returns an error but still wants to report retry metadata.
func NewErrorResult(retryMetrics RetryMetrics) HandlerResult {
	return HandlerResult{
		Idempotent:       false,
		RetryAttempts:    retryMetrics.Attempts,
		TotalRetryDelay:  retryMetrics.TotalDelay,
		LastErrorType:    retryMetrics.LastErrorType,
		RetriesExhausted: retryMetrics.RetriesExhausted,
	}
}
