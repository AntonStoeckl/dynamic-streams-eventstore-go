package registeredreaders_test

import (
	"context"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/registerreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/registeredreaders"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/snapshot"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

func Test_SnapshotAwareQueryHandler_Handle_SnapshotMiss(t *testing.T) {
	t.Skip("TEMP: Skipping component metrics test during observable wrapper refactoring")

	// Setup test environment with metrics spy
	ctx, snapshotHandler, metricsCollector, wrapper := setupSnapshotTestWithMetrics(t)

	// Create test data (1 reader event)
	createTestReader(ctx, t, wrapper)

	// Reset metrics to only capture the snapshot handler behavior (no snapshot exists)
	metricsCollector.Reset()

	// Act: Query using snapshot handler (should miss snapshot and fall back to base handler)
	result, err := snapshotHandler.Handle(ctx, registeredreaders.BuildQuery())
	assert.NoError(t, err, "Snapshot handler should work")
	assert.Equal(t, 1, result.Count, "Should have 1 reader")

	// Assert: Should record snapshot miss metrics (snapshot_load fails, then fallback to base handler)
	assertSnapshotMissMetrics(t, metricsCollector)
}

func Test_SnapshotAwareQueryHandler_Handle_SnapshotCreationAndHitWithNoNewEvents(t *testing.T) {
	t.Skip("TEMP: Skipping component metrics test during observable wrapper refactoring")

	// Setup test environment with metrics spy
	ctx, snapshotHandler, metricsCollector, wrapper := setupSnapshotTestWithMetrics(t)

	// Create test data (1 reader event)
	createTestReader(ctx, t, wrapper)

	// Reset metrics to capture the first query behavior (no snapshot exists)
	metricsCollector.Reset()

	// First query: Should miss snapshot and fall back to base handler
	// If wrapper works correctly, it should create a snapshot after a successful fallback
	result, err := snapshotHandler.Handle(ctx, registeredreaders.BuildQuery())
	assert.NoError(t, err, "First query should work")
	assert.Equal(t, 1, result.Count, "Should have 1 reader")

	// Assert: The first query should record snapshot miss metrics
	assertSnapshotMissMetrics(t, metricsCollector)

	// Give async snapshot saving some time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that a snapshot was created in the database
	filter := registeredreaders.BuildEventFilter()
	savedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, snapshotHandler.BuildSnapshotType(registeredreaders.BuildQuery()), filter)
	assert.NoError(t, err, "Should be able to load saved snapshot")
	assert.NotNil(t, savedSnapshot, "Snapshot should exist after first query")

	// Reset metrics to capture second query behavior
	metricsCollector.Reset()

	// Second query: Should hit the snapshot created by the first query (if wrapper creates snapshots automatically)
	hitResult, err := snapshotHandler.Handle(ctx, registeredreaders.BuildQuery())
	assert.NoError(t, err, "Second query should work")
	assert.Equal(t, 1, hitResult.Count, "Should have 1 reader")
	assert.Equal(t, result, hitResult, "Results should be identical")

	// Assert: The second query should record snapshot hit metrics (if wrapper created snapshot after the first query)
	assertSnapshotHitMetrics(t, metricsCollector)
}

func Test_SnapshotAwareQueryHandler_Handle_SnapshotHitWithNewEvents(t *testing.T) {
	t.Skip("TEMP: Skipping component metrics test during observable wrapper refactoring")

	// Setup test environment with metrics spy
	ctx, snapshotHandler, metricsCollector, wrapper := setupSnapshotTestWithMetrics(t)

	// Create the first test reader (will create sequence=1)
	createTestReader(ctx, t, wrapper)

	// First query: Should miss snapshot and fall back to base handler, then create snapshot
	result1, err := snapshotHandler.Handle(ctx, registeredreaders.BuildQuery())
	assert.NoError(t, err, "First query should work")
	assert.Equal(t, 1, result1.Count, "Should have 1 reader initially")
	assert.Equal(t, uint(1), result1.SequenceNumber, "Should have sequence=1")

	// Give async snapshot saving some time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that a snapshot was created in the database
	filter := registeredreaders.BuildEventFilter()
	savedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, snapshotHandler.BuildSnapshotType(registeredreaders.BuildQuery()), filter)
	assert.NoError(t, err, "Should be able to load saved snapshot")
	assert.NotNil(t, savedSnapshot, "Snapshot should exist after first query")
	assert.Equal(t, uint(1), savedSnapshot.SequenceNumber, "Snapshot should have sequence=1")

	// Add a SECOND reader (this will create sequence=2)
	createSecondTestReader(ctx, t, wrapper)

	// Reset metrics to capture snapshot hit behavior with incremental events
	metricsCollector.Reset()

	// Second query: Should hit snapshot and process incremental events (new reader)
	result2, err := snapshotHandler.Handle(ctx, registeredreaders.BuildQuery())
	assert.NoError(t, err, "Second query should work")
	assert.Equal(t, 2, result2.Count, "Should have 2 readers after incremental processing")
	assert.Equal(t, uint(2), result2.SequenceNumber, "Should have sequence=2 after processing new events")

	// Verify we have both readers (the first reader should be at index 0 due to earlier timestamp)
	assert.Equal(t, result1.Readers[0].ReaderID, result2.Readers[0].ReaderID, "First reader should still be present")
	assert.Len(t, result2.Readers, 2, "Should have exactly 2 readers")
	// The second reader should be different from the first
	assert.NotEqual(t, result2.Readers[0].ReaderID, result2.Readers[1].ReaderID, "Second reader should be different")

	// Assert: A second query should record snapshot hit metrics with incremental processing
	assertSnapshotHitMetrics(t, metricsCollector)

	// Wait for the async snapshot update to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that the snapshot was updated with incremental changes
	filter = registeredreaders.BuildEventFilter()
	updatedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, snapshotHandler.BuildSnapshotType(registeredreaders.BuildQuery()), filter)
	assert.NoError(t, err, "Should be able to load updated snapshot")
	assert.NotNil(t, updatedSnapshot, "Updated snapshot should exist")
	assert.Equal(t, uint(2), updatedSnapshot.SequenceNumber, "Updated snapshot should have sequence=2")

	// Deserialize and verify the updated snapshot contains incremental data (2 readers)
	var updatedProjection registeredreaders.RegisteredReaders
	err = jsoniter.ConfigFastest.Unmarshal(updatedSnapshot.Data, &updatedProjection)
	assert.NoError(t, err, "Should be able to deserialize updated snapshot")
	assert.Equal(t, 2, updatedProjection.Count, "Updated snapshot should contain 2 readers")
	assert.Equal(t, uint(2), updatedProjection.SequenceNumber, "Updated snapshot projection should have sequence=2")
}

func setupSnapshotTestWithMetrics(t *testing.T) (
	context.Context,
	*snapshot.GenericSnapshotWrapper[registeredreaders.Query, registeredreaders.RegisteredReaders],
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
	baseHandler, err := registeredreaders.NewQueryHandler(
		wrapper.GetEventStore(),
		registeredreaders.WithMetrics(metricsCollector),
	)
	assert.NoError(t, err, "Should create base query handler with metrics")

	snapshotHandler, err := snapshot.NewGenericSnapshotWrapper[
		registeredreaders.Query,
		registeredreaders.RegisteredReaders,
	](
		baseHandler,
		registeredreaders.Project,
		func(_ registeredreaders.Query) eventstore.Filter {
			return registeredreaders.BuildEventFilter()
		},
	)
	assert.NoError(t, err, "Should create snapshot-aware query handler")

	return ctx, snapshotHandler, metricsCollector, wrapper
}

// Helper function to create a test reader (single RegisterReader event).
func createTestReader(ctx context.Context, t *testing.T, wrapper Wrapper) {
	t.Helper()

	readerID := GivenUniqueID(t)
	fakeClock := time.Unix(0, 0).UTC()

	// Register a reader
	registerReaderCmd := registerreader.BuildCommand(readerID, "Test Reader", fakeClock)
	readerHandler := registerreader.NewCommandHandler(wrapper.GetEventStore())
	_, err := readerHandler.Handle(ctx, registerReaderCmd)
	assert.NoError(t, err, "Should register reader")
}

// Helper function to create a second test reader for testing incremental snapshot updates.
func createSecondTestReader(ctx context.Context, t *testing.T, wrapper Wrapper) {
	t.Helper()

	readerID := GivenUniqueID(t)
	fakeClock := time.Unix(1, 0).UTC() // Slightly different timestamp

	// Register a second reader
	registerReaderCmd := registerreader.BuildCommand(readerID, "Second Reader", fakeClock)
	readerHandler := registerreader.NewCommandHandler(wrapper.GetEventStore())
	_, err := readerHandler.Handle(ctx, registerReaderCmd)
	assert.NoError(t, err, "Should register second reader")
}

// Helper function to assert snapshot miss metrics.
func assertSnapshotMissMetrics(t *testing.T, metricsCollector *MetricsCollectorSpy) {
	t.Helper()

	componentRecords := getComponentMetrics(metricsCollector)

	// We should have 2 component records: snapshot_load (error), snapshot_save (success)
	// Base handlers no longer record component metrics - they only record total timing
	assert.Len(t, componentRecords, 2, "should record exactly 2 component metrics for snapshot miss")

	// Check for expected components with the correct status
	expectedComponents := map[string]string{
		"snapshot_load": "error",   // Snapshot miss
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
		"snapshot_unmarshal":     "success", // Incremental events unmarshal (renamed from unmarshal)
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
