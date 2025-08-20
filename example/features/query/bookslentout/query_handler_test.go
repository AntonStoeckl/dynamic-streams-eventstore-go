package bookslentout_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

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

type testBooks struct {
	book1, book2, book3, book4 uuid.UUID
}

type testReaders struct {
	reader1, reader2, reader3 uuid.UUID
}

func Test_QueryHandler_Handle_ReturnsLentBooksSortedByLentAt(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	handlers := createAllCommandHandlers(t, wrapper)
	books := createTestBooks(t)
	readers := createTestReaders(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	addBooksToLibrary(t, handlers, books, fakeClock)
	registerTestReaders(t, handlers, readers, fakeClock)

	// Lend books at different times
	lendBookCmd1 := lendbookcopytoreader.BuildCommand(books.book1, readers.reader1, fakeClock.Add(time.Hour))
	err := handlers.lendBook.Handle(ctx, lendBookCmd1)
	assert.NoError(t, err, "Should lend book1")

	lendBookCmd2 := lendbookcopytoreader.BuildCommand(books.book2, readers.reader2, fakeClock.Add(2*time.Hour))
	err = handlers.lendBook.Handle(ctx, lendBookCmd2)
	assert.NoError(t, err, "Should lend book2")

	// act
	result, err := handlers.query.Handle(ctx)

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 books lent out")

	expectedLendings := []expectedLending{
		{BookID: books.book1.String(), ReaderID: readers.reader1.String(), LentAt: fakeClock.Add(time.Hour)},
		{BookID: books.book2.String(), ReaderID: readers.reader2.String(), LentAt: fakeClock.Add(2 * time.Hour)},
	}
	assertLendingsSortedCorrectly(t, result.Lendings, expectedLendings)
}

func Test_QueryHandler_Handle_ExcludesReturnedBooks(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	handlers := createAllCommandHandlers(t, wrapper)
	books := createTestBooks(t)
	readers := createTestReaders(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	addBooksToLibrary(t, handlers, books, fakeClock)
	registerTestReaders(t, handlers, readers, fakeClock)

	// Lend multiple books
	lendAllBooks(t, handlers, books, readers, fakeClock)

	// Return some books (book2 and book3) - should not appear in final results
	returnBookCmd2 := returnbookcopyfromreader.BuildCommand(books.book2, readers.reader2, fakeClock.Add(5*time.Hour))
	err := handlers.returnBook.Handle(ctx, returnBookCmd2)
	assert.NoError(t, err, "Should return book2")

	returnBookCmd3 := returnbookcopyfromreader.BuildCommand(books.book3, readers.reader1, fakeClock.Add(6*time.Hour))
	err = handlers.returnBook.Handle(ctx, returnBookCmd3)
	assert.NoError(t, err, "Should return book3")

	// act
	result, err := handlers.query.Handle(ctx)

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 books lent out (book2 and book3 were returned)")

	// Verify returned books are not in results
	for _, lending := range result.Lendings {
		assert.NotEqual(t, books.book2.String(), lending.BookID, "book2 should not be in results (was returned)")
		assert.NotEqual(t, books.book3.String(), lending.BookID, "book3 should not be in results (was returned)")
	}
}

func Test_QueryHandler_Handle_ReturnsEmptyResult_WhenNoBooksLent(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	handlers := createAllCommandHandlers(t, wrapper)
	books := createTestBooks(t)
	readers := createTestReaders(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange - add books and readers but don't lend any
	addBooksToLibrary(t, handlers, books, fakeClock)
	registerTestReaders(t, handlers, readers, fakeClock)

	// act
	result, err := handlers.query.Handle(ctx)

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 0, result.Count, "Should have 0 books lent out")
	assert.Len(t, result.Lendings, 0, "Should return empty Lendings slice")
}

func Test_QueryHandler_Handle_ReturnsCorrectLendingDetails(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	handlers := createAllCommandHandlers(t, wrapper)
	books := createTestBooks(t)
	readers := createTestReaders(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	addBooksToLibrary(t, handlers, books, fakeClock)
	registerTestReaders(t, handlers, readers, fakeClock)

	// Lend a specific book to a specific reader
	lendBookCmd := lendbookcopytoreader.BuildCommand(books.book1, readers.reader1, fakeClock.Add(time.Hour))
	err := handlers.lendBook.Handle(ctx, lendBookCmd)
	assert.NoError(t, err, "Should lend book1 to reader1")

	// act
	result, err := handlers.query.Handle(ctx)

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 1, result.Count, "Should have 1 book lent out")
	assert.Len(t, result.Lendings, 1, "Should return 1 lending in Lendings slice")

	lending := result.Lendings[0]
	assertSpecificLendingDetails(t, lending, books.book1.String(), readers.reader1.String(), fakeClock.Add(time.Hour))
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

func createAllCommandHandlers(t *testing.T, wrapper Wrapper) testHandlers {
	addBookHandler, err := addbookcopy.NewCommandHandler(wrapper.GetEventStore())
	assert.NoError(t, err, "Should create AddBookCopy handler")

	registerReaderHandler, err := registerreader.NewCommandHandler(wrapper.GetEventStore())
	assert.NoError(t, err, "Should create RegisterReader handler")

	lendBookHandler, err := lendbookcopytoreader.NewCommandHandler(wrapper.GetEventStore())
	assert.NoError(t, err, "Should create LendBook handler")

	returnBookHandler, err := returnbookcopyfromreader.NewCommandHandler(wrapper.GetEventStore())
	assert.NoError(t, err, "Should create ReturnBook handler")

	queryHandler, err := bookslentout.NewQueryHandler(wrapper.GetEventStore())
	assert.NoError(t, err, "Should create BooksLentOut query handler")

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
		book4: GivenUniqueID(t),
	}
}

func createTestReaders(t *testing.T) testReaders {
	return testReaders{
		reader1: GivenUniqueID(t),
		reader2: GivenUniqueID(t),
		reader3: GivenUniqueID(t),
	}
}

func addBooksToLibrary(t *testing.T, handlers testHandlers, books testBooks, fakeClock time.Time) {
	addBookCmd1 := addbookcopy.BuildCommand(books.book1, "978-1-098-10013-1", "Learning Domain-Driven Design", "Vlad Khononov", "First Edition", "O'Reilly Media, Inc.", 2021, fakeClock)
	err := handlers.addBook.Handle(context.Background(), addBookCmd1)
	assert.NoError(t, err, "Should add book1 to circulation")

	addBookCmd2 := addbookcopy.BuildCommand(books.book2, "978-0-201-63361-0", "Design Patterns", "Gang of Four", "First Edition", "Addison-Wesley", 1994, fakeClock.Add(time.Minute))
	err = handlers.addBook.Handle(context.Background(), addBookCmd2)
	assert.NoError(t, err, "Should add book2 to circulation")

	addBookCmd3 := addbookcopy.BuildCommand(books.book3, "978-0-134-75316-6", "Effective Java", "Joshua Bloch", "Third Edition", "Addison-Wesley", 2017, fakeClock.Add(2*time.Minute))
	err = handlers.addBook.Handle(context.Background(), addBookCmd3)
	assert.NoError(t, err, "Should add book3 to circulation")

	addBookCmd4 := addbookcopy.BuildCommand(books.book4, "978-0-321-12742-6", "Refactoring", "Martin Fowler", "Second Edition", "Addison-Wesley", 2018, fakeClock.Add(3*time.Minute))
	err = handlers.addBook.Handle(context.Background(), addBookCmd4)
	assert.NoError(t, err, "Should add book4 to circulation")
}

func registerTestReaders(t *testing.T, handlers testHandlers, readers testReaders, fakeClock time.Time) {
	registerReaderCmd1 := registerreader.BuildCommand(readers.reader1, "Test Reader 1", fakeClock.Add(4*time.Minute))
	err := handlers.registerReader.Handle(context.Background(), registerReaderCmd1)
	assert.NoError(t, err, "Should register reader1")

	registerReaderCmd2 := registerreader.BuildCommand(readers.reader2, "Test Reader 2", fakeClock.Add(5*time.Minute))
	err = handlers.registerReader.Handle(context.Background(), registerReaderCmd2)
	assert.NoError(t, err, "Should register reader2")

	registerReaderCmd3 := registerreader.BuildCommand(readers.reader3, "Test Reader 3", fakeClock.Add(6*time.Minute))
	err = handlers.registerReader.Handle(context.Background(), registerReaderCmd3)
	assert.NoError(t, err, "Should register reader3")
}

func lendAllBooks(t *testing.T, handlers testHandlers, books testBooks, readers testReaders, fakeClock time.Time) {
	lendBookCmd1 := lendbookcopytoreader.BuildCommand(books.book1, readers.reader1, fakeClock.Add(time.Hour))
	err := handlers.lendBook.Handle(context.Background(), lendBookCmd1)
	assert.NoError(t, err, "Should lend book1 to reader1")

	lendBookCmd2 := lendbookcopytoreader.BuildCommand(books.book2, readers.reader2, fakeClock.Add(2*time.Hour))
	err = handlers.lendBook.Handle(context.Background(), lendBookCmd2)
	assert.NoError(t, err, "Should lend book2 to reader2")

	lendBookCmd3 := lendbookcopytoreader.BuildCommand(books.book3, readers.reader1, fakeClock.Add(3*time.Hour))
	err = handlers.lendBook.Handle(context.Background(), lendBookCmd3)
	assert.NoError(t, err, "Should lend book3 to reader1")

	lendBookCmd4 := lendbookcopytoreader.BuildCommand(books.book4, readers.reader3, fakeClock.Add(4*time.Hour))
	err = handlers.lendBook.Handle(context.Background(), lendBookCmd4)
	assert.NoError(t, err, "Should lend book4 to reader3")
}

// Assertion helpers.
type expectedLending struct {
	BookID   string
	ReaderID string
	LentAt   time.Time
}

func assertLendingsSortedCorrectly(t *testing.T, lendings []bookslentout.LendingInfo, expected []expectedLending) {
	assert.Len(t, lendings, len(expected), "Should have correct number of lendings")

	for i, lending := range lendings {
		assert.Equal(t, expected[i].BookID, lending.BookID, "BookID should match for lending %d", i)
		assert.Equal(t, expected[i].ReaderID, lending.ReaderID, "ReaderID should match for lending %d", i)
		assert.Equal(t, expected[i].LentAt, lending.LentAt, "LentAt should match for lending %d", i)
	}

	// Verify timestamps are properly ordered (oldest first)
	for i := 0; i < len(lendings)-1; i++ {
		assert.True(t, lendings[i].LentAt.Before(lendings[i+1].LentAt) || lendings[i].LentAt.Equal(lendings[i+1].LentAt),
			"Lendings should be sorted by LentAt time (oldest first)")
	}
}

func assertSpecificLendingDetails(t *testing.T, lending bookslentout.LendingInfo, expectedBookID, expectedReaderID string, expectedLentAt time.Time) {
	assert.Equal(t, expectedBookID, lending.BookID, "Lending should have correct BookID")
	assert.Equal(t, expectedReaderID, lending.ReaderID, "Lending should have correct ReaderID")
	assert.Equal(t, expectedLentAt, lending.LentAt, "Lending should have correct LentAt time")
}
