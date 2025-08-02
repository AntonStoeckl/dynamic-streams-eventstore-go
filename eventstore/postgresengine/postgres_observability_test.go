package postgresengine_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

func Test_Observability_Eventstore_WithLogger_LogsQueries(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testHandler := NewLogHandlerSpy(false)
	logger := slog.New(testHandler)

	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithLogger(logger))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	_, _, err := es.Query(ctxWithTimeout, filter)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, 2, testHandler.GetRecordCount(), "query should log exactly one SQL statement and one operational statement")
	assert.True(t, testHandler.HasDebugLog("executed sql for: query"), "should log with correct message")
	assert.True(t,
		testHandler.HasDebugLogWithMessage("executed sql for: query").
			WithDurationMS().
			Assert(), "should log with duration_ms attribute",
	)
	assert.True(t,
		testHandler.HasInfoLogWithMessage("eventstore operation: query completed").
			WithDurationMS().
			WithEventCount().
			Assert(), "should log query completion with duration and event count",
	)
}

func Test_Observability_Eventstore_WithLogger_LogsAppends(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testHandler := NewLogHandlerSpy(false)
	logger := slog.New(testHandler)

	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithLogger(logger))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	err := es.Append(
		ctxWithTimeout,
		filter,
		QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter),
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock.Add(time.Second))),
	)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, 4, testHandler.GetRecordCount(), "query and append should log exactly one sql statement and one operational statement each")
	assert.True(t, testHandler.HasDebugLog("executed sql for: query"), "Should log with correct message")
	assert.True(t, testHandler.HasDebugLog("executed sql for: append"), "Should log with correct message")
	assert.True(t,
		testHandler.HasDebugLogWithMessage("executed sql for: query").
			WithDurationMS().
			Assert(), "Should log query with duration_ms attribute",
	)
	assert.True(t,
		testHandler.HasDebugLogWithMessage("executed sql for: append").
			WithDurationMS().
			Assert(), "Should log append with duration_ms attribute",
	)
	assert.True(t,
		testHandler.HasInfoLogWithMessage("eventstore operation: query completed").
			WithDurationMS().
			WithEventCount().
			Assert(), "Should log query completion with duration and event count",
	)
	assert.True(t,
		testHandler.HasInfoLogWithMessage("eventstore operation: events appended").
			WithDurationMS().
			WithEventCount().
			Assert(), "Should log append completion with duration and event count",
	)
}

func Test_Observability_Eventstore_WithLogger_LogsOperations(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testHandler := NewLogHandlerSpy(false)
	logger := slog.New(testHandler)

	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithLogger(logger))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	_, _, err := es.Query(ctxWithTimeout, filter)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, 2, testHandler.GetRecordCount(), "query should log exactly one sql statement and one operational statement")
	assert.True(t,
		testHandler.HasInfoLogWithMessage("eventstore operation: query completed").
			WithDurationMS().
			WithEventCount().
			Assert(), "should log query completion with duration and event count",
	)
}

func Test_Observability_Eventstore_WithLogger_LogsAppendOperations(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testHandler := NewLogHandlerSpy(false)
	logger := slog.New(testHandler)

	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithLogger(logger))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	err := es.Append(
		ctxWithTimeout,
		filter,
		QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter),
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock.Add(time.Second))),
	)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, 4, testHandler.GetRecordCount(), "query and append should log exactly one sql statement and one operational statement each")
	assert.True(t,
		testHandler.HasInfoLogWithMessage("eventstore operation: query completed").
			WithDurationMS().
			WithEventCount().
			Assert(), "Should log query completion with duration and event count",
	)
	assert.True(t,
		testHandler.HasInfoLogWithMessage("eventstore operation: events appended").
			WithDurationMS().
			WithEventCount().
			Assert(), "Should log append completion with duration and event count",
	)
}

func Test_Observability_Eventstore_WithLogger_LogsConcurrencyConflicts(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testHandler := NewLogHandlerSpy(false)
	logger := slog.New(testHandler)

	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithLogger(logger))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// First, add an event to establish a sequence number
	err := es.Append(
		ctxWithTimeout,
		filter,
		0, // Start with sequence 0
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock)),
	)
	assert.NoError(t, err)

	// Reset test handler to only capture the conflict
	testHandler.Reset()

	// act - try to append with the wrong expected sequence number (should cause conflict)
	err = es.Append(
		ctxWithTimeout,
		filter,
		0, // Wrong sequence number - should be 1 now
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock.Add(time.Second))),
	)

	// assert
	assert.ErrorContains(t, err, eventstore.ErrConcurrencyConflict.Error())
	assert.Equal(t, 2, testHandler.GetRecordCount(), "should log exactly one sql statement and one operational statement for query")
	assert.True(t,
		testHandler.HasInfoLogWithMessage("eventstore operation: concurrency conflict detected").
			WithExpectedEvents().
			WithRowsAffected().
			WithExpectedSequence().
			Assert(), "should log concurrency conflict with expected events, rows affected, and expected sequence",
	)
}

func Test_Observability_Eventstore_WithMetrics_RecordsQueryMetrics(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	metricsCollector := NewMetricsCollectorSpy(true)
	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithMetrics(metricsCollector))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	_, _, err := es.Query(ctxWithTimeout, filter)

	// assert
	assert.NoError(t, err)
	assert.True(t, metricsCollector.HasDurationRecordForMetric("eventstore_query_duration_seconds").
		WithOperation("query").
		WithStatus("success").
		Assert(), "should record query duration metric with correct labels")
	assert.True(t, metricsCollector.HasValueRecordForMetric("eventstore_events_queried_total").
		WithOperation("query").
		WithStatus("success").
		Assert(), "should record events queried metric with correct labels")
}

func Test_Observability_Eventstore_WithMetrics_RecordsAppendMetrics(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	metricsCollector := NewMetricsCollectorSpy(true)
	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithMetrics(metricsCollector))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	err := es.Append(
		ctxWithTimeout,
		filter,
		QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter),
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock.Add(time.Second))),
	)

	// assert
	assert.NoError(t, err)
	assert.True(t, metricsCollector.HasDurationRecordForMetric("eventstore_query_duration_seconds").
		WithOperation("query").
		WithStatus("success").
		Assert(), "should record query duration metric for pre-append query")
	assert.True(t, metricsCollector.HasDurationRecordForMetric("eventstore_append_duration_seconds").
		WithOperation("append").
		WithStatus("success").
		Assert(), "should record append duration metric with correct labels")
	assert.True(t, metricsCollector.HasValueRecordForMetric("eventstore_events_queried_total").
		WithOperation("query").
		WithStatus("success").
		Assert(), "should record events queried metric for pre-append query")
	assert.True(t, metricsCollector.HasValueRecordForMetric("eventstore_events_appended_total").
		WithOperation("append").
		WithStatus("success").
		Assert(), "should record events appended metric with correct labels")
}

func Test_Observability_Eventstore_WithMetrics_RecordsConcurrencyConflicts(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	metricsCollector := NewMetricsCollectorSpy(true)
	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithMetrics(metricsCollector))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// First, add an event to establish a sequence number
	err := es.Append(
		ctxWithTimeout,
		filter,
		0, // Start with sequence 0
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock)),
	)
	assert.NoError(t, err)

	// Reset metrics collector to only capture the conflict
	metricsCollector.Reset()

	// act - try to append with the wrong expected sequence number (should cause conflict)
	err = es.Append(
		ctxWithTimeout,
		filter,
		0, // Wrong sequence number - should be 1 now
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock.Add(time.Second))),
	)

	// assert
	assert.ErrorContains(t, err, eventstore.ErrConcurrencyConflict.Error())
	assert.True(t, metricsCollector.HasCounterRecordForMetric("eventstore_concurrency_conflicts_total").
		WithOperation("append").
		WithConflictType("concurrency").
		Assert(), "should record concurrency conflict counter with correct labels")
}

func Test_Observability_Eventstore_WithMetrics_RecordsErrorMetrics(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	metricsCollector := NewMetricsCollectorSpy(true)
	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithTableName("non_existent_table_2"), postgresengine.WithMetrics(metricsCollector))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act - attempt to query the non-existent table
	_, _, err := es.Query(ctxWithTimeout, filter)

	// assert
	assert.Error(t, err)
	assert.True(t, metricsCollector.HasDurationRecordForMetric("eventstore_query_duration_seconds").
		WithOperation("query").
		WithStatus("error").
		Assert(), "should record query duration metric with error status")
	assert.True(t, metricsCollector.HasCounterRecordForMetric("eventstore_database_errors_total").
		WithOperation("query").
		WithStatus("error").
		WithErrorType("database_query").
		Assert(), "should record database error counter with correct labels")
}

func Test_Observability_Eventstore_WithTracing_RecordsQuerySpans(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tracingCollector := NewTracingCollectorSpy(true)
	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithTracing(tracingCollector))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	_, _, err := es.Query(ctxWithTimeout, filter)

	// assert
	assert.NoError(t, err)
	assert.True(t, tracingCollector.HasSpanRecordForName("eventstore.query").
		WithStatus("success").
		WithStartAttribute("operation", "query").
		Assert(), "should record query span with correct attributes and status")
}

func Test_Observability_Eventstore_WithTracing_RecordsAppendSpans(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tracingCollector := NewTracingCollectorSpy(true)
	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithTracing(tracingCollector))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	err := es.Append(
		ctxWithTimeout,
		filter,
		QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter),
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock.Add(time.Second))),
	)

	// assert
	assert.NoError(t, err)
	assert.True(t, tracingCollector.HasSpanRecordForName("eventstore.append").
		WithStatus("success").
		WithStartAttribute("operation", "append").
		WithStartAttribute("event_count", "1").
		WithStartAttribute("event_type", "BookCopyAddedToCirculation").
		Assert(), "should record append span with correct attributes and status")
}

func Test_Observability_Eventstore_WithTracing_RecordsConcurrencyConflictSpans(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tracingCollector := NewTracingCollectorSpy(true)
	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithTracing(tracingCollector))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// Append the first event successfully
	err := es.Append(
		ctxWithTimeout,
		filter,
		QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter),
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock.Add(time.Second))),
	)
	assert.NoError(t, err)

	// Reset tracing collector to only capture the conflict
	tracingCollector.Reset()

	// act - try to append with the wrong expected sequence number (should cause conflict)
	err = es.Append(
		ctxWithTimeout,
		filter,
		0, // wrong expected sequence - should be 1
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock.Add(2*time.Second))),
	)

	// assert
	assert.ErrorContains(t, err, eventstore.ErrConcurrencyConflict.Error())
	assert.True(t, tracingCollector.HasSpanRecordForName("eventstore.append").
		WithStatus("error").
		WithStartAttribute("operation", "append").
		WithEndAttribute("error_type", "concurrency_conflict").
		Assert(), "should record append span with concurrency conflict error")
}

func Test_Observability_Eventstore_WithTracing_RecordsErrorSpans(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tracingCollector := NewTracingCollectorSpy(true)
	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithTableName("non_existent_table_3"), postgresengine.WithTracing(tracingCollector))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act - attempt to query the non-existent table
	_, _, err := es.Query(ctxWithTimeout, filter)

	// assert
	assert.Error(t, err)
	assert.True(t, tracingCollector.HasSpanRecordForName("eventstore.query").
		WithStatus("error").
		WithStartAttribute("operation", "query").
		WithEndAttribute("error_type", "database_query").
		Assert(), "should record query span with database error")
}

func Test_Observability_Eventstore_WithContextualLogger_LogsQueries(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	contextualLogger := NewContextualLoggerSpy(true)
	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithContextualLogger(contextualLogger))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	_, _, err := es.Query(ctxWithTimeout, filter)

	// assert
	assert.NoError(t, err)
	assert.True(t, contextualLogger.GetTotalRecordCount() >= 2, "contextual logger should record at least 2 log entries (debug SQL and info operation)")
	assert.True(t, contextualLogger.HasDebugLog("executed sql for: query"), "should log SQL execution with correct message")
	assert.True(t, contextualLogger.HasInfoLog("eventstore operation: query completed"), "should log operation completion")
}

func Test_Observability_Eventstore_WithContextualLogger_LogsAppends(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	contextualLogger := NewContextualLoggerSpy(true)
	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithContextualLogger(contextualLogger))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	err := es.Append(
		ctxWithTimeout,
		filter,
		QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter),
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock.Add(time.Second))),
	)

	// assert
	assert.NoError(t, err)
	assert.True(t, contextualLogger.GetTotalRecordCount() >= 4, "contextual logger should record at least 4 log entries (2 for query, 2 for append)")
	assert.True(t, contextualLogger.HasDebugLog("executed sql for: query"), "should log query SQL execution")
	assert.True(t, contextualLogger.HasDebugLog("executed sql for: append"), "should log append SQL execution")
	assert.True(t, contextualLogger.HasInfoLog("eventstore operation: query completed"), "should log query completion")
	assert.True(t, contextualLogger.HasInfoLog("eventstore operation: events appended"), "should log append completion")
}

func Test_Observability_Eventstore_WithContextualLogger_LogsErrors(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	contextualLogger := NewContextualLoggerSpy(true)
	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithTableName("non_existent_table_contextual"), postgresengine.WithContextualLogger(contextualLogger))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act - attempt to query the non-existent table
	_, _, err := es.Query(ctxWithTimeout, filter)

	// assert
	assert.Error(t, err)
	assert.True(t, contextualLogger.GetTotalRecordCount() >= 1, "contextual logger should record at least 1 error log entry")
	assert.True(t, contextualLogger.HasErrorLog("database query execution failed"), "should log database error with correct message")
}

func Test_Observability_Eventstore_WithoutLogger_HandlesLogErrorGracefully(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create EventStore without a logger to test logError's nil logger branch
	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithTableName("non_existent_table_no_logger"))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act - attempt to query the non-existent table, this should trigger logError with nil logger
	_, _, err := es.Query(ctxWithTimeout, filter)

	// assert - the operation should fail but not panic due to nil logger
	assert.Error(t, err)
	// If we get here without a panic, the nil logger branch in logError worked correctly
}

func Test_Observability_Eventstore_WithLogger_LogsErrorsCorrectly(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create EventStore with a logger to test logError's configured logger branch
	testHandler := NewLogHandlerSpy(true) // Enable recording to capture error logs
	logger := slog.New(testHandler)
	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithTableName("non_existent_table_with_logger"), postgresengine.WithLogger(logger))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act - attempt to query the non-existent table, this should trigger logError with the configured logger
	_, _, err := es.Query(ctxWithTimeout, filter)

	// assert - the operation should fail, and the error should be logged
	assert.Error(t, err)
	// Now we can directly test that the error was logged at the correct level
	assert.True(t, testHandler.HasErrorLog("database query execution failed"), "should log error with correct message and ERROR level")
}

func Test_Observability_Eventstore_WithTracing_RecordsAppendErrorWithDuration(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tracingCollector := NewTracingCollectorSpy(true)
	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithTableName("non_existent_table_tracing"), postgresengine.WithTracing(tracingCollector))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)
	event := ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock))

	// act - attempt to append to a non-existent table to trigger append error with span
	// This should exercise the formatDuration method in appendTracingObserver.finishError
	err := es.Append(ctxWithTimeout, filter, 0, event)

	// assert
	assert.Error(t, err)
	assert.True(t, tracingCollector.HasSpanRecordForName("eventstore.append").
		WithStatus("error").
		Assert(), "should record append error span (which exercises formatDuration method)")
}

func Test_Observability_Eventstore_WithMetrics_FallbackToNonContextual(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use the basic metrics collector (doesn't implement ContextualMetricsCollector)
	// This will test the fallback paths in recordDurationMetricsContext, recordValueMetricsContext, etc.
	metricsCollector := NewMetricsCollectorSpy(true)
	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithTableName("non_existent_table_fallback"), postgresengine.WithMetrics(metricsCollector))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act - attempt to query non-existent table to trigger fallback metric recording
	_, _, err := es.Query(ctxWithTimeout, filter)

	// assert
	assert.Error(t, err)
	assert.False(t, metricsCollector.SupportsContextual(), "basic spy should not support contextual interface")
	// This should exercise the non-contextual fallback paths in the 80% coverage functions
	assert.True(t, metricsCollector.HasDurationRecordForMetric("eventstore_query_duration_seconds").
		WithOperation("query").
		WithStatus("error").
		Assert(), "should record query duration via fallback path")
	assert.True(t, metricsCollector.HasCounterRecordForMetric("eventstore_database_errors_total").
		WithOperation("query").
		WithStatus("error").
		Assert(), "should record error counter via fallback path")
}

func Test_Observability_Eventstore_WithContextualMetrics_UsesContextualPath(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use the contextual metrics collector to test the contextual code paths
	metricsCollector := NewContextualMetricsCollectorSpy(true)
	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithTableName("non_existent_table_contextual"), postgresengine.WithMetrics(metricsCollector))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act - attempt to query non-existent table to trigger contextual metric recording
	_, _, err := es.Query(ctxWithTimeout, filter)

	// assert
	assert.Error(t, err)
	assert.True(t, metricsCollector.SupportsContextual(), "contextual spy should support contextual interface")
	// This should exercise the contextual paths in recordDurationMetricsContext, etc.
	assert.True(t, metricsCollector.HasDurationRecordForMetric("eventstore_query_duration_seconds").
		WithOperation("query").
		WithStatus("error").
		Assert(), "should record query duration via contextual path")
	assert.True(t, metricsCollector.HasCounterRecordForMetric("eventstore_database_errors_total").
		WithOperation("query").
		WithStatus("error").
		Assert(), "should record error counter via contextual path")
}
