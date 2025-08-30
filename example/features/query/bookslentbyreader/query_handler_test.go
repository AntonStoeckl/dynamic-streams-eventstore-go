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
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/returnbookcopyfromreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/bookslentbyreader"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

type testHandlers struct {
	addBook        addbookcopy.CommandHandler
	lendBook       lendbookcopytoreader.CommandHandler
	returnBook     returnbookcopyfromreader.CommandHandler
	registerReader registerreader.CommandHandler
	query          bookslentbyreader.QueryHandler
}

type testBooks struct {
	book1, book2, book3 uuid.UUID
}

func Test_QueryHandler_Handle_ReturnsBooksLentByReader_SortedByLentAt(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	books := createTestBooks(t)
	readerID := GivenUniqueID(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	addBooksToLibrary(t, handlers, books, fakeClock)
	registerSingleReader(t, handlers, readerID, fakeClock)

	// Lend books at different times
	lendBookCmd1 := lendbookcopytoreader.BuildCommand(books.book1, readerID, fakeClock.Add(time.Hour))
	_, err := handlers.lendBook.Handle(ctx, lendBookCmd1)
	assert.NoError(t, err, "Should lend book1 to reader")

	lendBookCmd2 := lendbookcopytoreader.BuildCommand(books.book2, readerID, fakeClock.Add(2*time.Hour))
	_, err = handlers.lendBook.Handle(ctx, lendBookCmd2)
	assert.NoError(t, err, "Should lend book2 to reader")

	// act
	query := bookslentbyreader.BuildQuery(readerID)
	result, err := handlers.query.Handle(ctx, query)

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, readerID.String(), result.ReaderID, "Should return correct ReaderID")
	assert.Equal(t, 2, result.Count, "Reader should have 2 borrowed books")

	expectedBooks := []expectedBook{
		{BookID: books.book1.String(), LentAt: fakeClock.Add(time.Hour)},
		{BookID: books.book2.String(), LentAt: fakeClock.Add(2 * time.Hour)},
	}
	assertBooksSortedByLentAt(t, result.Books, expectedBooks)
}

func Test_QueryHandler_Handle_ExcludesReturnedBooks(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	books := createTestBooks(t)
	readerID := GivenUniqueID(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	addBooksToLibrary(t, handlers, books, fakeClock)
	registerSingleReader(t, handlers, readerID, fakeClock)

	// Lend all books to reader
	lendAllBooksToReader(t, handlers, books, readerID, fakeClock)

	// Return book2 - should not appear in final results
	returnBookCmd := returnbookcopyfromreader.BuildCommand(books.book2, readerID, fakeClock.Add(4*time.Hour))
	_, err := handlers.returnBook.Handle(ctx, returnBookCmd)
	assert.NoError(t, err, "Should return book2")

	// act
	query := bookslentbyreader.BuildQuery(readerID)
	result, err := handlers.query.Handle(ctx, query)

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Reader should have 2 borrowed books (book2 was returned)")

	// Verify the returned book is not in results
	for _, book := range result.Books {
		assert.NotEqual(t, books.book2.String(), book.BookID, "book2 should not be in results (was returned)")
	}
}

func Test_QueryHandler_Handle_ReturnsEmptyResult_WhenReaderHasNoBorrowedBooks(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	books := createTestBooks(t)
	readerID := GivenUniqueID(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange - add books and register reader but don't lend any
	addBooksToLibrary(t, handlers, books, fakeClock)
	registerSingleReader(t, handlers, readerID, fakeClock)

	// act
	query := bookslentbyreader.BuildQuery(readerID)
	result, err := handlers.query.Handle(ctx, query)

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, readerID.String(), result.ReaderID, "Should return correct ReaderID")
	assert.Equal(t, 0, result.Count, "Reader should have 0 borrowed books")
	assert.Len(t, result.Books, 0, "Should return empty Books slice")
}

func Test_QueryHandler_Handle_ReturnsCorrectBookDetails(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	books := createTestBooks(t)
	readerID := GivenUniqueID(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	addBooksToLibrary(t, handlers, books, fakeClock)
	registerSingleReader(t, handlers, readerID, fakeClock)

	// Lend a specific book
	lendBookCmd := lendbookcopytoreader.BuildCommand(books.book1, readerID, fakeClock.Add(time.Hour))
	_, err := handlers.lendBook.Handle(ctx, lendBookCmd)
	assert.NoError(t, err, "Should lend book1 to reader")

	// act
	query := bookslentbyreader.BuildQuery(readerID)
	result, err := handlers.query.Handle(ctx, query)

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 1, result.Count, "Reader should have 1 borrowed book")
	assert.Len(t, result.Books, 1, "Should return 1 book in Books slice")

	book := result.Books[0]
	assertSpecificBookDetails(t, book, books.book1.String(), fakeClock.Add(time.Hour))
}

func Test_QueryHandler_Handle_WithStrongConsistency_WorksCorrectly(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use strong consistency (non-default for query handlers) to verify it still works
	ctx = eventstore.WithStrongConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	books := createTestBooks(t)
	readerID := GivenUniqueID(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	addBooksToLibrary(t, handlers, books, fakeClock)
	registerSingleReader(t, handlers, readerID, fakeClock)

	lendBookCmd := lendbookcopytoreader.BuildCommand(books.book1, readerID, fakeClock.Add(time.Hour))
	_, err := handlers.lendBook.Handle(ctx, lendBookCmd)
	assert.NoError(t, err, "Should lend book to reader")

	// act
	query := bookslentbyreader.BuildQuery(readerID)
	result, err := handlers.query.Handle(ctx, query)

	// assert
	assert.NoError(t, err, "Query should succeed with strong consistency")
	assert.Equal(t, 1, result.Count, "Should find one lent book")
	assert.Len(t, result.Books, 1, "Should return one book")
	assert.Equal(t, books.book1.String(), result.Books[0].BookID, "Should return the lent book")
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
	addBookHandler := addbookcopy.NewCommandHandler(wrapper.GetEventStore())
	registerReaderHandler := registerreader.NewCommandHandler(wrapper.GetEventStore())
	lendBookHandler := lendbookcopytoreader.NewCommandHandler(wrapper.GetEventStore())
	returnBookHandler := returnbookcopyfromreader.NewCommandHandler(wrapper.GetEventStore())

	queryHandler, err := bookslentbyreader.NewQueryHandler(wrapper.GetEventStore())
	assert.NoError(t, err, "Should create BooksLentByReader query handler")

	return testHandlers{
		addBook:        addBookHandler,
		lendBook:       lendBookHandler,
		returnBook:     returnBookHandler,
		registerReader: registerReaderHandler,
		query:          queryHandler,
	}
}

func createTestBooks(t *testing.T) testBooks {
	return testBooks{
		book1: GivenUniqueID(t),
		book2: GivenUniqueID(t),
		book3: GivenUniqueID(t),
	}
}

func addBooksToLibrary(t *testing.T, handlers testHandlers, books testBooks, fakeClock time.Time) {
	addBookCmd1 := addbookcopy.BuildCommand(books.book1, "978-1-098-10013-1", "Learning Domain-Driven Design", "Vlad Khononov", "First Edition", "O'Reilly Media, Inc.", 2021, fakeClock)
	_, err := handlers.addBook.Handle(context.Background(), addBookCmd1)
	assert.NoError(t, err, "Should add book1 to circulation")

	addBookCmd2 := addbookcopy.BuildCommand(books.book2, "978-0-201-63361-0", "Design Patterns", "Gang of Four", "First Edition", "Addison-Wesley", 1994, fakeClock.Add(time.Minute))
	_, err = handlers.addBook.Handle(context.Background(), addBookCmd2)
	assert.NoError(t, err, "Should add book2 to circulation")

	addBookCmd3 := addbookcopy.BuildCommand(books.book3, "978-0-134-75316-6", "Effective Java", "Joshua Bloch", "Third Edition", "Addison-Wesley", 2017, fakeClock.Add(2*time.Minute))
	_, err = handlers.addBook.Handle(context.Background(), addBookCmd3)
	assert.NoError(t, err, "Should add book3 to circulation")
}

func registerSingleReader(t *testing.T, handlers testHandlers, readerID uuid.UUID, fakeClock time.Time) {
	registerReaderCmd := registerreader.BuildCommand(readerID, "Test Reader", fakeClock.Add(3*time.Minute))
	_, err := handlers.registerReader.Handle(context.Background(), registerReaderCmd)
	assert.NoError(t, err, "Should register reader")
}

func lendAllBooksToReader(t *testing.T, handlers testHandlers, books testBooks, readerID uuid.UUID, fakeClock time.Time) {
	lendBookCmd1 := lendbookcopytoreader.BuildCommand(books.book1, readerID, fakeClock.Add(time.Hour))
	_, err := handlers.lendBook.Handle(context.Background(), lendBookCmd1)
	assert.NoError(t, err, "Should lend book1 to reader")

	lendBookCmd2 := lendbookcopytoreader.BuildCommand(books.book2, readerID, fakeClock.Add(2*time.Hour))
	_, err = handlers.lendBook.Handle(context.Background(), lendBookCmd2)
	assert.NoError(t, err, "Should lend book2 to reader")

	lendBookCmd3 := lendbookcopytoreader.BuildCommand(books.book3, readerID, fakeClock.Add(3*time.Hour))
	_, err = handlers.lendBook.Handle(context.Background(), lendBookCmd3)
	assert.NoError(t, err, "Should lend book3 to reader")
}

// Assertion helpers.
type expectedBook struct {
	BookID string
	LentAt time.Time
}

func assertBooksSortedByLentAt(t *testing.T, books []bookslentbyreader.LendingInfo, expected []expectedBook) {
	assert.Len(t, books, len(expected), "Should have correct number of books")

	for i, book := range books {
		assert.Equal(t, expected[i].BookID, book.BookID, "BookID should match for book %d", i)
		assert.Equal(t, expected[i].LentAt, book.LentAt, "LentAt should match for book %d", i)
	}

	// Verify timestamps are properly ordered (oldest first)
	for i := 0; i < len(books)-1; i++ {
		assert.True(t, books[i].LentAt.Before(books[i+1].LentAt) || books[i].LentAt.Equal(books[i+1].LentAt),
			"Books should be sorted by LentAt time (oldest first)")
	}
}

func assertSpecificBookDetails(t *testing.T, book bookslentbyreader.LendingInfo, expectedBookID string, expectedLentAt time.Time) {
	assert.Equal(t, expectedBookID, book.BookID, "Book should have correct BookID")
	assert.Equal(t, expectedLentAt, book.LentAt, "Book should have correct LentAt time")
}
