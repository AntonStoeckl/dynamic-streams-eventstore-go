package bookslentbyreader_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/addbookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/lendbookcopytoreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/registerreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/returnbookcopyfromreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/bookslentbyreader"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

//nolint:funlen
func Test_QueryHandler_Handle_ReturnsBooksWithMetadata_WhenReaderHasBorrowedBooks(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	wrapper := CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	CleanUp(t, wrapper)
	readerID := GivenUniqueID(t)
	bookID1 := GivenUniqueID(t)
	bookID2 := GivenUniqueID(t)
	bookID3 := GivenUniqueID(t)
	fakeClock := time.Unix(0, 0).UTC()

	addBookHandler, err := addbookcopy.NewCommandHandler(es)
	assert.NoError(t, err, "Should create AddBookCopy handler")

	registerReaderHandler, err := registerreader.NewCommandHandler(es)
	assert.NoError(t, err, "Should create RegisterReader handler")

	lendBookHandler, err := lendbookcopytoreader.NewCommandHandler(es)
	assert.NoError(t, err, "Should create LendBook handler")

	queryHandler, err := bookslentbyreader.NewQueryHandler(es)
	assert.NoError(t, err, "Should create BooksLentByReader query handler")

	// Add multiple books
	addBookCmd1 := addbookcopy.BuildCommand(bookID1, "978-1-098-10013-1", "Learning Domain-Driven Design", "Vlad Khononov", "First Edition", "O'Reilly Media, Inc.", 2021, fakeClock)
	err = addBookHandler.Handle(ctxWithTimeout, addBookCmd1)
	assert.NoError(t, err, "Should add book1 to circulation")

	addBookCmd2 := addbookcopy.BuildCommand(bookID2, "978-0-201-63361-0", "Design Patterns", "Gang of Four", "First Edition", "Addison-Wesley", 1994, fakeClock.Add(time.Minute))
	err = addBookHandler.Handle(ctxWithTimeout, addBookCmd2)
	assert.NoError(t, err, "Should add book2 to circulation")

	addBookCmd3 := addbookcopy.BuildCommand(bookID3, "978-0-134-75316-6", "Effective Java", "Joshua Bloch", "Third Edition", "Addison-Wesley", 2017, fakeClock.Add(2*time.Minute))
	err = addBookHandler.Handle(ctxWithTimeout, addBookCmd3)
	assert.NoError(t, err, "Should add book3 to circulation")

	registerReaderCmd := registerreader.BuildCommand(readerID, "Test Reader", fakeClock.Add(3*time.Minute))
	err = registerReaderHandler.Handle(ctxWithTimeout, registerReaderCmd)
	assert.NoError(t, err, "Should register reader")

	// Lend multiple books
	lendBookCmd1 := lendbookcopytoreader.BuildCommand(bookID1, readerID, fakeClock.Add(time.Hour))
	err = lendBookHandler.Handle(ctxWithTimeout, lendBookCmd1)
	assert.NoError(t, err, "Should lend book1 to reader")

	lendBookCmd2 := lendbookcopytoreader.BuildCommand(bookID2, readerID, fakeClock.Add(2*time.Hour))
	err = lendBookHandler.Handle(ctxWithTimeout, lendBookCmd2)
	assert.NoError(t, err, "Should lend book2 to reader")

	lendBookCmd3 := lendbookcopytoreader.BuildCommand(bookID3, readerID, fakeClock.Add(3*time.Hour))
	err = lendBookHandler.Handle(ctxWithTimeout, lendBookCmd3)
	assert.NoError(t, err, "Should lend book3 to reader")

	// Return one book (book2) - should not appear in final results
	returnBookHandler, err := returnbookcopyfromreader.NewCommandHandler(es)
	assert.NoError(t, err, "Should create ReturnBook handler")

	returnBookCmd := returnbookcopyfromreader.BuildCommand(bookID2, readerID, fakeClock.Add(4*time.Hour))
	err = returnBookHandler.Handle(ctxWithTimeout, returnBookCmd)
	assert.NoError(t, err, "Should return book2")

	// act
	query := bookslentbyreader.BuildQuery(readerID)
	result, err := queryHandler.Handle(ctxWithTimeout, query)

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, readerID.String(), result.ReaderID, "Should return correct ReaderID")
	assert.Equal(t, 2, result.Count, "Reader should have 2 borrowed books (book2 was returned)")
	assert.Len(t, result.Books, 2, "Should return 2 books in Books slice")

	if len(result.Books) >= 2 {
		// Should be sorted by LentAt (oldest first)
		assert.Equal(t, bookID1.String(), result.Books[0].BookID, "First book should be book1")
		assert.Equal(t, fakeClock.Add(time.Hour), result.Books[0].LentAt, "Should include correct lending time for book1")

		assert.Equal(t, bookID3.String(), result.Books[1].BookID, "Second book should be book3")
		assert.Equal(t, fakeClock.Add(3*time.Hour), result.Books[1].LentAt, "Should include correct lending time for book3")
	}
}
