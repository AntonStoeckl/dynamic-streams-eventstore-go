package booksincirculation_test

import (
	"context"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/addbookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/booksincirculation"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/snapshot"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

func Test_SnapshotAwareQueryHandler_Handle_SnapshotMiss(t *testing.T) {
	// Setup test environment with metrics spy
	ctx, snapshotHandler, metricsCollector, wrapper := setupSnapshotTestWithMetrics(t)

	// Create test data (1 book event)
	createTestBook(ctx, t, wrapper)

	// Reset metrics to only capture the snapshot handler behavior (no snapshot exists)
	metricsCollector.Reset()

	// Act: Query using snapshot handler (should miss snapshot and fall back to base handler)
	result, err := snapshotHandler.Handle(ctx, booksincirculation.BuildQuery())
	assert.NoError(t, err, "Snapshot handler should work")
	assert.Equal(t, 1, result.Count, "Should have 1 book")
}

func Test_SnapshotAwareQueryHandler_Handle_SnapshotCreationAndHitWithNoNewEvents(t *testing.T) {
	// Setup test environment with metrics spy
	ctx, snapshotHandler, metricsCollector, wrapper := setupSnapshotTestWithMetrics(t)

	// Create test data (1 book event)
	createTestBook(ctx, t, wrapper)

	// Reset metrics to capture the first query behavior (no snapshot exists)
	metricsCollector.Reset()

	// First query: Should miss snapshot and fall back to base handler
	// If wrapper works correctly, it should create a snapshot after a successful fallback
	query := booksincirculation.BuildQuery()

	result, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "First query should work")
	assert.Equal(t, 1, result.Count, "Should have 1 book")

	// Give async snapshot saving some time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that a snapshot was created in the database
	filter := booksincirculation.BuildEventFilter()
	savedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, query.SnapshotType(), filter)
	assert.NoError(t, err, "Should be able to load saved snapshot")
	assert.NotNil(t, savedSnapshot, "Snapshot should exist after first query")

	// Reset metrics to capture second query behavior
	metricsCollector.Reset()

	// Second query: Should hit the snapshot created by the first query (if wrapper creates snapshots automatically)
	hitResult, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "Second query should work")
	assert.Equal(t, 1, hitResult.Count, "Should have 1 book")
	assert.Equal(t, result, hitResult, "Results should be identical")
}

func Test_SnapshotAwareQueryHandler_Handle_SnapshotHitWithNewEvents(t *testing.T) {
	// Setup test environment with metrics spy
	ctx, snapshotHandler, metricsCollector, wrapper := setupSnapshotTestWithMetrics(t)

	// Create the first test book (will create sequence=1)
	createTestBook(ctx, t, wrapper)

	// First query: Should miss snapshot and fall back to base handler, then create snapshot
	query := booksincirculation.BuildQuery()

	result1, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "First query should work")
	assert.Equal(t, 1, result1.Count, "Should have 1 book initially")
	assert.Equal(t, uint(1), result1.SequenceNumber, "Should have sequence=1")

	// Give async snapshot saving some time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that a snapshot was created in the database
	filter := booksincirculation.BuildEventFilter()
	savedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, query.SnapshotType(), filter)
	assert.NoError(t, err, "Should be able to load saved snapshot")
	assert.NotNil(t, savedSnapshot, "Snapshot should exist after first query")
	assert.Equal(t, uint(1), savedSnapshot.SequenceNumber, "Snapshot should have sequence=1")

	// Add a SECOND book (this will create sequence=2)
	createSecondTestBook(ctx, t, wrapper)

	// Reset metrics to capture snapshot hit behavior with incremental events
	metricsCollector.Reset()

	// Second query: Should hit snapshot and process incremental events (new book)
	result2, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "Second query should work")
	assert.Equal(t, 2, result2.Count, "Should have 2 books after incremental processing")
	assert.Equal(t, uint(2), result2.SequenceNumber, "Should have sequence=2 after processing new events")

	// Verify we have both books (the first book should be at index 0 due to earlier timestamp)
	assert.Equal(t, result1.Books[0].BookID, result2.Books[0].BookID, "First book should still be present")
	assert.Equal(t, "Second Book", result2.Books[1].Title, "Second book should be added")

	// Wait for the async snapshot update to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that the snapshot was updated with new incremental data
	updatedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, query.SnapshotType(), filter)
	assert.NoError(t, err, "Should be able to load updated snapshot")
	assert.NotNil(t, updatedSnapshot, "Updated snapshot should exist")
	assert.Equal(t, uint(2), updatedSnapshot.SequenceNumber, "Updated snapshot should have sequence=2")

	// Deserialize and verify the updated snapshot contains incremental data (2 books)
	var updatedProjection booksincirculation.BooksInCirculation
	err = jsoniter.ConfigFastest.Unmarshal(updatedSnapshot.Data, &updatedProjection)
	assert.NoError(t, err, "Should be able to unmarshal updated snapshot data")
	assert.Equal(t, 2, updatedProjection.Count, "Updated snapshot should contain 2 books")
	assert.Equal(t, uint(2), updatedProjection.SequenceNumber, "Updated snapshot projection should have sequence=2")
}

// Helper function to set up the test environment with new QueryWrapper.
func setupSnapshotTestWithMetrics(t *testing.T) (
	context.Context,
	*snapshot.QueryWrapper[booksincirculation.Query, booksincirculation.BooksInCirculation],
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

	// Create the base handler with metrics spy
	baseHandler, err := booksincirculation.NewQueryHandler(
		wrapper.GetEventStore(),
		booksincirculation.WithMetrics(metricsCollector),
	)
	assert.NoError(t, err, "Should create base query handler with metrics")

	snapshotHandler, err := snapshot.NewQueryWrapper[
		booksincirculation.Query,
		booksincirculation.BooksInCirculation,
	](
		baseHandler,
		booksincirculation.Project,
		func(_ booksincirculation.Query) eventstore.Filter {
			return booksincirculation.BuildEventFilter()
		},
	)
	assert.NoError(t, err, "Should create snapshot-aware query handler")

	return ctx, snapshotHandler, metricsCollector, wrapper
}

// Helper function to create a test book (event data only, no queries).
func createTestBook(ctx context.Context, t *testing.T, wrapper Wrapper) {
	t.Helper()

	bookID := GivenUniqueID(t)
	fakeClock := time.Unix(0, 0).UTC()

	addBookCmd := addbookcopy.BuildCommand(bookID, "978-1-234-56789-0", "Test Book", "Test Author", "1st", "Test Publisher", 2023, fakeClock)
	addBookHandler := addbookcopy.NewCommandHandler(wrapper.GetEventStore())
	_, err := addBookHandler.Handle(ctx, addBookCmd)
	assert.NoError(t, err, "Should add book to circulation")

	// Don't query here - let the test control when queries happen
}

// Helper function to create a second test book for testing incremental snapshot updates.
func createSecondTestBook(ctx context.Context, t *testing.T, wrapper Wrapper) {
	t.Helper()

	bookID := GivenUniqueID(t)
	fakeClock := time.Unix(1, 0).UTC() // Slightly different timestamp

	addBookCmd := addbookcopy.BuildCommand(bookID, "978-2-345-67890-1", "Second Book", "Second Author", "2nd", "Second Publisher", 2024, fakeClock)
	addBookHandler := addbookcopy.NewCommandHandler(wrapper.GetEventStore())
	_, err := addBookHandler.Handle(ctx, addBookCmd)
	assert.NoError(t, err, "Should add second book to circulation")
}
