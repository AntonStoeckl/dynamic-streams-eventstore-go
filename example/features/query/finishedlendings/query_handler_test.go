package finishedlendings_test

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
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/returnbookcopyfromreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/finishedlendings"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

type testHandlers struct {
	addBookCopy    addbookcopy.CommandHandler
	registerReader registerreader.CommandHandler
	lendBook       lendbookcopytoreader.CommandHandler
	returnBook     returnbookcopyfromreader.CommandHandler
	query          finishedlendings.QueryHandler
}

type testEntityIDs struct {
	book1ID, book2ID, book3ID, book4ID         uuid.UUID
	reader1ID, reader2ID, reader3ID, reader4ID uuid.UUID
}

func Test_QueryHandler_Handle_ReturnsEmptyResult_WhenNoLendingsConducted(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	entities := createTestEntities(t)
	setupBooksAndReaders(t, handlers, entities, fakeClock)

	// act
	result, err := handlers.query.Handle(ctx, finishedlendings.BuildQuery(0))

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 0, result.Count, "Should have 0 finished lendings")
	assert.Len(t, result.Lendings, 0, "Should return empty Lendings slice")
}

func Test_QueryHandler_Handle_IncludesFinishedLendingsSortedByReturnedAt(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	entities := createTestEntities(t)
	setupBooksAndReaders(t, handlers, entities, fakeClock)
	lendCmd1 := lendbookcopytoreader.BuildCommand(entities.book1ID, entities.reader1ID, fakeClock.Add(10*time.Minute))
	_, err := handlers.lendBook.Handle(ctx, lendCmd1)
	assert.NoError(t, err, "Should lend book1 to reader1")

	returnCmd1 := returnbookcopyfromreader.BuildCommand(entities.book1ID, entities.reader1ID, fakeClock.Add(20*time.Minute))
	_, err = handlers.returnBook.Handle(ctx, returnCmd1)
	assert.NoError(t, err, "Should return book1 from reader1")

	lendCmd2 := lendbookcopytoreader.BuildCommand(entities.book2ID, entities.reader2ID, fakeClock.Add(15*time.Minute))
	_, err = handlers.lendBook.Handle(ctx, lendCmd2)
	assert.NoError(t, err, "Should lend book2 to reader2")

	returnCmd2 := returnbookcopyfromreader.BuildCommand(entities.book2ID, entities.reader2ID, fakeClock.Add(25*time.Minute))
	_, err = handlers.returnBook.Handle(ctx, returnCmd2)
	assert.NoError(t, err, "Should return book2 from reader2")

	// act
	result, err := handlers.query.Handle(ctx, finishedlendings.BuildQuery(0))

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 finished lendings")

	expectedOrder := []struct {
		BookID   string
		ReaderID string
	}{
		{entities.book1ID.String(), entities.reader1ID.String()},
		{entities.book2ID.String(), entities.reader2ID.String()},
	}
	assertFinishedLendingsSortedCorrectly(t, result.Lendings, expectedOrder)
	assert.Equal(t, fakeClock.Add(20*time.Minute), result.Lendings[0].ReturnedAt, "book1 should have correct return time")
	assert.Equal(t, fakeClock.Add(25*time.Minute), result.Lendings[1].ReturnedAt, "book2 should have correct return time")
}

func Test_QueryHandler_Handle_ReturnsEmptyResult_WhenNoBooksAddedOrReadersRegistered(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)

	// act
	result, err := handlers.query.Handle(ctx, finishedlendings.BuildQuery(0))

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 0, result.Count, "Should have 0 finished lendings")
	assert.Len(t, result.Lendings, 0, "Should return empty Lendings slice")
}

func Test_QueryHandler_Handle_IgnoresOngoingLendings(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	entities := createTestEntities(t)
	setupBooksAndReaders(t, handlers, entities, fakeClock)
	lendCmd1 := lendbookcopytoreader.BuildCommand(entities.book1ID, entities.reader1ID, fakeClock.Add(10*time.Minute))
	_, err := handlers.lendBook.Handle(ctx, lendCmd1)
	assert.NoError(t, err, "Should lend book1 to reader1")

	returnCmd1 := returnbookcopyfromreader.BuildCommand(entities.book1ID, entities.reader1ID, fakeClock.Add(20*time.Minute))
	_, err = handlers.returnBook.Handle(ctx, returnCmd1)
	assert.NoError(t, err, "Should return book1 from reader1")

	lendCmd2 := lendbookcopytoreader.BuildCommand(entities.book2ID, entities.reader2ID, fakeClock.Add(15*time.Minute))
	_, err = handlers.lendBook.Handle(ctx, lendCmd2)
	assert.NoError(t, err, "Should lend book2 to reader2")

	// act
	result, err := handlers.query.Handle(ctx, finishedlendings.BuildQuery(0))

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 1, result.Count, "Should have 1 finished lending (not counting the ongoing one)")

	assert.Equal(t, entities.book1ID.String(), result.Lendings[0].BookID, "Should be book1")
	assert.Equal(t, entities.reader1ID.String(), result.Lendings[0].ReaderID, "Should be reader1")
	assert.Equal(t, fakeClock.Add(20*time.Minute), result.Lendings[0].ReturnedAt, "Should have correct return time")
}

func Test_QueryHandler_Handle_HandlesComplexLendingScenarios(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	entities := createTestEntities(t)
	setupBooksAndReaders(t, handlers, entities, fakeClock)
	lendCmd1 := lendbookcopytoreader.BuildCommand(entities.book1ID, entities.reader1ID, fakeClock.Add(10*time.Minute))
	_, err := handlers.lendBook.Handle(ctx, lendCmd1)
	assert.NoError(t, err, "Should lend book1 to reader1")

	returnCmd1 := returnbookcopyfromreader.BuildCommand(entities.book1ID, entities.reader1ID, fakeClock.Add(20*time.Minute))
	_, err = handlers.returnBook.Handle(ctx, returnCmd1)
	assert.NoError(t, err, "Should return book1 from reader1")

	lendCmd2 := lendbookcopytoreader.BuildCommand(entities.book1ID, entities.reader2ID, fakeClock.Add(30*time.Minute))
	_, err = handlers.lendBook.Handle(ctx, lendCmd2)
	assert.NoError(t, err, "Should lend book1 to reader2")

	returnCmd2 := returnbookcopyfromreader.BuildCommand(entities.book1ID, entities.reader2ID, fakeClock.Add(40*time.Minute))
	_, err = handlers.returnBook.Handle(ctx, returnCmd2)
	assert.NoError(t, err, "Should return book1 from reader2")

	lendCmd3 := lendbookcopytoreader.BuildCommand(entities.book2ID, entities.reader1ID, fakeClock.Add(35*time.Minute))
	_, err = handlers.lendBook.Handle(ctx, lendCmd3)
	assert.NoError(t, err, "Should lend book2 to reader1")

	returnCmd3 := returnbookcopyfromreader.BuildCommand(entities.book2ID, entities.reader1ID, fakeClock.Add(45*time.Minute))
	_, err = handlers.returnBook.Handle(ctx, returnCmd3)
	assert.NoError(t, err, "Should return book2 from reader1")

	// act
	result, err := handlers.query.Handle(ctx, finishedlendings.BuildQuery(0))

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 3, result.Count, "Should have 3 finished lendings")

	expectedOrder := []struct {
		BookID   string
		ReaderID string
	}{
		{entities.book1ID.String(), entities.reader1ID.String()},
		{entities.book1ID.String(), entities.reader2ID.String()},
		{entities.book2ID.String(), entities.reader1ID.String()},
	}
	assertFinishedLendingsSortedCorrectly(t, result.Lendings, expectedOrder)
}

func Test_QueryHandler_Handle_IdempotentReturns_ShowsEachLendingOnce(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	entities := createTestEntities(t)
	setupBooksAndReaders(t, handlers, entities, fakeClock)
	lendCmd1 := lendbookcopytoreader.BuildCommand(entities.book1ID, entities.reader1ID, fakeClock.Add(10*time.Minute))
	_, err := handlers.lendBook.Handle(ctx, lendCmd1)
	assert.NoError(t, err, "Should lend book1 to reader1")

	returnCmd1 := returnbookcopyfromreader.BuildCommand(entities.book1ID, entities.reader1ID, fakeClock.Add(20*time.Minute))
	_, err = handlers.returnBook.Handle(ctx, returnCmd1)
	assert.NoError(t, err, "Should return book1 from reader1 (first time)")

	returnCmd2 := returnbookcopyfromreader.BuildCommand(entities.book1ID, entities.reader1ID, fakeClock.Add(25*time.Minute))
	_, err = handlers.returnBook.Handle(ctx, returnCmd2)
	assert.NoError(t, err, "Should handle idempotent return (second time)")

	// act
	result, err := handlers.query.Handle(ctx, finishedlendings.BuildQuery(0))

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 1, result.Count, "Should have exactly 1 finished lending (no duplicates)")
	assert.Len(t, result.Lendings, 1, "Should return exactly 1 finished lending")

	assert.Equal(t, entities.book1ID.String(), result.Lendings[0].BookID, "Should be book1")
	assert.Equal(t, entities.reader1ID.String(), result.Lendings[0].ReaderID, "Should be reader1")
	assert.Equal(t, fakeClock.Add(20*time.Minute), result.Lendings[0].ReturnedAt, "Should show first return time")
}

func Test_QueryHandler_Handle_WithStrongConsistency_WorksCorrectly(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithStrongConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)

	// act
	query := finishedlendings.BuildQuery(0)
	result, err := handlers.query.Handle(ctx, query)

	// assert
	assert.NoError(t, err, "Query should succeed with strong consistency")
	assert.NotNil(t, result, "Should return result")
}

// Test helpers

func setupTestEnvironment(t *testing.T) (context.Context, Wrapper, func()) {
	t.Helper()

	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	wrapper := CreateWrapperWithTestConfig(t)
	CleanUp(t, wrapper)

	cleanup := func() {
		cancel()
		wrapper.Close()
	}

	return ctxWithTimeout, wrapper, cleanup
}

func createAllHandlers(t *testing.T, wrapper Wrapper) testHandlers {
	t.Helper()

	addBookCopyHandler := addbookcopy.NewCommandHandler(wrapper.GetEventStore())
	registerReaderHandler := registerreader.NewCommandHandler(wrapper.GetEventStore())
	lendBookHandler := lendbookcopytoreader.NewCommandHandler(wrapper.GetEventStore())
	returnBookHandler := returnbookcopyfromreader.NewCommandHandler(wrapper.GetEventStore())
	queryHandler := finishedlendings.NewQueryHandler(wrapper.GetEventStore())

	return testHandlers{
		addBookCopy:    addBookCopyHandler,
		registerReader: registerReaderHandler,
		lendBook:       lendBookHandler,
		returnBook:     returnBookHandler,
		query:          queryHandler,
	}
}

func createTestEntities(t *testing.T) testEntityIDs {
	t.Helper()

	return testEntityIDs{
		book1ID:   GivenUniqueID(t),
		book2ID:   GivenUniqueID(t),
		book3ID:   GivenUniqueID(t),
		book4ID:   GivenUniqueID(t),
		reader1ID: GivenUniqueID(t),
		reader2ID: GivenUniqueID(t),
		reader3ID: GivenUniqueID(t),
		reader4ID: GivenUniqueID(t),
	}
}

func setupBooksAndReaders(
	t *testing.T,
	handlers testHandlers,
	entities testEntityIDs,
	fakeClock time.Time,
) {
	t.Helper()
	addBookCmd1 := addbookcopy.BuildCommand(entities.book1ID, "Test Book 1", "Author 1", "ISBN1", "1st", "Publisher 1", 2020, fakeClock)
	_, err := handlers.addBookCopy.Handle(context.Background(), addBookCmd1)
	assert.NoError(t, err, "Should add book1")

	addBookCmd2 := addbookcopy.BuildCommand(entities.book2ID, "Test Book 2", "Author 2", "ISBN2", "1st", "Publisher 2", 2021, fakeClock.Add(time.Minute))
	_, err = handlers.addBookCopy.Handle(context.Background(), addBookCmd2)
	assert.NoError(t, err, "Should add book2")

	addBookCmd3 := addbookcopy.BuildCommand(entities.book3ID, "Test Book 3", "Author 3", "ISBN3", "1st", "Publisher 3", 2022, fakeClock.Add(2*time.Minute))
	_, err = handlers.addBookCopy.Handle(context.Background(), addBookCmd3)
	assert.NoError(t, err, "Should add book3")

	addBookCmd4 := addbookcopy.BuildCommand(entities.book4ID, "Test Book 4", "Author 4", "ISBN4", "1st", "Publisher 4", 2023, fakeClock.Add(3*time.Minute))
	_, err = handlers.addBookCopy.Handle(context.Background(), addBookCmd4)
	assert.NoError(t, err, "Should add book4")

	// Register readers to the library
	registerReaderCmd1 := registerreader.BuildCommand(entities.reader1ID, "Alice Reader", fakeClock.Add(4*time.Minute))
	_, err = handlers.registerReader.Handle(context.Background(), registerReaderCmd1)
	assert.NoError(t, err, "Should register reader1")

	registerReaderCmd2 := registerreader.BuildCommand(entities.reader2ID, "Bob Reader", fakeClock.Add(5*time.Minute))
	_, err = handlers.registerReader.Handle(context.Background(), registerReaderCmd2)
	assert.NoError(t, err, "Should register reader2")

	registerReaderCmd3 := registerreader.BuildCommand(entities.reader3ID, "Charlie Reader", fakeClock.Add(6*time.Minute))
	_, err = handlers.registerReader.Handle(context.Background(), registerReaderCmd3)
	assert.NoError(t, err, "Should register reader3")

	registerReaderCmd4 := registerreader.BuildCommand(entities.reader4ID, "Diana Reader", fakeClock.Add(7*time.Minute))
	_, err = handlers.registerReader.Handle(context.Background(), registerReaderCmd4)
	assert.NoError(t, err, "Should register reader4")
}

func assertFinishedLendingsSortedCorrectly(
	t *testing.T,
	lendings []finishedlendings.LendingInfo,
	expectedOrder []struct{ BookID, ReaderID string },
) {
	t.Helper()

	assert.Len(t, lendings, len(expectedOrder), "Should have correct number of finished lendings")

	for i, lending := range lendings {
		assert.Equal(t, expectedOrder[i].BookID, lending.BookID, "Finished lendings should be sorted by ReturnedAt time")
		assert.Equal(t, expectedOrder[i].ReaderID, lending.ReaderID, "Finished lendings should be sorted by ReturnedAt time")
	}

	// Verify timestamps are properly ordered
	for i := 0; i < len(lendings)-1; i++ {
		assert.True(t, lendings[i].ReturnedAt.Before(lendings[i+1].ReturnedAt) || lendings[i].ReturnedAt.Equal(lendings[i+1].ReturnedAt),
			"Finished lendings should be sorted by ReturnedAt time (oldest first)")
	}
}
