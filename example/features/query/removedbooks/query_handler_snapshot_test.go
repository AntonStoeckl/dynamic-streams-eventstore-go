package removedbooks_test

import (
	"context"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/addbookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/removebookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/removedbooks"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/snapshot"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

func Test_SnapshotAwareQueryHandler_Handle_SnapshotMiss(t *testing.T) {
	// Setup test environment
	ctx, snapshotHandler, wrapper := setupSnapshotTest(t)

	// Create test data (1 removed book event)
	createTestRemovedBook(ctx, t, wrapper)

	// Act: Query using snapshot handler (should miss snapshot and fall back to base handler)
	result, err := snapshotHandler.Handle(ctx, removedbooks.BuildQuery())
	assert.NoError(t, err, "Snapshot handler should work")
	assert.Equal(t, 1, result.Count, "Should have 1 removed book")
}

func Test_SnapshotAwareQueryHandler_Handle_SnapshotCreationAndHitWithNoNewEvents(t *testing.T) {
	// Setup test environment
	ctx, snapshotHandler, wrapper := setupSnapshotTest(t)

	// Create test data (1 removed book event)
	createTestRemovedBook(ctx, t, wrapper)

	// First query: Should miss snapshot and fall back to base handler
	// If wrapper works correctly, it should create a snapshot after a successful fallback
	result, err := snapshotHandler.Handle(ctx, removedbooks.BuildQuery())
	assert.NoError(t, err, "First query should work")
	assert.Equal(t, 1, result.Count, "Should have 1 removed book")

	// Give async snapshot saving some time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that a snapshot was created in the database
	filter := removedbooks.BuildEventFilter()
	query := removedbooks.BuildQuery()
	savedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, query.SnapshotType(), filter)
	assert.NoError(t, err, "Should be able to load saved snapshot")
	assert.NotNil(t, savedSnapshot, "Snapshot should exist after first query")

	// Second query: Should hit the snapshot created by the first query (if wrapper creates snapshots automatically)
	hitResult, err := snapshotHandler.Handle(ctx, removedbooks.BuildQuery())
	assert.NoError(t, err, "Second query should work")
	assert.Equal(t, 1, hitResult.Count, "Should have 1 removed book")
	assert.Equal(t, result, hitResult, "Results should be identical")
}

func Test_SnapshotAwareQueryHandler_Handle_SnapshotHitWithNewEvents(t *testing.T) {
	// Setup test environment
	ctx, snapshotHandler, wrapper := setupSnapshotTest(t)

	// Create the first removed book (will create sequence=2, since we need to add and remove)
	createTestRemovedBook(ctx, t, wrapper)

	// First query: Should miss snapshot and fall back to base handler, then create snapshot
	result1, err := snapshotHandler.Handle(ctx, removedbooks.BuildQuery())
	assert.NoError(t, err, "First query should work")
	assert.Equal(t, 1, result1.Count, "Should have 1 removed book initially")
	assert.Equal(t, uint(2), result1.SequenceNumber, "Should have sequence=2 (add + remove)")

	// Give async snapshot saving some time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that a snapshot was created in the database
	filter := removedbooks.BuildEventFilter()
	query := removedbooks.BuildQuery()
	savedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, query.SnapshotType(), filter)
	assert.NoError(t, err, "Should be able to load saved snapshot")
	assert.NotNil(t, savedSnapshot, "Snapshot should exist after first query")
	assert.Equal(t, uint(2), savedSnapshot.SequenceNumber, "Snapshot should have sequence=2")

	// Add another removed book (this will create sequence=4, add and remove)
	createSecondTestRemovedBook(ctx, t, wrapper)

	// Second query: Should hit snapshot and process incremental events (new removed book)
	result2, err := snapshotHandler.Handle(ctx, removedbooks.BuildQuery())
	assert.NoError(t, err, "Second query should work")
	assert.Equal(t, 2, result2.Count, "Should have 2 removed books after incremental processing")
	assert.Equal(t, uint(4), result2.SequenceNumber, "Should have sequence=4 after processing new events")

	// Verify we have both removed books (the first book should be at index 0 due to earlier timestamp)
	assert.Equal(t, result1.Books[0].BookID, result2.Books[0].BookID, "First removed book should still be present")
	assert.Len(t, result2.Books, 2, "Should have exactly 2 removed books")
	// The second removed book should be different from the first
	assert.NotEqual(t, result2.Books[0].BookID, result2.Books[1].BookID, "Second removed book should be different")

	// Wait for the async snapshot update to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that the snapshot was updated with incremental changes
	filter = removedbooks.BuildEventFilter()
	updatedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, query.SnapshotType(), filter)
	assert.NoError(t, err, "Should be able to load updated snapshot")
	assert.NotNil(t, updatedSnapshot, "Updated snapshot should exist")
	assert.Equal(t, uint(4), updatedSnapshot.SequenceNumber, "Updated snapshot should have sequence=4")

	// Deserialize and verify the updated snapshot contains incremental data (2 removed books)
	var updatedProjection removedbooks.RemovedBooks
	err = jsoniter.ConfigFastest.Unmarshal(updatedSnapshot.Data, &updatedProjection)
	assert.NoError(t, err, "Should be able to deserialize updated snapshot")
	assert.Equal(t, 2, updatedProjection.Count, "Updated snapshot should contain 2 removed books")
	assert.Equal(t, uint(4), updatedProjection.SequenceNumber, "Updated snapshot projection should have sequence=4")
}

func setupSnapshotTest(t *testing.T) (
	context.Context,
	*snapshot.QueryWrapper[removedbooks.Query, removedbooks.RemovedBooks],
	Wrapper,
) {

	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	wrapper := CreateWrapperWithTestConfig(t)
	t.Cleanup(wrapper.Close)
	CleanUp(t, wrapper)

	// Create the clean base query handler
	baseHandler := removedbooks.NewQueryHandler(wrapper.GetEventStore())

	snapshotHandler, err := snapshot.NewQueryWrapper(
		baseHandler,
		wrapper.GetEventStore(),
		removedbooks.Project,
		func(_ removedbooks.Query) eventstore.Filter {
			return removedbooks.BuildEventFilter()
		},
	)
	assert.NoError(t, err, "Should create snapshot wrapper")

	return ctx, snapshotHandler, wrapper
}

// Helper function to create a removed book (add and remove events).
func createTestRemovedBook(ctx context.Context, t *testing.T, wrapper Wrapper) {
	t.Helper()

	bookID := GivenUniqueID(t)
	fakeClock := time.Unix(0, 0).UTC()

	// Add a book first
	addBookCmd := addbookcopy.BuildCommand(bookID, "Test Book", "Test Author", "ISBN123", "1st", "Test Publisher", 2020, fakeClock)
	addBookHandler := addbookcopy.NewCommandHandler(wrapper.GetEventStore())
	_, err := addBookHandler.Handle(ctx, addBookCmd)
	assert.NoError(t, err, "Should add book")

	// Then remove the book
	removeBookCmd := removebookcopy.BuildCommand(bookID, fakeClock.Add(time.Minute))
	removeBookHandler := removebookcopy.NewCommandHandler(wrapper.GetEventStore())
	_, err = removeBookHandler.Handle(ctx, removeBookCmd)
	assert.NoError(t, err, "Should remove book")
}

// Helper function to create a second removed book for testing incremental snapshot updates.
func createSecondTestRemovedBook(ctx context.Context, t *testing.T, wrapper Wrapper) {
	t.Helper()

	bookID := GivenUniqueID(t)
	fakeClock := time.Unix(1, 0).UTC() // Slightly different timestamp

	// Add a second book first
	addBookCmd := addbookcopy.BuildCommand(bookID, "Second Book", "Second Author", "ISBN456", "1st", "Second Publisher", 2021, fakeClock)
	addBookHandler := addbookcopy.NewCommandHandler(wrapper.GetEventStore())
	_, err := addBookHandler.Handle(ctx, addBookCmd)
	assert.NoError(t, err, "Should add second book")

	// Then remove the second book
	removeBookCmd := removebookcopy.BuildCommand(bookID, fakeClock.Add(time.Minute))
	removeBookHandler := removebookcopy.NewCommandHandler(wrapper.GetEventStore())
	_, err = removeBookHandler.Handle(ctx, removeBookCmd)
	assert.NoError(t, err, "Should remove second book")
}
