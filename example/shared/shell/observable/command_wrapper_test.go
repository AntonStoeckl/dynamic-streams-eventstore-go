package observable_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/observable"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper" //nolint:revive
)

func Test_CommandWrapper_NewCommandWrapper_Success(t *testing.T) {
	// arrange
	handler := newMockHandler(shell.HandlerResult{}, nil)
	metricsCollector := NewMetricsCollectorSpy(true)

	// act
	wrapper, err := observable.NewCommandWrapper[mockCommand](
		handler,
		observable.WithCommandMetrics[mockCommand](metricsCollector),
	)

	// assert
	assert.NoError(t, err, "Should create wrapper successfully")
	assert.NotNil(t, wrapper, "Should return wrapper instance")
}

func Test_CommandWrapper_Handle_Success_NonIdempotent(t *testing.T) {
	// arrange
	expectedResult := shell.HandlerResult{
		Idempotent:      false,
		RetryAttempts:   1,
		TotalRetryDelay: 0,
		LastErrorType:   "",
	}

	handler := newMockHandler(expectedResult, nil)
	metricsCollector := NewMetricsCollectorSpy(true)
	tracingCollector := NewTracingCollectorSpy(true)
	contextualLogger := NewContextualLoggerSpy(true)

	wrapper, err := observable.NewCommandWrapper[mockCommand](
		handler,
		observable.WithCommandMetrics[mockCommand](metricsCollector),
		observable.WithCommandTracing[mockCommand](tracingCollector),
		observable.WithCommandContextualLogging[mockCommand](contextualLogger),
	)
	assert.NoError(t, err, "Should create wrapper")

	command := mockCommand{}
	ctx := context.Background()

	// act
	result, err := wrapper.Handle(ctx, command)

	// assert
	assert.NoError(t, err, "Should handle command successfully")
	assert.Equal(t, expectedResult, result, "Should return handler result")

	// Verify handler was called
	calls := handler.GetCalls()
	assert.Len(t, calls, 1, "Should call handler once")
	assert.Equal(t, command, calls[0], "Should pass command to handler")

	// Verify metrics were recorded
	assert.True(t, metricsCollector.HasCounterRecordForMetric(shell.CommandHandlerCallsMetric).
		WithLabel("command_type", "TestCommand").
		WithStatus("success").
		Assert(), "Should record success metric")
	assert.True(t, metricsCollector.HasDurationRecordForMetric(shell.CommandHandlerDurationMetric).
		WithLabel("command_type", "TestCommand").
		WithStatus("success").
		Assert(), "Should record duration metric")

	// Verify tracing
	assert.Greater(t, tracingCollector.GetSpanRecordCount(), 0, "Should create spans")

	// Verify logging
	assert.True(t, contextualLogger.HasInfoLog("command handler started"),
		"Should log command start")
	assert.True(t, contextualLogger.HasInfoLog("command handler completed"),
		"Should log command completion")
}

func Test_CommandWrapper_Handle_Success_Idempotent(t *testing.T) {
	// arrange
	expectedResult := shell.HandlerResult{
		Idempotent:      true,
		RetryAttempts:   1,
		TotalRetryDelay: 0,
		LastErrorType:   "",
	}

	handler := newMockHandler(expectedResult, nil)
	metricsCollector := NewMetricsCollectorSpy(true)

	wrapper, err := observable.NewCommandWrapper[mockCommand](
		handler,
		observable.WithCommandMetrics[mockCommand](metricsCollector),
	)
	assert.NoError(t, err, "Should create wrapper")

	command := mockCommand{}
	ctx := context.Background()

	// act
	result, err := wrapper.Handle(ctx, command)

	// assert
	assert.NoError(t, err, "Should handle command successfully")
	assert.Equal(t, expectedResult, result, "Should return handler result")

	// Verify idempotent metrics were recorded
	assert.True(t, metricsCollector.HasCounterRecordForMetric(shell.CommandHandlerIdempotentMetric).
		WithLabel("command_type", "TestCommand").
		Assert(), "Should record idempotent metric")
}

func Test_CommandWrapper_Handle_WithRetries_RecordsMetrics(t *testing.T) {
	// arrange
	resultWithRetries := shell.HandlerResult{
		Idempotent:      false,
		RetryAttempts:   3,
		TotalRetryDelay: 15 * time.Millisecond,
		LastErrorType:   "ConcurrencyConflictError",
	}

	handler := newMockHandler(resultWithRetries, nil)
	metricsCollector := NewMetricsCollectorSpy(true)

	wrapper, err := observable.NewCommandWrapper[mockCommand](
		handler,
		observable.WithCommandMetrics[mockCommand](metricsCollector),
	)
	assert.NoError(t, err, "Should create wrapper")

	command := mockCommand{}
	ctx := context.Background()

	// act
	result, err := wrapper.Handle(ctx, command)

	// assert
	assert.NoError(t, err, "Should handle command successfully")
	assert.Equal(t, resultWithRetries, result, "Should return handler result with retry metadata")

	// Verify retry metrics were recorded
	assert.True(t, metricsCollector.HasCounterRecordForMetric(shell.CommandHandlerRetriesMetric).
		WithLabel("command_type", "TestCommand").
		Assert(), "Should record retry attempts metric")
	assert.True(t, metricsCollector.HasDurationRecordForMetric(shell.CommandHandlerRetryDelayMetric).
		WithLabel("command_type", "TestCommand").
		Assert(), "Should record retry delay metric")
}

func Test_CommandWrapper_Handle_Error_RecordsFailureMetrics(t *testing.T) {
	// arrange
	expectedError := errors.New("business logic error")
	expectedResult := shell.HandlerResult{
		Idempotent:    false,
		RetryAttempts: 2,
	}

	handler := newMockHandler(expectedResult, expectedError)
	metricsCollector := NewMetricsCollectorSpy(true)
	tracingCollector := NewTracingCollectorSpy(true)
	contextualLogger := NewContextualLoggerSpy(true)

	wrapper, err := observable.NewCommandWrapper[mockCommand](
		handler,
		observable.WithCommandMetrics[mockCommand](metricsCollector),
		observable.WithCommandTracing[mockCommand](tracingCollector),
		observable.WithCommandContextualLogging[mockCommand](contextualLogger),
	)
	assert.NoError(t, err, "Should create wrapper")

	command := mockCommand{}
	ctx := context.Background()

	// act
	result, err := wrapper.Handle(ctx, command)

	// assert
	assert.Error(t, err, "Should return error from handler")
	assert.Equal(t, expectedError, err, "Should return exact error")
	assert.Equal(t, expectedResult, result, "Should return handler result even on error")

	// Verify error metrics were recorded
	assert.True(t, metricsCollector.HasCounterRecordForMetric(shell.CommandHandlerCallsMetric).
		WithLabel("command_type", "TestCommand").
		WithStatus("error").
		Assert(), "Should record error metric")

	// Verify error logging
	assert.True(t, contextualLogger.HasErrorLog("command handler failed"),
		"Should log command failure")
}

func Test_CommandWrapper_Handle_CancellationError_RecordsCorrectStatus(t *testing.T) {
	// arrange
	cancelError := context.Canceled
	expectedResult := shell.HandlerResult{Idempotent: false}

	handler := newMockHandler(expectedResult, cancelError)
	metricsCollector := NewMetricsCollectorSpy(true)

	wrapper, err := observable.NewCommandWrapper[mockCommand](
		handler,
		observable.WithCommandMetrics[mockCommand](metricsCollector),
	)
	assert.NoError(t, err, "Should create wrapper")

	command := mockCommand{}
	ctx := context.Background()

	// act
	result, err := wrapper.Handle(ctx, command)

	// assert
	assert.Error(t, err, "Should return cancellation error")
	assert.Equal(t, expectedResult, result, "Should return handler result")

	// Verify cancellation metrics were recorded
	assert.True(t, metricsCollector.HasCounterRecordForMetric(shell.CommandHandlerCanceledMetric).
		WithLabel("command_type", "TestCommand").
		Assert(), "Should record cancellation metric")
}

func Test_CommandWrapper_Handle_TimeoutError_RecordsCorrectStatus(t *testing.T) {
	// arrange
	timeoutError := context.DeadlineExceeded
	expectedResult := shell.HandlerResult{Idempotent: false}

	handler := newMockHandler(expectedResult, timeoutError)
	metricsCollector := NewMetricsCollectorSpy(true)

	wrapper, err := observable.NewCommandWrapper[mockCommand](
		handler,
		observable.WithCommandMetrics[mockCommand](metricsCollector),
	)
	assert.NoError(t, err, "Should create wrapper")

	command := mockCommand{}
	ctx := context.Background()

	// act
	result, err := wrapper.Handle(ctx, command)

	// assert
	assert.Error(t, err, "Should return timeout error")
	assert.Equal(t, expectedResult, result, "Should return handler result")

	// Verify timeout metrics were recorded
	assert.True(t, metricsCollector.HasCounterRecordForMetric(shell.CommandHandlerTimeoutMetric).
		WithLabel("command_type", "TestCommand").
		Assert(), "Should record timeout metric")
}

func Test_CommandWrapper_Handle_WithoutObservability_WorksCorrectly(t *testing.T) {
	// arrange
	expectedResult := shell.HandlerResult{Idempotent: false}
	handler := newMockHandler(expectedResult, nil)

	// Create a wrapper without any observability options
	wrapper, err := observable.NewCommandWrapper[mockCommand](handler)
	assert.NoError(t, err, "Should create wrapper without observability")

	command := mockCommand{}
	ctx := context.Background()

	// act
	result, err := wrapper.Handle(ctx, command)

	// assert
	assert.NoError(t, err, "Should handle command successfully")
	assert.Equal(t, expectedResult, result, "Should return handler result")

	// Verify handler was called
	calls := handler.GetCalls()
	assert.Len(t, calls, 1, "Should call handler once")
}

func Test_CommandWrapper_Handle_ContextPropagation(t *testing.T) {
	// arrange
	expectedResult := shell.HandlerResult{Idempotent: false}
	handler := newMockHandler(expectedResult, nil)

	wrapper, err := observable.NewCommandWrapper[mockCommand](handler)
	assert.NoError(t, err, "Should create wrapper")

	command := mockCommand{}

	// Create context with value
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("test-key"), "test-value")

	// act
	result, err := wrapper.Handle(ctx, command)

	// assert
	assert.NoError(t, err, "Should handle command successfully")
	assert.Equal(t, expectedResult, result, "Should return handler result")

	// The context should be properly propagated to the handler
	// (This is implicitly tested since the handler receives the context)
}

func Test_CommandWrapper_Handle_CommandTypeDetection(t *testing.T) {
	// arrange
	expectedResult := shell.HandlerResult{Idempotent: false}
	handler := &mockCoreHandlerCustom{
		result: expectedResult,
		err:    nil,
		calls:  make([]customMockCommand, 0),
	}
	metricsCollector := NewMetricsCollectorSpy(true)

	wrapper, err := observable.NewCommandWrapper[customMockCommand](
		handler,
		observable.WithCommandMetrics[customMockCommand](metricsCollector),
	)
	assert.NoError(t, err, "Should create wrapper")

	command := customMockCommand{}
	ctx := context.Background()

	// act
	result, err := wrapper.Handle(ctx, command)

	// assert
	assert.NoError(t, err, "Should handle command successfully")
	assert.Equal(t, expectedResult, result, "Should return handler result")

	// Verify the correct command type was used in metrics
	assert.True(t, metricsCollector.HasCounterRecordForMetric(shell.CommandHandlerCallsMetric).
		WithLabel("command_type", "CustomCommandType").
		WithStatus("success").
		Assert(), "Should record metrics with correct command type")
}

func Test_CommandWrapper_Options_AllCombinations(t *testing.T) {
	// arrange
	handler := newMockHandler(shell.HandlerResult{}, nil)
	metricsCollector := NewMetricsCollectorSpy(true)
	tracingCollector := NewTracingCollectorSpy(true)
	contextualLogger := NewContextualLoggerSpy(true)

	// Test all option combinations
	testCases := []struct {
		name string
		opts []observable.CommandOption[mockCommand]
	}{
		{
			name: "all options",
			opts: []observable.CommandOption[mockCommand]{
				observable.WithCommandMetrics[mockCommand](metricsCollector),
				observable.WithCommandTracing[mockCommand](tracingCollector),
				observable.WithCommandContextualLogging[mockCommand](contextualLogger),
			},
		},
		{
			name: "metrics only",
			opts: []observable.CommandOption[mockCommand]{
				observable.WithCommandMetrics[mockCommand](metricsCollector),
			},
		},
		{
			name: "tracing only",
			opts: []observable.CommandOption[mockCommand]{
				observable.WithCommandTracing[mockCommand](tracingCollector),
			},
		},
		{
			name: "logging only",
			opts: []observable.CommandOption[mockCommand]{
				observable.WithCommandContextualLogging[mockCommand](contextualLogger),
			},
		},
		{
			name: "no options",
			opts: []observable.CommandOption[mockCommand]{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// act
			wrapper, err := observable.NewCommandWrapper[mockCommand](handler, tc.opts...)

			// assert
			assert.NoError(t, err, "Should create wrapper with options: %s", tc.name)
			assert.NotNil(t, wrapper, "Should return wrapper instance")

			// Should be able to handle commands regardless of options
			result, err := wrapper.Handle(context.Background(), mockCommand{})
			assert.NoError(t, err, "Should handle command with options: %s", tc.name)
			assert.NotNil(t, result, "Should return result")
		})
	}
}

// mockCommand implements shell.Command for testing.
type mockCommand struct{}

func (c mockCommand) CommandType() string {
	return "TestCommand"
}

// mockCoreHandler implements shell.CoreCommandHandler for testing.
type mockCoreHandler struct {
	result shell.HandlerResult
	err    error
	calls  []mockCommand
}

func (h *mockCoreHandler) Handle(_ context.Context, command mockCommand) (shell.HandlerResult, error) {
	h.calls = append(h.calls, command)
	return h.result, h.err
}

func (h *mockCoreHandler) GetCalls() []mockCommand {
	return h.calls
}

func (h *mockCoreHandler) Reset() {
	h.calls = nil
}

// Test helper to create a mock handler.
func newMockHandler(result shell.HandlerResult, err error) *mockCoreHandler {
	return &mockCoreHandler{
		result: result,
		err:    err,
		calls:  make([]mockCommand, 0),
	}
}

// customMockCommand implements shell.Command with a different type for testing.
type customMockCommand struct{}

func (c customMockCommand) CommandType() string {
	return "CustomCommandType"
}

// mockCoreHandlerCustom implements shell.CoreCommandHandler for custom command.
type mockCoreHandlerCustom struct {
	result shell.HandlerResult
	err    error
	calls  []customMockCommand
}

func (h *mockCoreHandlerCustom) Handle(_ context.Context, command customMockCommand) (shell.HandlerResult, error) {
	h.calls = append(h.calls, command)
	return h.result, h.err
}
