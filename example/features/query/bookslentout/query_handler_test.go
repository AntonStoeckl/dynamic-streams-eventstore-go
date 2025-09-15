package bookslentout_test

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
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/bookslentout"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

type testHandlers struct {
	addBook        addbookcopy.CommandHandler
	lendBook       lendbookcopytoreader.CommandHandler
	returnBook     returnbookcopyfromreader.CommandHandler
	registerReader registerreader.CommandHandler
	query          bookslentout.QueryHandler
}

type testBookIDs struct {
	book1ID, book2ID, book3ID, book4ID uuid.UUID
}

type testReaderIDs struct {
	reader1ID, reader2ID, reader3ID uuid.UUID
}

func Test_QueryHandler_Handle_ReturnsLentBooksSortedByLentAt(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	books := createTestBooks(t)
	readers := createTestReaders(t)
	addBooksToLibrary(t, handlers, books, fakeClock)
	registerTestReaders(t, handlers, readers, fakeClock)

	lendBookCmd1 := lendbookcopytoreader.BuildCommand(books.book1ID, readers.reader1ID, fakeClock.Add(time.Hour))
	_, err := handlers.lendBook.Handle(ctx, lendBookCmd1)
	assert.NoError(t, err, "Should lend book1")

	lendBookCmd2 := lendbookcopytoreader.BuildCommand(books.book2ID, readers.reader2ID, fakeClock.Add(2*time.Hour))
	_, err = handlers.lendBook.Handle(ctx, lendBookCmd2)
	assert.NoError(t, err, "Should lend book2")

	// act
	result, err := handlers.query.Handle(ctx, bookslentout.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 books lent out")

	expectedLendings := []expectedLending{
		{BookID: books.book1ID.String(), ReaderID: readers.reader1ID.String(), LentAt: fakeClock.Add(time.Hour)},
		{BookID: books.book2ID.String(), ReaderID: readers.reader2ID.String(), LentAt: fakeClock.Add(2 * time.Hour)},
	}
	assertLendingsSortedCorrectly(t, result.Lendings, expectedLendings)
}

func Test_QueryHandler_Handle_ExcludesReturnedBooks(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	books := createTestBooks(t)
	readers := createTestReaders(t)
	addBooksToLibrary(t, handlers, books, fakeClock)
	registerTestReaders(t, handlers, readers, fakeClock)

	lendAllBooks(t, handlers, books, readers, fakeClock)

	returnBookCmd2 := returnbookcopyfromreader.BuildCommand(books.book2ID, readers.reader2ID, fakeClock.Add(5*time.Hour))
	_, err := handlers.returnBook.Handle(ctx, returnBookCmd2)
	assert.NoError(t, err, "Should return book2")

	returnBookCmd3 := returnbookcopyfromreader.BuildCommand(books.book3ID, readers.reader1ID, fakeClock.Add(6*time.Hour))
	_, err = handlers.returnBook.Handle(ctx, returnBookCmd3)
	assert.NoError(t, err, "Should return book3")

	// act
	result, err := handlers.query.Handle(ctx, bookslentout.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 books lent out (book2 and book3 were returned)")

	for _, lending := range result.Lendings {
		assert.NotEqual(t, books.book2ID.String(), lending.BookID, "book2 should not be in results (was returned)")
		assert.NotEqual(t, books.book3ID.String(), lending.BookID, "book3 should not be in results (was returned)")
	}
}

func Test_QueryHandler_Handle_ReturnsEmptyResult_WhenNoBooksLent(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	books := createTestBooks(t)
	readers := createTestReaders(t)
	addBooksToLibrary(t, handlers, books, fakeClock)
	registerTestReaders(t, handlers, readers, fakeClock)

	// act
	result, err := handlers.query.Handle(ctx, bookslentout.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 0, result.Count, "Should have 0 books lent out")
	assert.Len(t, result.Lendings, 0, "Should return empty Lendings slice")
}

func Test_QueryHandler_Handle_ReturnsCorrectLendingDetails(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	books := createTestBooks(t)
	readers := createTestReaders(t)
	addBooksToLibrary(t, handlers, books, fakeClock)
	registerTestReaders(t, handlers, readers, fakeClock)

	lendBookCmd := lendbookcopytoreader.BuildCommand(books.book1ID, readers.reader1ID, fakeClock.Add(time.Hour))
	_, err := handlers.lendBook.Handle(ctx, lendBookCmd)
	assert.NoError(t, err, "Should lend book1 to reader1")

	// act
	result, err := handlers.query.Handle(ctx, bookslentout.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 1, result.Count, "Should have 1 book lent out")
	assert.Len(t, result.Lendings, 1, "Should return 1 lending in Lendings slice")

	lending := result.Lendings[0]
	assertSpecificLendingDetails(t, lending, books.book1ID.String(), readers.reader1ID.String(), fakeClock.Add(time.Hour))
}

func Test_QueryHandler_Handle_WithStrongConsistency_WorksCorrectly(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithStrongConsistency(ctx)
	handlers := createAllHandlers(wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	books := createTestBooks(t)
	readers := createTestReaders(t)
	addBooksToLibrary(t, handlers, books, fakeClock)
	registerTestReaders(t, handlers, readers, fakeClock)

	lendBookCmd := lendbookcopytoreader.BuildCommand(books.book1ID, readers.reader1ID, fakeClock.Add(time.Hour))
	_, err := handlers.lendBook.Handle(ctx, lendBookCmd)
	assert.NoError(t, err, "Should lend book to reader")

	// act
	query := bookslentout.BuildQuery()
	result, err := handlers.query.Handle(ctx, query)

	// assert
	assert.NoError(t, err, "Query should succeed with strong consistency")
	assert.Equal(t, 1, result.Count, "Should find one lent book")
	assert.Len(t, result.Lendings, 1, "Should return one lending")
	assert.Equal(t, books.book1ID.String(), result.Lendings[0].BookID, "Should return the lent book")
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

func createAllHandlers(wrapper Wrapper) testHandlers {
	addBookHandler := addbookcopy.NewCommandHandler(wrapper.GetEventStore())
	registerReaderHandler := registerreader.NewCommandHandler(wrapper.GetEventStore())
	lendBookHandler := lendbookcopytoreader.NewCommandHandler(wrapper.GetEventStore())
	returnBookHandler := returnbookcopyfromreader.NewCommandHandler(wrapper.GetEventStore())

	queryHandler := bookslentout.NewQueryHandler(wrapper.GetEventStore())

	return testHandlers{
		addBook:        addBookHandler,
		lendBook:       lendBookHandler,
		returnBook:     returnBookHandler,
		registerReader: registerReaderHandler,
		query:          queryHandler,
	}
}

func createTestBooks(t *testing.T) testBookIDs {
	t.Helper()

	return testBookIDs{
		book1ID: GivenUniqueID(t),
		book2ID: GivenUniqueID(t),
		book3ID: GivenUniqueID(t),
		book4ID: GivenUniqueID(t),
	}
}

func createTestReaders(t *testing.T) testReaderIDs {
	t.Helper()

	return testReaderIDs{
		reader1ID: GivenUniqueID(t),
		reader2ID: GivenUniqueID(t),
		reader3ID: GivenUniqueID(t),
	}
}

func addBooksToLibrary(
	t *testing.T,
	handlers testHandlers,
	books testBookIDs,
	fakeClock time.Time,
) {
	t.Helper()

	addBookCmd1 := addbookcopy.BuildCommand(
		books.book1ID, "978-1-098-10013-1", "Learning Domain-Driven Design", "Vlad Khononov", "First Edition", "O'Reilly Media, Inc.", 2021, fakeClock,
	)
	_, err := handlers.addBook.Handle(context.Background(), addBookCmd1)
	assert.NoError(t, err, "Should add book1 to circulation")

	addBookCmd2 := addbookcopy.BuildCommand(
		books.book2ID, "978-0-201-63361-0", "Design Patterns", "Gang of Four", "First Edition", "Addison-Wesley", 1994, fakeClock.Add(time.Minute),
	)
	_, err = handlers.addBook.Handle(context.Background(), addBookCmd2)
	assert.NoError(t, err, "Should add book2 to circulation")

	addBookCmd3 := addbookcopy.BuildCommand(
		books.book3ID, "978-0-134-75316-6", "Effective Java", "Joshua Bloch", "Third Edition", "Addison-Wesley", 2017, fakeClock.Add(2*time.Minute),
	)
	_, err = handlers.addBook.Handle(context.Background(), addBookCmd3)
	assert.NoError(t, err, "Should add book3 to circulation")

	addBookCmd4 := addbookcopy.BuildCommand(
		books.book4ID, "978-0-321-12742-6", "Refactoring", "Martin Fowler", "Second Edition", "Addison-Wesley", 2018, fakeClock.Add(3*time.Minute),
	)
	_, err = handlers.addBook.Handle(context.Background(), addBookCmd4)
	assert.NoError(t, err, "Should add book4 to circulation")
}

func registerTestReaders(
	t *testing.T,
	handlers testHandlers,
	readers testReaderIDs,
	fakeClock time.Time,
) {
	t.Helper()

	registerReaderCmd1 := registerreader.BuildCommand(readers.reader1ID, "Test Reader 1", fakeClock.Add(4*time.Minute))
	_, err := handlers.registerReader.Handle(context.Background(), registerReaderCmd1)
	assert.NoError(t, err, "Should register reader1")

	registerReaderCmd2 := registerreader.BuildCommand(readers.reader2ID, "Test Reader 2", fakeClock.Add(5*time.Minute))
	_, err = handlers.registerReader.Handle(context.Background(), registerReaderCmd2)
	assert.NoError(t, err, "Should register reader2")

	registerReaderCmd3 := registerreader.BuildCommand(readers.reader3ID, "Test Reader 3", fakeClock.Add(6*time.Minute))
	_, err = handlers.registerReader.Handle(context.Background(), registerReaderCmd3)
	assert.NoError(t, err, "Should register reader3")
}

func lendAllBooks(
	t *testing.T,
	handlers testHandlers,
	books testBookIDs,
	readers testReaderIDs,
	fakeClock time.Time,
) {
	t.Helper()

	lendBookCmd1 := lendbookcopytoreader.BuildCommand(books.book1ID, readers.reader1ID, fakeClock.Add(time.Hour))
	_, err := handlers.lendBook.Handle(context.Background(), lendBookCmd1)
	assert.NoError(t, err, "Should lend book1 to reader1")

	lendBookCmd2 := lendbookcopytoreader.BuildCommand(books.book2ID, readers.reader2ID, fakeClock.Add(2*time.Hour))
	_, err = handlers.lendBook.Handle(context.Background(), lendBookCmd2)
	assert.NoError(t, err, "Should lend book2 to reader2")

	lendBookCmd3 := lendbookcopytoreader.BuildCommand(books.book3ID, readers.reader1ID, fakeClock.Add(3*time.Hour))
	_, err = handlers.lendBook.Handle(context.Background(), lendBookCmd3)
	assert.NoError(t, err, "Should lend book3 to reader1")

	lendBookCmd4 := lendbookcopytoreader.BuildCommand(books.book4ID, readers.reader3ID, fakeClock.Add(4*time.Hour))
	_, err = handlers.lendBook.Handle(context.Background(), lendBookCmd4)
	assert.NoError(t, err, "Should lend book4 to reader3")
}

// Assertion helpers

type expectedLending struct {
	BookID   string
	ReaderID string
	LentAt   time.Time
}

func assertLendingsSortedCorrectly(
	t *testing.T,
	lendings []bookslentout.LendingInfo,
	expected []expectedLending,
) {
	t.Helper()

	assert.Len(t, lendings, len(expected), "Should have correct number of lendings")

	for i, lending := range lendings {
		assert.Equal(t, expected[i].BookID, lending.BookID, "BookID should match for lending %d", i)
		assert.Equal(t, expected[i].ReaderID, lending.ReaderID, "ReaderID should match for lending %d", i)
		assert.Equal(t, expected[i].LentAt, lending.LentAt, "LentAt should match for lending %d", i)
	}

	for i := 0; i < len(lendings)-1; i++ {
		assert.True(t, lendings[i].LentAt.Before(lendings[i+1].LentAt) || lendings[i].LentAt.Equal(lendings[i+1].LentAt),
			"Lendings should be sorted by LentAt time (oldest first)")
	}
}

func assertSpecificLendingDetails(
	t *testing.T,
	lending bookslentout.LendingInfo,
	expectedBookID,
	expectedReaderID string,
	expectedLentAt time.Time,
) {
	t.Helper()

	assert.Equal(t, expectedBookID, lending.BookID, "Lending should have correct BookID")
	assert.Equal(t, expectedReaderID, lending.ReaderID, "Lending should have correct ReaderID")
	assert.Equal(t, expectedLentAt, lending.LentAt, "Lending should have correct LentAt time")
}
