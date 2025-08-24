package shell

import "errors"

var (
	// ErrIdempotentOperation is a sentinel error to indicate an idempotent operation that should be recorded in metrics.
	ErrIdempotentOperation = errors.New("idempotent operation - no state change needed")
)
