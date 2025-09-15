package bookslentout_test

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
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/bookslentout"
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
	result, err := snapshotHandler.Handle(ctx, bookslentout.BuildQuery())
	assert.NoError(t, err, "Snapshot handler should work")
	assert.Equal(t, 1, result.Count, "Should have 1 lending")
}

func Test_SnapshotAwareQueryHandler_Handle_SnapshotCreationAndHitWithNoNewEvents(t *testing.T) {
	// setup
	ctx, snapshotHandler, wrapper := setupSnapshotTest(t)

	// arrange
	createTestBook(ctx, t, wrapper)

	// act / assert

	// First query: Should miss snapshot and fall back to base handler
	// If wrapper works correctly, it should create a snapshot after a successful fallback
	query := bookslentout.BuildQuery()

	result, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "First query should work")
	assert.Equal(t, 1, result.Count, "Should have 1 lending")

	// Give snapshot saving some time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that a snapshot was created in the database
	filter := bookslentout.BuildEventFilter()
	savedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, query.SnapshotType(), filter)
	assert.NoError(t, err, "Should be able to load saved snapshot")
	assert.NotNil(t, savedSnapshot, "Snapshot should exist after first query")

	// Second query: Should hit snapshot (no new events since last query)
	result2, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "Second query should work")
	assert.Equal(t, 1, result2.Count, "Should still have 1 lending")
	assert.Equal(t, result.SequenceNumber, result2.SequenceNumber, "Both results should have same sequence number")
}

func Test_SnapshotAwareQueryHandler_Handle_IncrementalUpdateWithNewEvents(t *testing.T) {
	// setup
	ctx, snapshotHandler, wrapper := setupSnapshotTest(t)

	// arrange: create the first test book (will create sequence=3)
	createTestBook(ctx, t, wrapper)

	// act / assert

	// First query: Should miss snapshot and fall back to base handler, then create snapshot
	query := bookslentout.BuildQuery()

	result1, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "First query should work")
	assert.Equal(t, 1, result1.Count, "Should have 1 lending initially")
	assert.Equal(t, uint(3), result1.SequenceNumber, "Should have sequence=3 (register+addbook+lend)")

	// Give snapshot saving some time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that a snapshot was created in the database
	filter := bookslentout.BuildEventFilter()
	savedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, query.SnapshotType(), filter)
	assert.NoError(t, err, "Should be able to load saved snapshot")
	assert.NotNil(t, savedSnapshot, "Snapshot should exist after first query")

	// arrange: create a second test book (will create sequence=6)
	createSecondTestBook(ctx, t, wrapper)

	// act / assert

	// Second query: Should hit snapshot and do incremental update with new events
	result2, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "Second query should work")
	assert.Equal(t, 2, result2.Count, "Should have 2 lendings after incremental update")
	assert.Equal(t, uint(6), result2.SequenceNumber, "Should have sequence=6 after processing new events")

	// Verify we have both lendings (the first lending should be at index 0 due to earlier timestamp)
	assert.Equal(t, result1.Lendings[0].BookID, result2.Lendings[0].BookID, "First lending should still be present")
	assert.Len(t, result2.Lendings, 2, "Should have exactly 2 lendings")
	// The second lending should be different from the first
	assert.NotEqual(t, result2.Lendings[0].BookID, result2.Lendings[1].BookID, "Second lending should be different book")

	// Verify the snapshot was updated with incremental changes
	time.Sleep(100 * time.Millisecond)
	updatedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, query.SnapshotType(), filter)
	assert.NoError(t, err, "Should be able to load updated snapshot")
	assert.NotNil(t, updatedSnapshot, "Updated snapshot should exist")
	assert.Equal(t, uint(6), updatedSnapshot.SequenceNumber, "Updated snapshot should have sequence=6")

	// Verify the updated snapshot contains the correct data
	var snapshotData bookslentout.BooksLentOut
	err = jsoniter.ConfigFastest.Unmarshal(updatedSnapshot.Data, &snapshotData)
	assert.NoError(t, err, "Should unmarshal snapshot data")
	assert.Equal(t, 2, snapshotData.Count, "Snapshot should contain 2 lendings")
	assert.Len(t, snapshotData.Lendings, 2, "Snapshot should have 2 lending entries")
}

// Test helpers

func setupSnapshotTest(t *testing.T) (
	context.Context,
	*snapshot.QueryWrapper[bookslentout.Query, bookslentout.BooksLentOut],
	Wrapper,
) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	wrapper := CreateWrapperWithTestConfig(t)
	t.Cleanup(wrapper.Close)
	CleanUp(t, wrapper)

	baseHandler := bookslentout.NewQueryHandler(wrapper.GetEventStore())

	snapshotHandler, err := snapshot.NewQueryWrapper[
		bookslentout.Query,
		bookslentout.BooksLentOut,
	](
		baseHandler,
		wrapper.GetEventStore(),
		bookslentout.Project,
		func(_ bookslentout.Query) eventstore.Filter {
			return bookslentout.BuildEventFilter()
		},
	)
	assert.NoError(t, err, "Should create snapshot-aware query handler")

	return ctx, snapshotHandler, wrapper
}

func createTestBook(ctx context.Context, t *testing.T, wrapper Wrapper) {
	t.Helper()

	bookID := GivenUniqueID(t)
	readerID := GivenUniqueID(t)
	fakeClock := time.Unix(0, 0).UTC()

	// First, register a reader
	registerReaderCmd := registerreader.BuildCommand(readerID, "Test Reader", fakeClock)
	readerHandler := registerreader.NewCommandHandler(wrapper.GetEventStore())
	_, err := readerHandler.Handle(ctx, registerReaderCmd)
	assert.NoError(t, err, "Should register reader")

	// Then add a book to circulation
	addBookCmd := addbookcopy.BuildCommand(bookID, "978-1-234-56789-0", "Test Book", "Test Author", "1st", "Test Publisher", 2023, fakeClock)
	addBookHandler := addbookcopy.NewCommandHandler(wrapper.GetEventStore())
	_, err = addBookHandler.Handle(ctx, addBookCmd)
	assert.NoError(t, err, "Should add book to circulation")

	// Finally, lend the book to the reader
	lendBookCmd := lendbookcopytoreader.BuildCommand(bookID, readerID, fakeClock.Add(time.Minute))
	lendHandler := lendbookcopytoreader.NewCommandHandler(wrapper.GetEventStore())
	_, err = lendHandler.Handle(ctx, lendBookCmd)
	assert.NoError(t, err, "Should lend book to reader")
}

func createSecondTestBook(ctx context.Context, t *testing.T, wrapper Wrapper) {
	t.Helper()

	bookID := GivenUniqueID(t)
	readerID := GivenUniqueID(t)
	fakeClock := time.Unix(1, 0).UTC() // Slightly different timestamp

	// First register a second reader
	registerReaderCmd := registerreader.BuildCommand(readerID, "Second Reader", fakeClock)
	readerHandler := registerreader.NewCommandHandler(wrapper.GetEventStore())
	_, err := readerHandler.Handle(ctx, registerReaderCmd)
	assert.NoError(t, err, "Should register second reader")

	// Then add the second book to circulation
	addBookCmd := addbookcopy.BuildCommand(bookID, "978-2-345-67890-1", "Second Book", "Second Author", "2nd", "Second Publisher", 2024, fakeClock)
	addBookHandler := addbookcopy.NewCommandHandler(wrapper.GetEventStore())
	_, err = addBookHandler.Handle(ctx, addBookCmd)
	assert.NoError(t, err, "Should add second book to circulation")

	// Finally, lend the second book to the second reader
	lendBookCmd := lendbookcopytoreader.BuildCommand(bookID, readerID, fakeClock.Add(time.Minute))
	lendHandler := lendbookcopytoreader.NewCommandHandler(wrapper.GetEventStore())
	_, err = lendHandler.Handle(ctx, lendBookCmd)
	assert.NoError(t, err, "Should lend second book to reader")
}
