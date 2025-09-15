package bookslentbyreader_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/addbookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/lendbookcopytoreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/registerreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/bookslentbyreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/snapshot"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

func Test_SnapshotAwareQueryHandler_Handle_SnapshotMiss(t *testing.T) {
	// setup
	ctx, snapshotHandler, wrapper := setupSnapshotTest(t)

	// arrange
	readerID := createFirstTestLending(ctx, t, wrapper)

	// act
	query := bookslentbyreader.BuildQuery(readerID)
	result, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "Snapshot handler should work")
	assert.Equal(t, 1, result.Count, "Should have 1 lending")
}

func Test_SnapshotAwareQueryHandler_Handle_SnapshotCreationAndHitWithNoNewEvents(t *testing.T) {
	// setup
	ctx, snapshotHandler, wrapper := setupSnapshotTest(t)

	// arrange
	readerID := createFirstTestLending(ctx, t, wrapper)

	// act / assert

	// First query: Should miss snapshot and fall back to base handler
	// If wrapper works correctly, it should create a snapshot after a successful fallback
	query := bookslentbyreader.BuildQuery(readerID)

	result, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "First query should work")
	assert.Equal(t, 1, result.Count, "Should have 1 lending")

	// Give snapshot saving some time to complete
	time.Sleep(100 * time.Millisecond)

	// Second query: Should hit snapshot (no new events since maxSeq=3)
	result2, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "Second query should work")
	assert.Equal(t, 1, result2.Count, "Should still have 1 lending")
}

func Test_SnapshotAwareQueryHandler_Handle_SnapshotHitWithNewEvents(t *testing.T) {

	// setup
	ctx, snapshotHandler, wrapper := setupSnapshotTest(t)

	// arrange
	readerID := createFirstTestLending(ctx, t, wrapper)

	// act / assert

	// First query: Should miss snapshot and fall back to base handler, then create snapshot
	query := bookslentbyreader.BuildQuery(readerID)
	result1, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "First query should work")
	assert.Equal(t, 1, result1.Count, "Should have 1 lending initially")

	// Give snapshot saving some time to complete
	time.Sleep(100 * time.Millisecond)

	// Add a second lending (creates new events)
	createSecondTestLending(ctx, t, readerID, wrapper)

	// Second query: Should hit snapshot and process incremental events
	result2, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "Second query should work")
	assert.Equal(t, 2, result2.Count, "Should have 2 lendings after incremental processing")
}

// Test helpers

func setupSnapshotTest(t *testing.T) (
	context.Context,
	*snapshot.QueryWrapper[bookslentbyreader.Query, bookslentbyreader.BooksCurrentlyLent],
	Wrapper,
) {

	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	wrapper := CreateWrapperWithTestConfig(t)
	t.Cleanup(wrapper.Close)
	CleanUp(t, wrapper)

	baseHandler := bookslentbyreader.NewQueryHandler(wrapper.GetEventStore())

	snapshotHandler, err := snapshot.NewQueryWrapper[
		bookslentbyreader.Query,
		bookslentbyreader.BooksCurrentlyLent,
	](
		baseHandler,
		wrapper.GetEventStore(),
		bookslentbyreader.Project,
		func(q bookslentbyreader.Query) eventstore.Filter {
			return bookslentbyreader.BuildEventFilter(q.ReaderID)
		},
	)
	assert.NoError(t, err, "Should create snapshot-aware query handler")

	return ctx, snapshotHandler, wrapper
}

func createFirstTestLending(ctx context.Context, t *testing.T, wrapper Wrapper) uuid.UUID {
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

	return readerID
}

func createSecondTestLending(ctx context.Context, t *testing.T, readerID uuid.UUID, wrapper Wrapper) {
	t.Helper()

	bookID := GivenUniqueID(t)
	fakeClock := time.Unix(1, 0).UTC() // Slightly different timestamp

	// Add the second book to circulation
	addBookCmd := addbookcopy.BuildCommand(bookID, "978-2-345-67890-1", "Second Book", "Second Author", "2nd", "Second Publisher", 2024, fakeClock)
	addBookHandler := addbookcopy.NewCommandHandler(wrapper.GetEventStore())
	_, err := addBookHandler.Handle(ctx, addBookCmd)
	assert.NoError(t, err, "Should add second book to circulation")

	// Then lend the second book to the reader
	lendBookCmd := lendbookcopytoreader.BuildCommand(bookID, readerID, fakeClock.Add(time.Minute))
	lendHandler := lendbookcopytoreader.NewCommandHandler(wrapper.GetEventStore())
	_, err = lendHandler.Handle(ctx, lendBookCmd)
	assert.NoError(t, err, "Should lend second book to reader")
}
