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

type testBooks struct {
	book1, book2, book3, book4 uuid.UUID
}

func Test_QueryHandler_Handle_ReturnsEmptyResult_WhenNoBooksRemoved(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	books := createTestBooks(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange - add books but don't remove any
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

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	books := createTestBooks(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	addBooksToLibrary(t, handlers, books, fakeClock)

	// Remove book2 and book4 at different times
	removeBookCmd2 := removebookcopy.BuildCommand(books.book2, fakeClock.Add(20*time.Minute))
	_, err := handlers.removeBookCopy.Handle(ctx, removeBookCmd2)
	assert.NoError(t, err, "Should remove book2 from circulation")

	removeBookCmd4 := removebookcopy.BuildCommand(books.book4, fakeClock.Add(21*time.Minute))
	_, err = handlers.removeBookCopy.Handle(ctx, removeBookCmd4)
	assert.NoError(t, err, "Should remove book4 from circulation")

	// act
	result, err := handlers.query.Handle(ctx, removedbooks.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 removed books")

	// Should be sorted by RemovedAt (oldest first)
	expectedOrder := []string{books.book2.String(), books.book4.String()}
	assertRemovedBooksSortedCorrectly(t, result.Books, expectedOrder)

	// Verify removed books ARE in the results
	assert.Equal(t, books.book2.String(), result.Books[0].BookID, "book2 should be in the results (was removed)")
	assert.Equal(t, books.book4.String(), result.Books[1].BookID, "book4 should be in the results (was removed)")

	// Verify RemovedAt times are correct
	assert.Equal(t, fakeClock.Add(20*time.Minute), result.Books[0].RemovedAt, "book2 should have correct removed time")
	assert.Equal(t, fakeClock.Add(21*time.Minute), result.Books[1].RemovedAt, "book4 should have correct removed time")
}

func Test_QueryHandler_Handle_ReturnsEmptyResult_WhenNoBooksAdded(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)

	// act - no books added
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

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	books := createTestBooks(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange - add books and then remove them all
	addBooksToLibrary(t, handlers, books, fakeClock)

	// Remove all books at different times
	removeBookCmd1 := removebookcopy.BuildCommand(books.book1, fakeClock.Add(10*time.Minute))
	_, err := handlers.removeBookCopy.Handle(ctx, removeBookCmd1)
	assert.NoError(t, err, "Should remove book1 from circulation")

	removeBookCmd2 := removebookcopy.BuildCommand(books.book2, fakeClock.Add(11*time.Minute))
	_, err = handlers.removeBookCopy.Handle(ctx, removeBookCmd2)
	assert.NoError(t, err, "Should remove book2 from circulation")

	removeBookCmd3 := removebookcopy.BuildCommand(books.book3, fakeClock.Add(12*time.Minute))
	_, err = handlers.removeBookCopy.Handle(ctx, removeBookCmd3)
	assert.NoError(t, err, "Should remove book3 from circulation")

	removeBookCmd4 := removebookcopy.BuildCommand(books.book4, fakeClock.Add(13*time.Minute))
	_, err = handlers.removeBookCopy.Handle(ctx, removeBookCmd4)
	assert.NoError(t, err, "Should remove book4 from circulation")

	// act
	result, err := handlers.query.Handle(ctx, removedbooks.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 4, result.Count, "Should have 4 removed books")
	assert.Len(t, result.Books, 4, "Should return 4 removed books")

	// Verify all books are returned in the correct order (sorted by RemovedAt)
	expectedOrder := []string{books.book1.String(), books.book2.String(), books.book3.String(), books.book4.String()}
	assertRemovedBooksSortedCorrectly(t, result.Books, expectedOrder)
}

func Test_QueryHandler_Handle_HandlesMixedAddAndRemoveOperations(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	books := createTestBooks(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange - complex scenario with add/remove operations

	// Add book1 and book2
	addBookCmd1 := addbookcopy.BuildCommand(books.book1, "Test Book 1", "Author 1", "ISBN1", "1st", "Publisher 1", 2020, fakeClock)
	_, err := handlers.addBookCopy.Handle(ctx, addBookCmd1)
	assert.NoError(t, err, "Should add book1")

	addBookCmd2 := addbookcopy.BuildCommand(books.book2, "Test Book 2", "Author 2", "ISBN2", "1st", "Publisher 2", 2021, fakeClock.Add(time.Minute))
	_, err = handlers.addBookCopy.Handle(ctx, addBookCmd2)
	assert.NoError(t, err, "Should add book2")

	// Remove book1
	removeBookCmd1 := removebookcopy.BuildCommand(books.book1, fakeClock.Add(2*time.Minute))
	_, err = handlers.removeBookCopy.Handle(ctx, removeBookCmd1)
	assert.NoError(t, err, "Should remove book1 from circulation")

	// Add book3 (but don't remove)
	addBookCmd3 := addbookcopy.BuildCommand(books.book3, "Test Book 3", "Author 3", "ISBN3", "1st", "Publisher 3", 2022, fakeClock.Add(3*time.Minute))
	_, err = handlers.addBookCopy.Handle(ctx, addBookCmd3)
	assert.NoError(t, err, "Should add book3")

	// Remove book2 later
	removeBookCmd2 := removebookcopy.BuildCommand(books.book2, fakeClock.Add(4*time.Minute))
	_, err = handlers.removeBookCopy.Handle(ctx, removeBookCmd2)
	assert.NoError(t, err, "Should remove book2 from circulation")

	// act
	result, err := handlers.query.Handle(ctx, removedbooks.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 removed books (book1 and book2)")

	// Should be sorted by RemovedAt (oldest first)
	expectedOrder := []string{books.book1.String(), books.book2.String()}
	assertRemovedBooksSortedCorrectly(t, result.Books, expectedOrder)

	// Verify specific removed books and their times
	assert.Equal(t, books.book1.String(), result.Books[0].BookID, "book1 should be first (removed earlier)")
	assert.Equal(t, books.book2.String(), result.Books[1].BookID, "book2 should be second (removed later)")
	assert.Equal(t, fakeClock.Add(2*time.Minute), result.Books[0].RemovedAt, "book1 removed time")
	assert.Equal(t, fakeClock.Add(4*time.Minute), result.Books[1].RemovedAt, "book2 removed time")
}

func Test_QueryHandler_Handle_IdempotentRemoval_OnlyShowsBookOnce(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	books := createTestBooks(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange - add book1
	addBookCmd1 := addbookcopy.BuildCommand(books.book1, "Test Book 1", "Author 1", "ISBN1", "1st", "Publisher 1", 2020, fakeClock)
	_, err := handlers.addBookCopy.Handle(ctx, addBookCmd1)
	assert.NoError(t, err, "Should add book1")

	// Remove book1 multiple times (idempotent operations)
	removeBookCmd1 := removebookcopy.BuildCommand(books.book1, fakeClock.Add(10*time.Minute))
	_, err = handlers.removeBookCopy.Handle(ctx, removeBookCmd1)
	assert.NoError(t, err, "Should remove book1 from circulation (first time)")

	// Try to remove the same book again (should be idempotent)
	removeBookCmd2 := removebookcopy.BuildCommand(books.book1, fakeClock.Add(15*time.Minute))
	_, err = handlers.removeBookCopy.Handle(ctx, removeBookCmd2)
	assert.NoError(t, err, "Should handle idempotent removal (second time)")

	// act
	result, err := handlers.query.Handle(ctx, removedbooks.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 1, result.Count, "Should have exactly 1 removed book (no duplicates)")
	assert.Len(t, result.Books, 1, "Should return exactly 1 removed book")

	// Verify it's the correct book with the first removal time
	assert.Equal(t, books.book1.String(), result.Books[0].BookID, "Should be book1")
	assert.Equal(t, fakeClock.Add(10*time.Minute), result.Books[0].RemovedAt, "Should show first removal time")
}

func Test_QueryHandler_Handle_WithStrongConsistency_WorksCorrectly(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use strong consistency (non-default for query handlers) to verify it still works
	ctx = eventstore.WithStrongConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)

	// act
	query := removedbooks.BuildQuery()
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
	removeBookCopyHandler := removebookcopy.NewCommandHandler(wrapper.GetEventStore())

	queryHandler, err := removedbooks.NewQueryHandler(wrapper.GetEventStore())
	assert.NoError(t, err, "Should create RemovedBooks query handler")

	return testHandlers{
		addBookCopy:    addBookCopyHandler,
		removeBookCopy: removeBookCopyHandler,
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

func addBooksToLibrary(t *testing.T, handlers testHandlers, books testBooks, fakeClock time.Time) {
	addBookCmd1 := addbookcopy.BuildCommand(books.book1, "Test Book 1", "Author 1", "ISBN1", "1st", "Publisher 1", 2020, fakeClock)
	_, err := handlers.addBookCopy.Handle(context.Background(), addBookCmd1)
	assert.NoError(t, err, "Should add book1")

	addBookCmd2 := addbookcopy.BuildCommand(books.book2, "Test Book 2", "Author 2", "ISBN2", "1st", "Publisher 2", 2021, fakeClock.Add(time.Minute))
	_, err = handlers.addBookCopy.Handle(context.Background(), addBookCmd2)
	assert.NoError(t, err, "Should add book2")

	addBookCmd3 := addbookcopy.BuildCommand(books.book3, "Test Book 3", "Author 3", "ISBN3", "1st", "Publisher 3", 2022, fakeClock.Add(2*time.Minute))
	_, err = handlers.addBookCopy.Handle(context.Background(), addBookCmd3)
	assert.NoError(t, err, "Should add book3")

	addBookCmd4 := addbookcopy.BuildCommand(books.book4, "Test Book 4", "Author 4", "ISBN4", "1st", "Publisher 4", 2023, fakeClock.Add(3*time.Minute))
	_, err = handlers.addBookCopy.Handle(context.Background(), addBookCmd4)
	assert.NoError(t, err, "Should add book4")
}

// Assertion helpers.
func assertRemovedBooksSortedCorrectly(t *testing.T, books []removedbooks.BookInfo, expectedOrder []string) {
	assert.Len(t, books, len(expectedOrder), "Should have correct number of removed books")

	for i, book := range books {
		assert.Equal(t, expectedOrder[i], book.BookID, "Removed books should be sorted by RemovedAt time")
	}

	// Verify timestamps are properly ordered
	for i := 0; i < len(books)-1; i++ {
		assert.True(t, books[i].RemovedAt.Before(books[i+1].RemovedAt) || books[i].RemovedAt.Equal(books[i+1].RemovedAt),
			"Removed books should be sorted by RemovedAt time (oldest first)")
	}
}
