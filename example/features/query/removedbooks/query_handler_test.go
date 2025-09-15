package removedbooks_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/addbookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/removebookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/removedbooks"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

type testHandlers struct {
	addBookCopy    addbookcopy.CommandHandler
	removeBookCopy removebookcopy.CommandHandler
	query          removedbooks.QueryHandler
}

type testBookIDs struct {
	book1ID, book2ID, book3ID, book4ID uuid.UUID
}

func Test_QueryHandler_Handle_ReturnsEmptyResult_WhenNoBooksRemoved(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	books := createTestBooks(t)
	addBooksToLibrary(t, handlers, books, fakeClock)

	// act
	result, err := handlers.query.Handle(ctx, removedbooks.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 0, result.Count, "Should have 0 removed books")
	assert.Len(t, result.Books, 0, "Should return empty Books slice")
}

func Test_QueryHandler_Handle_IncludesRemovedBooksSortedByRemovedAt(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	books := createTestBooks(t)
	addBooksToLibrary(t, handlers, books, fakeClock)

	removeBookCmd2 := removebookcopy.BuildCommand(books.book2ID, fakeClock.Add(20*time.Minute))
	_, err := handlers.removeBookCopy.Handle(ctx, removeBookCmd2)
	assert.NoError(t, err, "Should remove book2 from circulation")

	removeBookCmd4 := removebookcopy.BuildCommand(books.book4ID, fakeClock.Add(21*time.Minute))
	_, err = handlers.removeBookCopy.Handle(ctx, removeBookCmd4)
	assert.NoError(t, err, "Should remove book4 from circulation")

	// act
	result, err := handlers.query.Handle(ctx, removedbooks.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 removed books")
	expectedOrder := []string{books.book2ID.String(), books.book4ID.String()}
	assertRemovedBooksSortedCorrectly(t, result.Books, expectedOrder)
	assert.Equal(t, books.book2ID.String(), result.Books[0].BookID, "book2 should be in the results (was removed)")
	assert.Equal(t, books.book4ID.String(), result.Books[1].BookID, "book4 should be in the results (was removed)")
	assert.Equal(t, fakeClock.Add(20*time.Minute), result.Books[0].RemovedAt, "book2 should have correct removed time")
	assert.Equal(t, fakeClock.Add(21*time.Minute), result.Books[1].RemovedAt, "book4 should have correct removed time")
}

func Test_QueryHandler_Handle_ReturnsEmptyResult_WhenNoBooksAdded(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)

	// act
	result, err := handlers.query.Handle(ctx, removedbooks.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 0, result.Count, "Should have 0 removed books")
	assert.Len(t, result.Books, 0, "Should return empty Books slice")
}

func Test_QueryHandler_Handle_ReturnsAllRemovedBooks_WhenAllBooksAreRemoved(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	books := createTestBooks(t)
	addBooksToLibrary(t, handlers, books, fakeClock)

	removeBookCmd1 := removebookcopy.BuildCommand(books.book1ID, fakeClock.Add(10*time.Minute))
	_, err := handlers.removeBookCopy.Handle(ctx, removeBookCmd1)
	assert.NoError(t, err, "Should remove book1 from circulation")

	removeBookCmd2 := removebookcopy.BuildCommand(books.book2ID, fakeClock.Add(11*time.Minute))
	_, err = handlers.removeBookCopy.Handle(ctx, removeBookCmd2)
	assert.NoError(t, err, "Should remove book2 from circulation")

	removeBookCmd3 := removebookcopy.BuildCommand(books.book3ID, fakeClock.Add(12*time.Minute))
	_, err = handlers.removeBookCopy.Handle(ctx, removeBookCmd3)
	assert.NoError(t, err, "Should remove book3 from circulation")

	removeBookCmd4 := removebookcopy.BuildCommand(books.book4ID, fakeClock.Add(13*time.Minute))
	_, err = handlers.removeBookCopy.Handle(ctx, removeBookCmd4)
	assert.NoError(t, err, "Should remove book4 from circulation")

	// act
	result, err := handlers.query.Handle(ctx, removedbooks.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 4, result.Count, "Should have 4 removed books")
	assert.Len(t, result.Books, 4, "Should return 4 removed books")
	expectedOrder := []string{books.book1ID.String(), books.book2ID.String(), books.book3ID.String(), books.book4ID.String()}
	assertRemovedBooksSortedCorrectly(t, result.Books, expectedOrder)
}

func Test_QueryHandler_Handle_HandlesMixedAddAndRemoveOperations(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	books := createTestBooks(t)

	addBookCmd1 := addbookcopy.BuildCommand(books.book1ID, "Test Book 1", "Author 1", "ISBN1", "1st", "Publisher 1", 2020, fakeClock)
	_, err := handlers.addBookCopy.Handle(ctx, addBookCmd1)
	assert.NoError(t, err, "Should add book1")

	addBookCmd2 := addbookcopy.BuildCommand(books.book2ID, "Test Book 2", "Author 2", "ISBN2", "1st", "Publisher 2", 2021, fakeClock.Add(time.Minute))
	_, err = handlers.addBookCopy.Handle(ctx, addBookCmd2)
	assert.NoError(t, err, "Should add book2")

	removeBookCmd1 := removebookcopy.BuildCommand(books.book1ID, fakeClock.Add(2*time.Minute))
	_, err = handlers.removeBookCopy.Handle(ctx, removeBookCmd1)
	assert.NoError(t, err, "Should remove book1 from circulation")

	addBookCmd3 := addbookcopy.BuildCommand(books.book3ID, "Test Book 3", "Author 3", "ISBN3", "1st", "Publisher 3", 2022, fakeClock.Add(3*time.Minute))
	_, err = handlers.addBookCopy.Handle(ctx, addBookCmd3)
	assert.NoError(t, err, "Should add book3")

	removeBookCmd2 := removebookcopy.BuildCommand(books.book2ID, fakeClock.Add(4*time.Minute))
	_, err = handlers.removeBookCopy.Handle(ctx, removeBookCmd2)
	assert.NoError(t, err, "Should remove book2 from circulation")

	// act
	result, err := handlers.query.Handle(ctx, removedbooks.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 removed books (book1 and book2)")
	expectedOrder := []string{books.book1ID.String(), books.book2ID.String()}
	assertRemovedBooksSortedCorrectly(t, result.Books, expectedOrder)
	assert.Equal(t, books.book1ID.String(), result.Books[0].BookID, "book1 should be first (removed earlier)")
	assert.Equal(t, books.book2ID.String(), result.Books[1].BookID, "book2 should be second (removed later)")
	assert.Equal(t, fakeClock.Add(2*time.Minute), result.Books[0].RemovedAt, "book1 removed time")
	assert.Equal(t, fakeClock.Add(4*time.Minute), result.Books[1].RemovedAt, "book2 removed time")
}

func Test_QueryHandler_Handle_IdempotentRemoval_OnlyShowsBookOnce(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	books := createTestBooks(t)
	addBookCmd1 := addbookcopy.BuildCommand(books.book1ID, "Test Book 1", "Author 1", "ISBN1", "1st", "Publisher 1", 2020, fakeClock)
	_, err := handlers.addBookCopy.Handle(ctx, addBookCmd1)
	assert.NoError(t, err, "Should add book1")

	removeBookCmd1 := removebookcopy.BuildCommand(books.book1ID, fakeClock.Add(10*time.Minute))
	_, err = handlers.removeBookCopy.Handle(ctx, removeBookCmd1)
	assert.NoError(t, err, "Should remove book1 from circulation (first time)")

	removeBookCmd2 := removebookcopy.BuildCommand(books.book1ID, fakeClock.Add(15*time.Minute))
	_, err = handlers.removeBookCopy.Handle(ctx, removeBookCmd2)
	assert.NoError(t, err, "Should handle idempotent removal (second time)")

	// act
	result, err := handlers.query.Handle(ctx, removedbooks.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 1, result.Count, "Should have exactly 1 removed book (no duplicates)")
	assert.Len(t, result.Books, 1, "Should return exactly 1 removed book")
	assert.Equal(t, books.book1ID.String(), result.Books[0].BookID, "Should be book1")
	assert.Equal(t, fakeClock.Add(10*time.Minute), result.Books[0].RemovedAt, "Should show first removal time")
}

func Test_QueryHandler_Handle_WithStrongConsistency_WorksCorrectly(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithStrongConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)

	// act
	query := removedbooks.BuildQuery()
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
	removeBookCopyHandler := removebookcopy.NewCommandHandler(wrapper.GetEventStore())
	queryHandler := removedbooks.NewQueryHandler(wrapper.GetEventStore())

	return testHandlers{
		addBookCopy:    addBookCopyHandler,
		removeBookCopy: removeBookCopyHandler,
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

func addBooksToLibrary(
	t *testing.T,
	handlers testHandlers,
	books testBookIDs,
	fakeClock time.Time,
) {
	t.Helper()

	addBookCmd1 := addbookcopy.BuildCommand(books.book1ID, "Test Book 1", "Author 1", "ISBN1", "1st", "Publisher 1", 2020, fakeClock)
	_, err := handlers.addBookCopy.Handle(context.Background(), addBookCmd1)
	assert.NoError(t, err, "Should add book1")

	addBookCmd2 := addbookcopy.BuildCommand(books.book2ID, "Test Book 2", "Author 2", "ISBN2", "1st", "Publisher 2", 2021, fakeClock.Add(time.Minute))
	_, err = handlers.addBookCopy.Handle(context.Background(), addBookCmd2)
	assert.NoError(t, err, "Should add book2")

	addBookCmd3 := addbookcopy.BuildCommand(books.book3ID, "Test Book 3", "Author 3", "ISBN3", "1st", "Publisher 3", 2022, fakeClock.Add(2*time.Minute))
	_, err = handlers.addBookCopy.Handle(context.Background(), addBookCmd3)
	assert.NoError(t, err, "Should add book3")

	addBookCmd4 := addbookcopy.BuildCommand(books.book4ID, "Test Book 4", "Author 4", "ISBN4", "1st", "Publisher 4", 2023, fakeClock.Add(3*time.Minute))
	_, err = handlers.addBookCopy.Handle(context.Background(), addBookCmd4)
	assert.NoError(t, err, "Should add book4")
}

// Assertion helpers

func assertRemovedBooksSortedCorrectly(
	t *testing.T,
	books []removedbooks.BookInfo,
	expectedOrder []string,
) {
	t.Helper()

	assert.Len(t, books, len(expectedOrder), "Should have correct number of removed books")

	for i, book := range books {
		assert.Equal(t, expectedOrder[i], book.BookID, "Removed books should be sorted by RemovedAt time")
	}

	for i := 0; i < len(books)-1; i++ {
		assert.True(t, books[i].RemovedAt.Before(books[i+1].RemovedAt) || books[i].RemovedAt.Equal(books[i+1].RemovedAt),
			"Removed books should be sorted by RemovedAt time (oldest first)")
	}
}
