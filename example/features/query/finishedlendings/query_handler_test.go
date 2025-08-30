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

type testEntities struct {
	book1, book2, book3, book4         uuid.UUID
	reader1, reader2, reader3, reader4 uuid.UUID
}

func Test_QueryHandler_Handle_ReturnsEmptyResult_WhenNoLendingsConducted(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	entities := createTestEntities(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange - add books and readers but don't lend or return any
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

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	entities := createTestEntities(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	setupBooksAndReaders(t, handlers, entities, fakeClock)

	// Lend book1 to reader1 and return at a specific time
	lendCmd1 := lendbookcopytoreader.BuildCommand(entities.book1, entities.reader1, fakeClock.Add(10*time.Minute))
	_, err := handlers.lendBook.Handle(ctx, lendCmd1)
	assert.NoError(t, err, "Should lend book1 to reader1")

	returnCmd1 := returnbookcopyfromreader.BuildCommand(entities.book1, entities.reader1, fakeClock.Add(20*time.Minute))
	_, err = handlers.returnBook.Handle(ctx, returnCmd1)
	assert.NoError(t, err, "Should return book1 from reader1")

	// Lend book2 to reader2 and return at a later time
	lendCmd2 := lendbookcopytoreader.BuildCommand(entities.book2, entities.reader2, fakeClock.Add(15*time.Minute))
	_, err = handlers.lendBook.Handle(ctx, lendCmd2)
	assert.NoError(t, err, "Should lend book2 to reader2")

	returnCmd2 := returnbookcopyfromreader.BuildCommand(entities.book2, entities.reader2, fakeClock.Add(25*time.Minute))
	_, err = handlers.returnBook.Handle(ctx, returnCmd2)
	assert.NoError(t, err, "Should return book2 from reader2")

	// act
	result, err := handlers.query.Handle(ctx, finishedlendings.BuildQuery(0))

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 finished lendings")

	// Should be sorted by ReturnedAt (oldest first)
	expectedOrder := []struct {
		BookID   string
		ReaderID string
	}{
		{entities.book1.String(), entities.reader1.String()},
		{entities.book2.String(), entities.reader2.String()},
	}
	assertFinishedLendingsSortedCorrectly(t, result.Lendings, expectedOrder)

	// Verify ReturnedAt times are correct
	assert.Equal(t, fakeClock.Add(20*time.Minute), result.Lendings[0].ReturnedAt, "book1 should have correct return time")
	assert.Equal(t, fakeClock.Add(25*time.Minute), result.Lendings[1].ReturnedAt, "book2 should have correct return time")
}

func Test_QueryHandler_Handle_ReturnsEmptyResult_WhenNoBooksAddedOrReadersRegistered(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)

	// act - nothing added
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

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	entities := createTestEntities(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	setupBooksAndReaders(t, handlers, entities, fakeClock)

	// Create one finished lending
	lendCmd1 := lendbookcopytoreader.BuildCommand(entities.book1, entities.reader1, fakeClock.Add(10*time.Minute))
	_, err := handlers.lendBook.Handle(ctx, lendCmd1)
	assert.NoError(t, err, "Should lend book1 to reader1")

	returnCmd1 := returnbookcopyfromreader.BuildCommand(entities.book1, entities.reader1, fakeClock.Add(20*time.Minute))
	_, err = handlers.returnBook.Handle(ctx, returnCmd1)
	assert.NoError(t, err, "Should return book1 from reader1")

	// Create an ongoing lending (lent but not returned)
	lendCmd2 := lendbookcopytoreader.BuildCommand(entities.book2, entities.reader2, fakeClock.Add(15*time.Minute))
	_, err = handlers.lendBook.Handle(ctx, lendCmd2)
	assert.NoError(t, err, "Should lend book2 to reader2")
	// Note: book2 is NOT returned, so it should not appear in finished lendings

	// act
	result, err := handlers.query.Handle(ctx, finishedlendings.BuildQuery(0))

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 1, result.Count, "Should have 1 finished lending (not counting the ongoing one)")

	// Verify only the finished lending is returned
	assert.Equal(t, entities.book1.String(), result.Lendings[0].BookID, "Should be book1")
	assert.Equal(t, entities.reader1.String(), result.Lendings[0].ReaderID, "Should be reader1")
	assert.Equal(t, fakeClock.Add(20*time.Minute), result.Lendings[0].ReturnedAt, "Should have correct return time")
}

func Test_QueryHandler_Handle_HandlesComplexLendingScenarios(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	entities := createTestEntities(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	setupBooksAndReaders(t, handlers, entities, fakeClock)

	// Scenario: book1 lent to reader1, returned, then lent again to reader2, then returned
	lendCmd1 := lendbookcopytoreader.BuildCommand(entities.book1, entities.reader1, fakeClock.Add(10*time.Minute))
	_, err := handlers.lendBook.Handle(ctx, lendCmd1)
	assert.NoError(t, err, "Should lend book1 to reader1")

	returnCmd1 := returnbookcopyfromreader.BuildCommand(entities.book1, entities.reader1, fakeClock.Add(20*time.Minute))
	_, err = handlers.returnBook.Handle(ctx, returnCmd1)
	assert.NoError(t, err, "Should return book1 from reader1")

	lendCmd2 := lendbookcopytoreader.BuildCommand(entities.book1, entities.reader2, fakeClock.Add(30*time.Minute))
	_, err = handlers.lendBook.Handle(ctx, lendCmd2)
	assert.NoError(t, err, "Should lend book1 to reader2")

	returnCmd2 := returnbookcopyfromreader.BuildCommand(entities.book1, entities.reader2, fakeClock.Add(40*time.Minute))
	_, err = handlers.returnBook.Handle(ctx, returnCmd2)
	assert.NoError(t, err, "Should return book1 from reader2")

	// Also add a different book lent to the same reader
	lendCmd3 := lendbookcopytoreader.BuildCommand(entities.book2, entities.reader1, fakeClock.Add(35*time.Minute))
	_, err = handlers.lendBook.Handle(ctx, lendCmd3)
	assert.NoError(t, err, "Should lend book2 to reader1")

	returnCmd3 := returnbookcopyfromreader.BuildCommand(entities.book2, entities.reader1, fakeClock.Add(45*time.Minute))
	_, err = handlers.returnBook.Handle(ctx, returnCmd3)
	assert.NoError(t, err, "Should return book2 from reader1")

	// act
	result, err := handlers.query.Handle(ctx, finishedlendings.BuildQuery(0))

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 3, result.Count, "Should have 3 finished lendings")

	// Should be sorted by ReturnedAt (oldest first)
	expectedOrder := []struct {
		BookID   string
		ReaderID string
	}{
		{entities.book1.String(), entities.reader1.String()}, // Returned at 20 min
		{entities.book1.String(), entities.reader2.String()}, // Returned at 40 min
		{entities.book2.String(), entities.reader1.String()}, // Returned at 45 min
	}
	assertFinishedLendingsSortedCorrectly(t, result.Lendings, expectedOrder)
}

func Test_QueryHandler_Handle_IdempotentReturns_ShowsEachLendingOnce(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	entities := createTestEntities(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	setupBooksAndReaders(t, handlers, entities, fakeClock)

	// Lend book1 to reader1
	lendCmd1 := lendbookcopytoreader.BuildCommand(entities.book1, entities.reader1, fakeClock.Add(10*time.Minute))
	_, err := handlers.lendBook.Handle(ctx, lendCmd1)
	assert.NoError(t, err, "Should lend book1 to reader1")

	// Return book1 multiple times (idempotent operations)
	returnCmd1 := returnbookcopyfromreader.BuildCommand(entities.book1, entities.reader1, fakeClock.Add(20*time.Minute))
	_, err = handlers.returnBook.Handle(ctx, returnCmd1)
	assert.NoError(t, err, "Should return book1 from reader1 (first time)")

	returnCmd2 := returnbookcopyfromreader.BuildCommand(entities.book1, entities.reader1, fakeClock.Add(25*time.Minute))
	_, err = handlers.returnBook.Handle(ctx, returnCmd2)
	assert.NoError(t, err, "Should handle idempotent return (second time)")

	// act
	result, err := handlers.query.Handle(ctx, finishedlendings.BuildQuery(0))

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 1, result.Count, "Should have exactly 1 finished lending (no duplicates)")
	assert.Len(t, result.Lendings, 1, "Should return exactly 1 finished lending")

	// Verify it's the correct lending with the first return time
	assert.Equal(t, entities.book1.String(), result.Lendings[0].BookID, "Should be book1")
	assert.Equal(t, entities.reader1.String(), result.Lendings[0].ReaderID, "Should be reader1")
	assert.Equal(t, fakeClock.Add(20*time.Minute), result.Lendings[0].ReturnedAt, "Should show first return time")
}

func Test_QueryHandler_Handle_WithStrongConsistency_WorksCorrectly(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use strong consistency (non-default for query handlers) to verify it still works
	ctx = eventstore.WithStrongConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)

	// act
	query := finishedlendings.BuildQuery(0)
	result, err := handlers.query.Handle(ctx, query)

	// assert
	assert.NoError(t, err, "Query should succeed with strong consistency")
	assert.NotNil(t, result, "Should return result")
}

// Test setup helpers.
func setupTestEnvironment(t *testing.T) (context.Context, Wrapper, func()) {
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
	addBookCopyHandler := addbookcopy.NewCommandHandler(wrapper.GetEventStore())
	registerReaderHandler := registerreader.NewCommandHandler(wrapper.GetEventStore())
	lendBookHandler := lendbookcopytoreader.NewCommandHandler(wrapper.GetEventStore())
	returnBookHandler := returnbookcopyfromreader.NewCommandHandler(wrapper.GetEventStore())

	queryHandler, err := finishedlendings.NewQueryHandler(wrapper.GetEventStore())
	assert.NoError(t, err, "Should create FinishedLendings query handler")

	return testHandlers{
		addBookCopy:    addBookCopyHandler,
		registerReader: registerReaderHandler,
		lendBook:       lendBookHandler,
		returnBook:     returnBookHandler,
		query:          queryHandler,
	}
}

func createTestEntities(t *testing.T) testEntities {
	return testEntities{
		book1:   GivenUniqueID(t),
		book2:   GivenUniqueID(t),
		book3:   GivenUniqueID(t),
		book4:   GivenUniqueID(t),
		reader1: GivenUniqueID(t),
		reader2: GivenUniqueID(t),
		reader3: GivenUniqueID(t),
		reader4: GivenUniqueID(t),
	}
}

func setupBooksAndReaders(t *testing.T, handlers testHandlers, entities testEntities, fakeClock time.Time) {
	// Add books to the library
	addBookCmd1 := addbookcopy.BuildCommand(entities.book1, "Test Book 1", "Author 1", "ISBN1", "1st", "Publisher 1", 2020, fakeClock)
	_, err := handlers.addBookCopy.Handle(context.Background(), addBookCmd1)
	assert.NoError(t, err, "Should add book1")

	addBookCmd2 := addbookcopy.BuildCommand(entities.book2, "Test Book 2", "Author 2", "ISBN2", "1st", "Publisher 2", 2021, fakeClock.Add(time.Minute))
	_, err = handlers.addBookCopy.Handle(context.Background(), addBookCmd2)
	assert.NoError(t, err, "Should add book2")

	addBookCmd3 := addbookcopy.BuildCommand(entities.book3, "Test Book 3", "Author 3", "ISBN3", "1st", "Publisher 3", 2022, fakeClock.Add(2*time.Minute))
	_, err = handlers.addBookCopy.Handle(context.Background(), addBookCmd3)
	assert.NoError(t, err, "Should add book3")

	addBookCmd4 := addbookcopy.BuildCommand(entities.book4, "Test Book 4", "Author 4", "ISBN4", "1st", "Publisher 4", 2023, fakeClock.Add(3*time.Minute))
	_, err = handlers.addBookCopy.Handle(context.Background(), addBookCmd4)
	assert.NoError(t, err, "Should add book4")

	// Register readers to the library
	registerReaderCmd1 := registerreader.BuildCommand(entities.reader1, "Alice Reader", fakeClock.Add(4*time.Minute))
	_, err = handlers.registerReader.Handle(context.Background(), registerReaderCmd1)
	assert.NoError(t, err, "Should register reader1")

	registerReaderCmd2 := registerreader.BuildCommand(entities.reader2, "Bob Reader", fakeClock.Add(5*time.Minute))
	_, err = handlers.registerReader.Handle(context.Background(), registerReaderCmd2)
	assert.NoError(t, err, "Should register reader2")

	registerReaderCmd3 := registerreader.BuildCommand(entities.reader3, "Charlie Reader", fakeClock.Add(6*time.Minute))
	_, err = handlers.registerReader.Handle(context.Background(), registerReaderCmd3)
	assert.NoError(t, err, "Should register reader3")

	registerReaderCmd4 := registerreader.BuildCommand(entities.reader4, "Diana Reader", fakeClock.Add(7*time.Minute))
	_, err = handlers.registerReader.Handle(context.Background(), registerReaderCmd4)
	assert.NoError(t, err, "Should register reader4")
}

// Assertion helpers.
func assertFinishedLendingsSortedCorrectly(t *testing.T, lendings []finishedlendings.LendingInfo, expectedOrder []struct{ BookID, ReaderID string }) {
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
