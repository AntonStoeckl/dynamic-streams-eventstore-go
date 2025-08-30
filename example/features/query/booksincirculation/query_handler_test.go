package booksincirculation_test

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
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/removebookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/returnbookcopyfromreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/booksincirculation"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

type testHandlers struct {
	addBook        addbookcopy.CommandHandler
	removeBook     removebookcopy.CommandHandler
	lendBook       lendbookcopytoreader.CommandHandler
	returnBook     returnbookcopyfromreader.CommandHandler
	registerReader registerreader.CommandHandler
	query          booksincirculation.QueryHandler
}

type testBooks struct {
	book1, book2, book3, book4 uuid.UUID
}

type testReaders struct {
	reader1, reader2 uuid.UUID
}

func Test_QueryHandler_Handle_ReturnsCorrectBooksSortedByAddedAt(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	books := createTestBooks(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange - add books in chronological order
	addBooksToLibrary(t, handlers, books, fakeClock)

	// act
	result, err := handlers.query.Handle(ctx, booksincirculation.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 4, result.Count, "Should have 4 books in circulation")

	expectedOrder := []string{books.book1.String(), books.book2.String(), books.book3.String(), books.book4.String()}
	assertBooksSortedCorrectly(t, result.Books, expectedOrder)

	// Verify specific book details for the first book
	assertSpecificBookDetails(t, result.Books[0], "Learning Domain-Driven Design", "Vlad Khononov", "978-1-098-10013-1", 2021, false)
}

func Test_QueryHandler_Handle_ReturnsCorrectLendingStatus(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	books := createTestBooks(t)
	readers := createTestReaders(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	addBooksToLibrary(t, handlers, books, fakeClock)
	registerTestReaders(t, handlers, readers, fakeClock)

	// Lend book1 and book3
	lendBookCmd1 := lendbookcopytoreader.BuildCommand(books.book1, readers.reader1, fakeClock.Add(time.Hour))
	_, err := handlers.lendBook.Handle(ctx, lendBookCmd1)
	assert.NoError(t, err, "Should lend book1")

	lendBookCmd3 := lendbookcopytoreader.BuildCommand(books.book3, readers.reader2, fakeClock.Add(2*time.Hour))
	_, err = handlers.lendBook.Handle(ctx, lendBookCmd3)
	assert.NoError(t, err, "Should lend book3")

	// act
	result, err := handlers.query.Handle(ctx, booksincirculation.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	expectedLentStatus := []bool{true, false, true, false} // book1 and book3 are lent
	assertLendingStatusCorrect(t, result.Books, expectedLentStatus)
}

func Test_QueryHandler_Handle_ExcludesRemovedBooks(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	books := createTestBooks(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	addBooksToLibrary(t, handlers, books, fakeClock)

	// Remove book2 and book4
	removeBookCmd2 := removebookcopy.BuildCommand(books.book2, fakeClock.Add(20*time.Minute))
	_, err := handlers.removeBook.Handle(ctx, removeBookCmd2)
	assert.NoError(t, err, "Should remove book2 from circulation")

	removeBookCmd4 := removebookcopy.BuildCommand(books.book4, fakeClock.Add(21*time.Minute))
	_, err = handlers.removeBook.Handle(ctx, removeBookCmd4)
	assert.NoError(t, err, "Should remove book4 from circulation")

	// act
	result, err := handlers.query.Handle(ctx, booksincirculation.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 books remaining (book2 and book4 were removed)")

	expectedOrder := []string{books.book1.String(), books.book3.String()}
	assertBooksSortedCorrectly(t, result.Books, expectedOrder)

	// Verify removed books are not in results
	for _, book := range result.Books {
		assert.NotEqual(t, books.book2.String(), book.BookID, "book2 should not be in results (was removed)")
		assert.NotEqual(t, books.book4.String(), book.BookID, "book4 should not be in results (was removed)")
	}
}

func Test_QueryHandler_Handle_IncludesReturnedBooksAsAvailable(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	books := createTestBooks(t)
	readers := createTestReaders(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	addBooksToLibrary(t, handlers, books, fakeClock)
	registerTestReaders(t, handlers, readers, fakeClock)

	// Lend book1 and book2
	lendBookCmd1 := lendbookcopytoreader.BuildCommand(books.book1, readers.reader1, fakeClock.Add(time.Hour))
	_, err := handlers.lendBook.Handle(ctx, lendBookCmd1)
	assert.NoError(t, err, "Should lend book1")

	lendBookCmd2 := lendbookcopytoreader.BuildCommand(books.book2, readers.reader2, fakeClock.Add(2*time.Hour))
	_, err = handlers.lendBook.Handle(ctx, lendBookCmd2)
	assert.NoError(t, err, "Should lend book2")

	// Return book2 (should still be in circulation but not lent)
	returnBookCmd2 := returnbookcopyfromreader.BuildCommand(books.book2, readers.reader2, fakeClock.Add(3*time.Hour))
	_, err = handlers.returnBook.Handle(ctx, returnBookCmd2)
	assert.NoError(t, err, "Should return book2")

	// act
	result, err := handlers.query.Handle(ctx, booksincirculation.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 4, result.Count, "Should have all 4 books in circulation")

	expectedLentStatus := []bool{true, false, false, false} // only book1 is still lent
	assertLendingStatusCorrect(t, result.Books, expectedLentStatus)

	// Verify book2 is in results but not lent
	book2Found := false
	for _, book := range result.Books {
		if book.BookID == books.book2.String() {
			book2Found = true
			assert.False(t, book.IsCurrentlyLent, "book2 should be available (was returned)")
			break
		}
	}
	assert.True(t, book2Found, "book2 should be in results")
}

func Test_QueryHandler_Handle_ReturnsEmptyResult_WhenNoBooksInCirculation(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)

	// act
	result, err := handlers.query.Handle(ctx, booksincirculation.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 0, result.Count, "Should have 0 books in circulation")
	assert.Len(t, result.Books, 0, "Should return empty Books slice")
}

func Test_QueryHandler_Handle_ReturnsCorrectResult_WhenAllBooksAreRemoved(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	books := createTestBooks(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange - add books and then remove them all
	addBooksToLibrary(t, handlers, books, fakeClock)

	// Remove all books
	removeBookCmd1 := removebookcopy.BuildCommand(books.book1, fakeClock.Add(10*time.Minute))
	_, err := handlers.removeBook.Handle(ctx, removeBookCmd1)
	assert.NoError(t, err, "Should remove book1 from circulation")

	removeBookCmd2 := removebookcopy.BuildCommand(books.book2, fakeClock.Add(11*time.Minute))
	_, err = handlers.removeBook.Handle(ctx, removeBookCmd2)
	assert.NoError(t, err, "Should remove book2 from circulation")

	removeBookCmd3 := removebookcopy.BuildCommand(books.book3, fakeClock.Add(12*time.Minute))
	_, err = handlers.removeBook.Handle(ctx, removeBookCmd3)
	assert.NoError(t, err, "Should remove book3 from circulation")

	removeBookCmd4 := removebookcopy.BuildCommand(books.book4, fakeClock.Add(13*time.Minute))
	_, err = handlers.removeBook.Handle(ctx, removeBookCmd4)
	assert.NoError(t, err, "Should remove book4 from circulation")

	// act
	result, err := handlers.query.Handle(ctx, booksincirculation.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 0, result.Count, "Should have 0 books in circulation after all are removed")
	assert.Len(t, result.Books, 0, "Should return empty Books slice after all are removed")
}

func Test_QueryHandler_Handle_WorksWithStrongConsistency(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use strong consistency (non-default for query handlers) to verify it still works
	ctx = eventstore.WithStrongConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	books := createTestBooks(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange - add some books
	addBooksToLibrary(t, handlers, books, fakeClock)

	// act
	result, err := handlers.query.Handle(ctx, booksincirculation.BuildQuery())

	// assert - should work the same as with eventual consistency
	assert.NoError(t, err, "Query should succeed with strong consistency")
	assert.Equal(t, 4, result.Count, "Should have 4 books in circulation")
	assert.Len(t, result.Books, 4, "Should return 4 books")
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
	removeBookHandler := removebookcopy.NewCommandHandler(wrapper.GetEventStore())
	registerReaderHandler := registerreader.NewCommandHandler(wrapper.GetEventStore())
	lendBookHandler := lendbookcopytoreader.NewCommandHandler(wrapper.GetEventStore())
	returnBookHandler := returnbookcopyfromreader.NewCommandHandler(wrapper.GetEventStore())

	queryHandler, err := booksincirculation.NewQueryHandler(wrapper.GetEventStore())
	assert.NoError(t, err, "Should create BooksInCirculation query handler")

	return testHandlers{
		addBook:        addBookHandler,
		removeBook:     removeBookHandler,
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
		book4: GivenUniqueID(t),
	}
}

func createTestReaders(t *testing.T) testReaders {
	return testReaders{
		reader1: GivenUniqueID(t),
		reader2: GivenUniqueID(t),
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

	addBookCmd4 := addbookcopy.BuildCommand(books.book4, "978-0-321-12742-6", "Refactoring", "Martin Fowler", "Second Edition", "Addison-Wesley", 2018, fakeClock.Add(3*time.Minute))
	_, err = handlers.addBook.Handle(context.Background(), addBookCmd4)
	assert.NoError(t, err, "Should add book4 to circulation")
}

func registerTestReaders(t *testing.T, handlers testHandlers, readers testReaders, fakeClock time.Time) {
	registerReaderCmd1 := registerreader.BuildCommand(readers.reader1, "Alice Reader", fakeClock.Add(10*time.Minute))
	_, err := handlers.registerReader.Handle(context.Background(), registerReaderCmd1)
	assert.NoError(t, err, "Should register reader1")

	registerReaderCmd2 := registerreader.BuildCommand(readers.reader2, "Bob Reader", fakeClock.Add(11*time.Minute))
	_, err = handlers.registerReader.Handle(context.Background(), registerReaderCmd2)
	assert.NoError(t, err, "Should register reader2")
}

// Assertion helpers.
func assertBooksSortedCorrectly(t *testing.T, books []booksincirculation.BookInfo, expectedOrder []string) {
	assert.Len(t, books, len(expectedOrder), "Should have correct number of books")

	for i, book := range books {
		assert.Equal(t, expectedOrder[i], book.BookID, "Books should be sorted by AddedAt time")
	}

	// Verify timestamps are properly ordered
	for i := 0; i < len(books)-1; i++ {
		assert.True(t, books[i].AddedAt.Before(books[i+1].AddedAt) || books[i].AddedAt.Equal(books[i+1].AddedAt),
			"Books should be sorted by AddedAt time (oldest first)")
	}
}

func assertLendingStatusCorrect(t *testing.T, books []booksincirculation.BookInfo, expectedStatus []bool) {
	assert.Len(t, books, len(expectedStatus), "Should have correct number of books for status check")

	for i, book := range books {
		assert.Equal(t, expectedStatus[i], book.IsCurrentlyLent, "Book %d lending status should be correct", i)
	}
}

func assertSpecificBookDetails(t *testing.T, book booksincirculation.BookInfo, expectedTitle, expectedAuthor, expectedISBN string, expectedYear uint, expectedLent bool) {
	assert.Equal(t, expectedTitle, book.Title, "Book should have correct title")
	assert.Equal(t, expectedAuthor, book.Authors, "Book should have correct authors")
	assert.Equal(t, expectedISBN, book.ISBN, "Book should have correct ISBN")
	assert.Equal(t, expectedYear, book.PublicationYear, "Book should have correct publication year")
	assert.Equal(t, expectedLent, book.IsCurrentlyLent, "Book should have correct lending status")
}
