package shell

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

func Test_RetryWithExponentialBackoff_Success_NoRetries(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	fn := func(_ context.Context) error {
		callCount++
		return nil // Success on the first attempt
	}

	meta, err := RetryWithExponentialBackoff(ctx, fn)

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
	assert.Equal(t, 1, meta.Attempts)
	assert.Equal(t, time.Duration(0), meta.TotalDelay)
	assert.Equal(t, "none", meta.LastErrorType)
}

func Test_RetryWithExponentialBackoff_RetryOnConcurrencyConflict(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	fn := func(_ context.Context) error {
		callCount++
		if callCount < 3 {
			return eventstore.ErrConcurrencyConflict // Fail twice
		}
		return nil // Success on the third attempt
	}

	meta, err := RetryWithExponentialBackoff(ctx, fn)

	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
	assert.Equal(t, 3, meta.Attempts)
	assert.Greater(t, meta.TotalDelay, time.Duration(0))
	assert.Equal(t, "none", meta.LastErrorType)
}

func Test_RetryWithExponentialBackoff_WithAllOptions(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	fn := func(_ context.Context) error {
		callCount++
		if callCount < 2 {
			return eventstore.ErrConcurrencyConflict
		}
		return nil
	}

	meta, err := RetryWithExponentialBackoff(ctx, fn,
		WithMaxAttempts(3),
		WithBaseDelay(5*time.Millisecond),
		WithJitterFactor(0.1),
	)

	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)
	assert.Equal(t, 2, meta.Attempts)
	assert.Greater(t, meta.TotalDelay, time.Duration(0))
	assert.Equal(t, "none", meta.LastErrorType)
}

func Test_RetryWithExponentialBackoff_InvalidOptions(t *testing.T) {
	ctx := context.Background()
	fn := func(_ context.Context) error { return nil }

	// Test invalid max attempts
	_, err := RetryWithExponentialBackoff(ctx, fn, WithMaxAttempts(0))
	assert.ErrorIs(t, err, ErrInvalidMaxAttempts)

	// Test negative base delay
	_, err = RetryWithExponentialBackoff(ctx, fn, WithBaseDelay(-1*time.Second))
	assert.ErrorIs(t, err, ErrNegativeBaseDelay)

	// Test invalid jitter factor
	_, err = RetryWithExponentialBackoff(ctx, fn, WithJitterFactor(1.5))
	assert.ErrorIs(t, err, ErrInvalidJitterFactor)
}
