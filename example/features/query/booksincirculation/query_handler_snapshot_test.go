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
	// setup
	ctx, snapshotHandler, wrapper := setupSnapshotTest(t)

	// arrange
	createTestBook(ctx, t, wrapper)

	// act
	result, err := snapshotHandler.Handle(ctx, booksincirculation.BuildQuery())
	assert.NoError(t, err, "Snapshot handler should work")
	assert.Equal(t, 1, result.Count, "Should have 1 book")
}

func Test_SnapshotAwareQueryHandler_Handle_SnapshotCreationAndHitWithNoNewEvents(t *testing.T) {
	// setup
	ctx, snapshotHandler, wrapper := setupSnapshotTest(t)

	// arrange
	createTestBook(ctx, t, wrapper)

	// act / assert

	// First query: Should miss snapshot and fall back to base handler
	// If wrapper works correctly, it should create a snapshot after a successful fallback
	query := booksincirculation.BuildQuery()

	result, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "First query should work")
	assert.Equal(t, 1, result.Count, "Should have 1 book")

	// Give snapshot saving some time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that a snapshot was created in the database
	filter := booksincirculation.BuildEventFilter()
	savedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, query.SnapshotType(), filter)
	assert.NoError(t, err, "Should be able to load saved snapshot")
	assert.NotNil(t, savedSnapshot, "Snapshot should exist after first query")

	// Second query: Should hit snapshot (no new events since last query)
	result2, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "Second query should work")
	assert.Equal(t, 1, result2.Count, "Should still have 1 book")
	assert.Equal(t, result.SequenceNumber, result2.SequenceNumber, "Both results should have same sequence number")
}

func Test_SnapshotAwareQueryHandler_Handle_IncrementalUpdateWithNewEvents(t *testing.T) {
	// setup
	ctx, snapshotHandler, wrapper := setupSnapshotTest(t)

	// arrange: create the first test book (will create sequence=1)
	createTestBook(ctx, t, wrapper)

	// act / assert

	// First query: Should miss snapshot and fall back to base handler, then create snapshot
	query := booksincirculation.BuildQuery()

	result1, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "First query should work")
	assert.Equal(t, 1, result1.Count, "Should have 1 book initially")
	assert.Equal(t, uint(1), result1.SequenceNumber, "Should have sequence=1")

	// Give snapshot saving some time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that a snapshot was created in the database
	filter := booksincirculation.BuildEventFilter()
	savedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, query.SnapshotType(), filter)
	assert.NoError(t, err, "Should be able to load saved snapshot")
	assert.NotNil(t, savedSnapshot, "Snapshot should exist after first query")

	// arrange: create a second test book (will create sequence=2)
	createTestBook(ctx, t, wrapper)

	// act / assert

	// Second query: Should hit snapshot and do incremental update with the new event
	result2, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "Second query should work")
	assert.Equal(t, 2, result2.Count, "Should have 2 books after incremental update")
	assert.Equal(t, uint(2), result2.SequenceNumber, "Should have sequence=2")

	// Verify the snapshot was updated with incremental changes
	time.Sleep(100 * time.Millisecond)
	updatedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, query.SnapshotType(), filter)
	assert.NoError(t, err, "Should be able to load updated snapshot")
	assert.NotNil(t, updatedSnapshot, "Updated snapshot should exist")
	assert.Equal(t, uint(2), updatedSnapshot.SequenceNumber, "Updated snapshot should have sequence=2")

	// Verify the updated snapshot contains the correct data
	var snapshotData booksincirculation.BooksInCirculation
	err = jsoniter.ConfigFastest.Unmarshal(updatedSnapshot.Data, &snapshotData)
	assert.NoError(t, err, "Should unmarshal snapshot data")
	assert.Equal(t, 2, snapshotData.Count, "Snapshot should contain 2 books")
	assert.Len(t, snapshotData.Books, 2, "Snapshot should have 2 book entries")
}

// Test helpers

func setupSnapshotTest(t *testing.T) (
	context.Context,
	*snapshot.QueryWrapper[booksincirculation.Query, booksincirculation.BooksInCirculation],
	Wrapper,
) {

	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	wrapper := CreateWrapperWithTestConfig(t)
	t.Cleanup(wrapper.Close)
	CleanUp(t, wrapper)

	baseHandler := booksincirculation.NewQueryHandler(wrapper.GetEventStore())

	snapshotHandler, err := snapshot.NewQueryWrapper[
		booksincirculation.Query,
		booksincirculation.BooksInCirculation,
	](
		baseHandler,
		wrapper.GetEventStore(),
		booksincirculation.Project,
		func(_ booksincirculation.Query) eventstore.Filter {
			return booksincirculation.BuildEventFilter()
		},
	)
	assert.NoError(t, err, "Should create snapshot-aware query handler")

	return ctx, snapshotHandler, wrapper
}

func createTestBook(ctx context.Context, t *testing.T, wrapper Wrapper) {
	t.Helper()

	bookID := GivenUniqueID(t)
	fakeClock := time.Unix(0, 0).UTC()

	addBookCmd := addbookcopy.BuildCommand(bookID, "978-1-234-56789-0", "Test Book", "Test Author", "1st", "Test Publisher", 2023, fakeClock)
	addBookHandler := addbookcopy.NewCommandHandler(wrapper.GetEventStore())

	_, err := addBookHandler.Handle(ctx, addBookCmd)
	assert.NoError(t, err, "Should create test book")
}
