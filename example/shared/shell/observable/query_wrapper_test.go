package observable_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/observable"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper" //nolint:revive
)

func Test_QueryWrapper_NewQueryWrapper_Success(t *testing.T) {
	// arrange
	handler := newMockQueryHandler(mockResult{}, nil)
	metricsCollector := NewMetricsCollectorSpy(true)

	// act
	wrapper, err := observable.NewQueryWrapper[mockQuery, mockResult](
		handler,
		observable.WithQueryMetrics[mockQuery, mockResult](metricsCollector),
	)

	// assert
	assert.NoError(t, err, "Should create wrapper successfully")
	assert.NotNil(t, wrapper, "Should return wrapper instance")
}

func Test_QueryWrapper_Handle_Success(t *testing.T) {
	// arrange
	expectedResult := mockResult{Value: "test_value", SequenceNumber: 42}
	handler := newMockQueryHandler(expectedResult, nil)
	metricsCollector := NewMetricsCollectorSpy(true)
	tracingCollector := NewTracingCollectorSpy(true)
	contextualLogger := NewContextualLoggerSpy(true)

	wrapper, err := observable.NewQueryWrapper[mockQuery, mockResult](
		handler,
		observable.WithQueryMetrics[mockQuery, mockResult](metricsCollector),
		observable.WithQueryTracing[mockQuery, mockResult](tracingCollector),
		observable.WithQueryContextualLogging[mockQuery, mockResult](contextualLogger),
	)
	assert.NoError(t, err, "Should create wrapper")

	query := mockQuery{}
	ctx := context.Background()

	// act
	result, err := wrapper.Handle(ctx, query)

	// assert
	assert.NoError(t, err, "Should handle query successfully")
	assert.Equal(t, expectedResult, result, "Should return handler result")

	// Verify handler was called
	calls := handler.GetCalls()
	assert.Len(t, calls, 1, "Should call handler once")
	assert.Equal(t, query, calls[0], "Should pass query to handler")

	// Verify metrics were recorded
	assert.True(t, metricsCollector.HasCounterRecordForMetric(shell.QueryHandlerCallsMetric).
		WithLabel("query_type", "TestQuery").
		WithStatus("success").
		Assert(), "Should record success metric")
	assert.True(t, metricsCollector.HasDurationRecordForMetric(shell.QueryHandlerDurationMetric).
		WithLabel("query_type", "TestQuery").
		WithStatus("success").
		Assert(), "Should record duration metric")

	// Verify tracing
	assert.Greater(t, tracingCollector.GetSpanRecordCount(), 0, "Should create spans")

	// Verify logging
	assert.True(t, contextualLogger.HasInfoLog("query handler started"),
		"Should log query start")
	assert.True(t, contextualLogger.HasInfoLog("query handler completed"),
		"Should log query completion")
}

func Test_QueryWrapper_Handle_Error(t *testing.T) {
	// arrange
	expectedError := errors.New("test error")
	handler := newMockQueryHandler(mockResult{}, expectedError)
	metricsCollector := NewMetricsCollectorSpy(true)
	tracingCollector := NewTracingCollectorSpy(true)
	contextualLogger := NewContextualLoggerSpy(true)

	wrapper, err := observable.NewQueryWrapper[mockQuery, mockResult](
		handler,
		observable.WithQueryMetrics[mockQuery, mockResult](metricsCollector),
		observable.WithQueryTracing[mockQuery, mockResult](tracingCollector),
		observable.WithQueryContextualLogging[mockQuery, mockResult](contextualLogger),
	)
	assert.NoError(t, err, "Should create wrapper")

	query := mockQuery{}
	ctx := context.Background()

	// act
	result, err := wrapper.Handle(ctx, query)

	// assert
	assert.Error(t, err, "Should return error")
	assert.Equal(t, expectedError, err, "Should return original error")
	assert.Equal(t, mockResult{}, result, "Should return zero result on error")

	// Verify handler was called
	calls := handler.GetCalls()
	assert.Len(t, calls, 1, "Should call handler once")

	// Verify error metrics were recorded
	assert.True(t, metricsCollector.HasCounterRecordForMetric(shell.QueryHandlerCallsMetric).
		WithLabel("query_type", "TestQuery").
		WithStatus("error").
		Assert(), "Should record error metric")
	assert.True(t, metricsCollector.HasDurationRecordForMetric(shell.QueryHandlerDurationMetric).
		WithLabel("query_type", "TestQuery").
		WithStatus("error").
		Assert(), "Should record duration metric")

	// Verify tracing
	assert.Greater(t, tracingCollector.GetSpanRecordCount(), 0, "Should create spans")

	// Verify logging
	assert.True(t, contextualLogger.HasInfoLog("query handler started"),
		"Should log query start")
	assert.True(t, contextualLogger.HasErrorLog("query handler failed"),
		"Should log query error")
}

func Test_QueryWrapper_Handle_Cancellation(t *testing.T) {
	// arrange
	expectedError := context.Canceled
	handler := newMockQueryHandler(mockResult{}, expectedError)
	metricsCollector := NewMetricsCollectorSpy(true)
	tracingCollector := NewTracingCollectorSpy(true)
	contextualLogger := NewContextualLoggerSpy(true)

	wrapper, err := observable.NewQueryWrapper[mockQuery, mockResult](
		handler,
		observable.WithQueryMetrics[mockQuery, mockResult](metricsCollector),
		observable.WithQueryTracing[mockQuery, mockResult](tracingCollector),
		observable.WithQueryContextualLogging[mockQuery, mockResult](contextualLogger),
	)
	assert.NoError(t, err, "Should create wrapper")

	query := mockQuery{}
	ctx := context.Background()

	// act
	result, err := wrapper.Handle(ctx, query)

	// assert
	assert.Error(t, err, "Should return error")
	assert.Equal(t, expectedError, err, "Should return cancellation error")
	assert.Equal(t, mockResult{}, result, "Should return zero result on error")

	// Verify canceled metrics were recorded
	assert.True(t, metricsCollector.HasCounterRecordForMetric(shell.QueryHandlerCallsMetric).
		WithLabel("query_type", "TestQuery").
		WithStatus("canceled").
		Assert(), "Should record canceled metric")
	assert.True(t, metricsCollector.HasCounterRecordForMetric(shell.QueryHandlerCanceledMetric).
		WithLabel("query_type", "TestQuery").
		WithStatus("canceled").
		Assert(), "Should record canceled counter")
}

func Test_QueryWrapper_Handle_Timeout(t *testing.T) {
	// arrange
	expectedError := context.DeadlineExceeded
	handler := newMockQueryHandler(mockResult{}, expectedError)
	metricsCollector := NewMetricsCollectorSpy(true)
	tracingCollector := NewTracingCollectorSpy(true)
	contextualLogger := NewContextualLoggerSpy(true)

	wrapper, err := observable.NewQueryWrapper[mockQuery, mockResult](
		handler,
		observable.WithQueryMetrics[mockQuery, mockResult](metricsCollector),
		observable.WithQueryTracing[mockQuery, mockResult](tracingCollector),
		observable.WithQueryContextualLogging[mockQuery, mockResult](contextualLogger),
	)
	assert.NoError(t, err, "Should create wrapper")

	query := mockQuery{}
	ctx := context.Background()

	// act
	result, err := wrapper.Handle(ctx, query)

	// assert
	assert.Error(t, err, "Should return error")
	assert.Equal(t, expectedError, err, "Should return timeout error")
	assert.Equal(t, mockResult{}, result, "Should return zero result on error")

	// Verify timeout metrics were recorded
	assert.True(t, metricsCollector.HasCounterRecordForMetric(shell.QueryHandlerCallsMetric).
		WithLabel("query_type", "TestQuery").
		WithStatus("timeout").
		Assert(), "Should record timeout metric")
	assert.True(t, metricsCollector.HasCounterRecordForMetric(shell.QueryHandlerTimeoutMetric).
		WithLabel("query_type", "TestQuery").
		WithStatus("timeout").
		Assert(), "Should record timeout counter")
}

func Test_QueryWrapper_Handle_WithoutObservability(t *testing.T) {
	// arrange
	expectedResult := mockResult{Value: "test_value", SequenceNumber: 42}
	handler := newMockQueryHandler(expectedResult, nil)

	// Create a wrapper without any observability components
	wrapper, err := observable.NewQueryWrapper[mockQuery, mockResult](handler)
	assert.NoError(t, err, "Should create wrapper")

	query := mockQuery{}
	ctx := context.Background()

	// act
	result, err := wrapper.Handle(ctx, query)

	// assert
	assert.NoError(t, err, "Should handle query successfully")
	assert.Equal(t, expectedResult, result, "Should return handler result")

	// Verify handler was called
	calls := handler.GetCalls()
	assert.Len(t, calls, 1, "Should call handler once")
}

// =================================================================
// Mock implementations and helpers
// =================================================================

// mockQuery implements shell.Query for testing.
type mockQuery struct{}

func (q mockQuery) QueryType() string    { return "TestQuery" }
func (q mockQuery) SnapshotType() string { return "TestSnapshot" }

// mockResult implements shell.QueryResult for testing.
type mockResult struct {
	Value          string
	SequenceNumber uint
}

func (r mockResult) GetSequenceNumber() uint { return r.SequenceNumber }

// mockQueryHandler implements shell.QueryHandler for testing (NO Expose* methods).
type mockQueryHandler struct {
	result mockResult
	err    error
	calls  []mockQuery
}

func (h *mockQueryHandler) Handle(_ context.Context, query mockQuery) (mockResult, error) {
	h.calls = append(h.calls, query)
	return h.result, h.err
}

func (h *mockQueryHandler) GetCalls() []mockQuery {
	return h.calls
}

func (h *mockQueryHandler) Reset() {
	h.calls = nil
}

// Test helper to create a mock query handler.
func newMockQueryHandler(result mockResult, err error) *mockQueryHandler {
	return &mockQueryHandler{
		result: result,
		err:    err,
		calls:  make([]mockQuery, 0),
	}
}
