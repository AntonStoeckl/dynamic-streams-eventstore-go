package finishedlendings_test

import (
	"context"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/addbookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/lendbookcopytoreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/registerreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/returnbookcopyfromreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/finishedlendings"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/snapshot"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

func Test_SnapshotAwareQueryHandler_Handle_SnapshotMiss(t *testing.T) {
	// Setup test environment with metrics spy
	ctx, snapshotHandler, metricsCollector, wrapper := setupSnapshotTestWithMetrics(t)

	// Create test data (1 finished lending)
	createTestFinishedLending(ctx, t, wrapper)

	// Reset metrics to only capture the snapshot handler behavior (no snapshot exists)
	metricsCollector.Reset()

	// Act: Query using snapshot handler (should miss snapshot and fall back to base handler)
	result, err := snapshotHandler.Handle(ctx, finishedlendings.BuildQuery())
	assert.NoError(t, err, "Snapshot handler should work")
	assert.Equal(t, 1, result.Count, "Should have 1 finished lending")

	// Assert: Should record snapshot miss metrics (snapshot_load fails, then fallback to base handler)
	assertSnapshotMissMetrics(t, metricsCollector)
}

func Test_SnapshotAwareQueryHandler_Handle_SnapshotCreationAndHitWithNoNewEvents(t *testing.T) {
	// Setup test environment with metrics spy
	ctx, snapshotHandler, metricsCollector, wrapper := setupSnapshotTestWithMetrics(t)

	// Create test data (1 finished lending)
	createTestFinishedLending(ctx, t, wrapper)

	// Reset metrics to capture the first query behavior (no snapshot exists)
	metricsCollector.Reset()

	// First query: Should miss snapshot and fall back to base handler
	// If wrapper works correctly, it should create a snapshot after a successful fallback
	result, err := snapshotHandler.Handle(ctx, finishedlendings.BuildQuery())
	assert.NoError(t, err, "First query should work")
	assert.Equal(t, 1, result.Count, "Should have 1 finished lending")

	// Assert: The first query should record snapshot miss metrics
	assertSnapshotMissMetrics(t, metricsCollector)

	// Give async snapshot saving some time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that a snapshot was created in the database
	filter := finishedlendings.BuildEventFilter()
	savedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, snapshotHandler.BuildSnapshotType(finishedlendings.BuildQuery()), filter)
	assert.NoError(t, err, "Should be able to load saved snapshot")
	assert.NotNil(t, savedSnapshot, "Snapshot should exist after first query")

	// Reset metrics to capture second query behavior
	metricsCollector.Reset()

	// Second query: Should hit the snapshot created by the first query (if wrapper creates snapshots automatically)
	hitResult, err := snapshotHandler.Handle(ctx, finishedlendings.BuildQuery())
	assert.NoError(t, err, "Second query should work")
	assert.Equal(t, 1, hitResult.Count, "Should have 1 finished lending")
	assert.Equal(t, result, hitResult, "Results should be identical")

	// Assert: The second query should record snapshot hit metrics (if wrapper created snapshot after the first query)
	assertSnapshotHitMetrics(t, metricsCollector)
}

func Test_SnapshotAwareQueryHandler_Handle_SnapshotHitWithNewEvents(t *testing.T) {
	// Setup test environment with metrics spy
	ctx, snapshotHandler, metricsCollector, wrapper := setupSnapshotTestWithMetrics(t)

	// Create the first finished lending (will create sequence=4: add, register, lend, return)
	createTestFinishedLending(ctx, t, wrapper)

	// First query: Should miss snapshot and fall back to base handler, then create snapshot
	result1, err := snapshotHandler.Handle(ctx, finishedlendings.BuildQuery())
	assert.NoError(t, err, "First query should work")
	assert.Equal(t, 1, result1.Count, "Should have 1 finished lending initially")
	assert.Equal(t, uint(4), result1.SequenceNumber, "Should have sequence=4 (add + register + lend + return)")

	// Give async snapshot saving some time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that a snapshot was created in the database
	filter := finishedlendings.BuildEventFilter()
	savedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, snapshotHandler.BuildSnapshotType(finishedlendings.BuildQuery()), filter)
	assert.NoError(t, err, "Should be able to load saved snapshot")
	assert.NotNil(t, savedSnapshot, "Snapshot should exist after first query")
	assert.Equal(t, uint(4), savedSnapshot.SequenceNumber, "Snapshot should have sequence=4")

	// Add a SECOND finished lending (this will create sequence=8: add + register + lend + return)
	createSecondTestFinishedLending(ctx, t, wrapper)

	// Reset metrics to capture snapshot hit behavior with incremental events
	metricsCollector.Reset()

	// Second query: Should hit snapshot and process incremental events (new finished lending)
	result2, err := snapshotHandler.Handle(ctx, finishedlendings.BuildQuery())
	assert.NoError(t, err, "Second query should work")
	assert.Equal(t, 2, result2.Count, "Should have 2 finished lendings after incremental processing")
	assert.Equal(t, uint(8), result2.SequenceNumber, "Should have sequence=8 after processing new events")

	// Verify we have both finished lendings (the first lending should be at index 0 due to earlier timestamp)
	assert.Equal(t, result1.Lendings[0].BookID, result2.Lendings[0].BookID, "First finished lending should still be present")
	assert.Equal(t, result1.Lendings[0].ReaderID, result2.Lendings[0].ReaderID, "First finished lending should still be present")
	assert.Len(t, result2.Lendings, 2, "Should have exactly 2 finished lendings")
	// The second finished lending should be different from the first
	assert.NotEqual(t, result2.Lendings[0].BookID+"-"+result2.Lendings[0].ReaderID, result2.Lendings[1].BookID+"-"+result2.Lendings[1].ReaderID, "Second finished lending should be different")

	// Assert: A second query should record snapshot hit metrics with incremental processing
	assertSnapshotHitMetrics(t, metricsCollector)

	// Wait for the async snapshot update to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that the snapshot was updated with incremental changes
	filter = finishedlendings.BuildEventFilter()
	updatedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, snapshotHandler.BuildSnapshotType(finishedlendings.BuildQuery()), filter)
	assert.NoError(t, err, "Should be able to load updated snapshot")
	assert.NotNil(t, updatedSnapshot, "Updated snapshot should exist")
	assert.Equal(t, uint(8), updatedSnapshot.SequenceNumber, "Updated snapshot should have sequence=8")

	// Deserialize and verify the updated snapshot contains incremental data (2 finished lendings)
	var updatedProjection finishedlendings.FinishedLendings
	err = jsoniter.ConfigFastest.Unmarshal(updatedSnapshot.Data, &updatedProjection)
	assert.NoError(t, err, "Should be able to deserialize updated snapshot")
	assert.Equal(t, 2, updatedProjection.Count, "Updated snapshot should contain 2 finished lendings")
	assert.Equal(t, uint(8), updatedProjection.SequenceNumber, "Updated snapshot projection should have sequence=8")
}

func setupSnapshotTestWithMetrics(t *testing.T) (
	context.Context,
	*snapshot.GenericSnapshotWrapper[finishedlendings.Query, finishedlendings.FinishedLendings],
	*MetricsCollectorSpy,
	Wrapper,
) {

	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	metricsCollector := NewMetricsCollectorSpy(true)

	wrapper := CreateWrapperWithTestConfig(t)
	t.Cleanup(wrapper.Close)
	CleanUp(t, wrapper)

	// Create the base query handler with metrics
	baseHandler, err := finishedlendings.NewQueryHandler(
		wrapper.GetEventStore(),
		finishedlendings.WithMetrics(metricsCollector),
	)
	assert.NoError(t, err, "Should create base query handler with metrics")

	snapshotHandler, err := snapshot.NewGenericSnapshotWrapper[
		finishedlendings.Query,
		finishedlendings.FinishedLendings,
	](
		baseHandler,
		finishedlendings.Project,
		func(_ finishedlendings.Query) eventstore.Filter {
			return finishedlendings.BuildEventFilter()
		},
		func(queryType string, _ finishedlendings.Query) string {
			return queryType
		},
	)
	assert.NoError(t, err, "Should create snapshot-aware query handler")

	return ctx, snapshotHandler, metricsCollector, wrapper
}

// Helper function to create a finished lending (add book and register reader + lend + return).
func createTestFinishedLending(ctx context.Context, t *testing.T, wrapper Wrapper) {
	t.Helper()

	bookID := GivenUniqueID(t)
	readerID := GivenUniqueID(t)
	fakeClock := time.Unix(0, 0).UTC()

	// Add a book first
	addBookCmd := addbookcopy.BuildCommand(bookID, "Test Book", "Test Author", "ISBN123", "1st", "Test Publisher", 2020, fakeClock)
	addBookHandler, _ := addbookcopy.NewCommandHandler(wrapper.GetEventStore())
	err := addBookHandler.Handle(ctx, addBookCmd)
	assert.NoError(t, err, "Should add book")

	// Register a reader
	registerReaderCmd := registerreader.BuildCommand(readerID, "Test Reader", fakeClock.Add(time.Minute))
	registerReaderHandler, _ := registerreader.NewCommandHandler(wrapper.GetEventStore())
	err = registerReaderHandler.Handle(ctx, registerReaderCmd)
	assert.NoError(t, err, "Should register reader")

	// Lend the book to the reader
	lendBookCmd := lendbookcopytoreader.BuildCommand(bookID, readerID, fakeClock.Add(2*time.Minute))
	lendBookHandler, _ := lendbookcopytoreader.NewCommandHandler(wrapper.GetEventStore())
	err = lendBookHandler.Handle(ctx, lendBookCmd)
	assert.NoError(t, err, "Should lend book")

	// Return the book from the reader
	returnBookCmd := returnbookcopyfromreader.BuildCommand(bookID, readerID, fakeClock.Add(3*time.Minute))
	returnBookHandler, _ := returnbookcopyfromreader.NewCommandHandler(wrapper.GetEventStore())
	err = returnBookHandler.Handle(ctx, returnBookCmd)
	assert.NoError(t, err, "Should return book")
}

// Helper function to create a second finished lending for testing incremental snapshot updates.
func createSecondTestFinishedLending(ctx context.Context, t *testing.T, wrapper Wrapper) {
	t.Helper()

	bookID := GivenUniqueID(t)
	readerID := GivenUniqueID(t)
	fakeClock := time.Unix(1, 0).UTC() // Slightly different timestamp

	// Add a second book first
	addBookCmd := addbookcopy.BuildCommand(bookID, "Second Book", "Second Author", "ISBN456", "1st", "Second Publisher", 2021, fakeClock)
	addBookHandler, _ := addbookcopy.NewCommandHandler(wrapper.GetEventStore())
	err := addBookHandler.Handle(ctx, addBookCmd)
	assert.NoError(t, err, "Should add second book")

	// Register a second reader
	registerReaderCmd := registerreader.BuildCommand(readerID, "Second Reader", fakeClock.Add(time.Minute))
	registerReaderHandler, _ := registerreader.NewCommandHandler(wrapper.GetEventStore())
	err = registerReaderHandler.Handle(ctx, registerReaderCmd)
	assert.NoError(t, err, "Should register second reader")

	// Lend the second book to the second reader
	lendBookCmd := lendbookcopytoreader.BuildCommand(bookID, readerID, fakeClock.Add(2*time.Minute))
	lendBookHandler, _ := lendbookcopytoreader.NewCommandHandler(wrapper.GetEventStore())
	err = lendBookHandler.Handle(ctx, lendBookCmd)
	assert.NoError(t, err, "Should lend second book")

	// Return the second book from the second reader
	returnBookCmd := returnbookcopyfromreader.BuildCommand(bookID, readerID, fakeClock.Add(3*time.Minute))
	returnBookHandler, _ := returnbookcopyfromreader.NewCommandHandler(wrapper.GetEventStore())
	err = returnBookHandler.Handle(ctx, returnBookCmd)
	assert.NoError(t, err, "Should return second book")
}

// Helper function to assert snapshot miss metrics.
func assertSnapshotMissMetrics(t *testing.T, metricsCollector *MetricsCollectorSpy) {
	t.Helper()

	componentRecords := getComponentMetrics(metricsCollector)

	// We should have 5 component records: snapshot_load (error), query (success), unmarshal (success), projection (success), snapshot_save (success)
	assert.Len(t, componentRecords, 5, "should record exactly 5 component metrics for snapshot miss")

	// Check for expected components with the correct status
	expectedComponents := map[string]string{
		"snapshot_load": "error",   // Snapshot miss
		"query":         "success", // Fallback to base handler
		"unmarshal":     "success", // Fallback to base handler
		"projection":    "success", // Fallback to base handler
		"snapshot_save": "success", // Save the initial snapshot after fallback
	}

	assertComponentMetrics(t, componentRecords, expectedComponents)
}

// Helper function to assert snapshot hit metrics.
func assertSnapshotHitMetrics(t *testing.T, metricsCollector *MetricsCollectorSpy) {
	t.Helper()

	componentRecords := getComponentMetrics(metricsCollector)

	// We should have 6 snapshot hit parts: all snapshot operations succeed, including snapshot save
	assert.Len(t, componentRecords, 6, "should record exactly 6 component metrics for snapshot hit")

	// Check for snapshot hit components with success status
	expectedComponents := map[string]string{
		"snapshot_load":          "success", // Snapshot hit
		"incremental_query":      "success", // Incremental query execution
		"unmarshal":              "success", // Incremental events unmarshal
		"snapshot_deserialize":   "success", // Snapshot data deserialization
		"incremental_projection": "success", // Incremental projection
		"snapshot_save":          "success", // Save the updated snapshot with incremental changes
	}

	assertComponentMetrics(t, componentRecords, expectedComponents)
}

// Helper function to extract component metrics from spy records.
func getComponentMetrics(metricsCollector *MetricsCollectorSpy) []SpyDurationRecord {
	durationRecords := metricsCollector.GetDurationRecords()
	componentRecords := make([]SpyDurationRecord, 0)
	for _, record := range durationRecords {
		if record.Metric == "queryhandler_component_duration_seconds" {
			componentRecords = append(componentRecords, record)
		}
	}
	return componentRecords
}

// Helper function to assert component metrics match expected components and statuses.
func assertComponentMetrics(t *testing.T, componentRecords []SpyDurationRecord, expectedComponents map[string]string) {
	t.Helper()

	foundComponents := make(map[string]bool)

	for _, record := range componentRecords {
		component := record.Labels["component"]
		status := record.Labels["status"]

		expectedStatus, exists := expectedComponents[component]
		if !exists {
			t.Errorf("Unexpected component: %s", component)
			continue
		}

		assert.Equal(t, expectedStatus, status, "component %s should have status %s", component, expectedStatus)
		foundComponents[component] = true
	}

	// Verify all expected components were found
	for component := range expectedComponents {
		assert.True(t, foundComponents[component], "should record %s component", component)
	}
}
