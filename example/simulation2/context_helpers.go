package main

import (
	"context"
	"time"
)

type contextKey string

const (
	timeoutTypeKey  contextKey = "timeout_type"
	batchContextKey contextKey = "batch_context"
)

type TimeoutType string

const (
	CommandTimeoutType TimeoutType = "command"
	BatchTimeoutType   TimeoutType = "batch"
)

// WithBatchTimeout creates a batch timeout context with metadata.
func WithBatchTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	ctx = context.WithValue(ctx, timeoutTypeKey, BatchTimeoutType)
	return ctx, cancel
}

// WithCommandTimeout creates a command timeout context with metadata.
func WithCommandTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	ctx = context.WithValue(ctx, timeoutTypeKey, CommandTimeoutType)
	// Store parent batch context for checking
	ctx = context.WithValue(ctx, batchContextKey, parent)
	return ctx, cancel
}

// GetTimeoutType retrieves the timeout type from context.
func GetTimeoutType(ctx context.Context) (TimeoutType, bool) {
	tt, ok := ctx.Value(timeoutTypeKey).(TimeoutType)
	return tt, ok
}
