package bookslentout_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/addbookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/lendbookcopytoreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/registerreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/returnbookcopyfromreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/bookslentout"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

//nolint:funlen
func Test_QueryHandler_Handle_ReturnsLentBooks_WhenBooksAreLentToReaders(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	wrapper := CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	CleanUp(t, wrapper)
	readerID1 := GivenUniqueID(t)
	readerID2 := GivenUniqueID(t)
	readerID3 := GivenUniqueID(t)
	bookID1 := GivenUniqueID(t)
	bookID2 := GivenUniqueID(t)
	bookID3 := GivenUniqueID(t)
	bookID4 := GivenUniqueID(t)
	fakeClock := time.Unix(0, 0).UTC()

	addBookHandler, err := addbookcopy.NewCommandHandler(es)
	assert.NoError(t, err, "Should create AddBookCopy handler")

	registerReaderHandler, err := registerreader.NewCommandHandler(es)
	assert.NoError(t, err, "Should create RegisterReader handler")

	lendBookHandler, err := lendbookcopytoreader.NewCommandHandler(es)
	assert.NoError(t, err, "Should create LendBook handler")

	returnBookHandler, err := returnbookcopyfromreader.NewCommandHandler(es)
	assert.NoError(t, err, "Should create ReturnBook handler")

	queryHandler, err := bookslentout.NewQueryHandler(es)
	assert.NoError(t, err, "Should create BooksLentOut query handler")

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

	addBookCmd4 := addbookcopy.BuildCommand(bookID4, "978-0-321-12742-6", "Refactoring", "Martin Fowler", "Second Edition", "Addison-Wesley", 2018, fakeClock.Add(3*time.Minute))
	err = addBookHandler.Handle(ctxWithTimeout, addBookCmd4)
	assert.NoError(t, err, "Should add book4 to circulation")

	// Register multiple readers
	registerReaderCmd1 := registerreader.BuildCommand(readerID1, "Test Reader 1", fakeClock.Add(4*time.Minute))
	err = registerReaderHandler.Handle(ctxWithTimeout, registerReaderCmd1)
	assert.NoError(t, err, "Should register reader1")

	registerReaderCmd2 := registerreader.BuildCommand(readerID2, "Test Reader 2", fakeClock.Add(5*time.Minute))
	err = registerReaderHandler.Handle(ctxWithTimeout, registerReaderCmd2)
	assert.NoError(t, err, "Should register reader2")

	registerReaderCmd3 := registerreader.BuildCommand(readerID3, "Test Reader 3", fakeClock.Add(6*time.Minute))
	err = registerReaderHandler.Handle(ctxWithTimeout, registerReaderCmd3)
	assert.NoError(t, err, "Should register reader3")

	// Lend multiple books to different readers
	lendBookCmd1 := lendbookcopytoreader.BuildCommand(bookID1, readerID1, fakeClock.Add(time.Hour))
	err = lendBookHandler.Handle(ctxWithTimeout, lendBookCmd1)
	assert.NoError(t, err, "Should lend book1 to reader1")

	lendBookCmd2 := lendbookcopytoreader.BuildCommand(bookID2, readerID2, fakeClock.Add(2*time.Hour))
	err = lendBookHandler.Handle(ctxWithTimeout, lendBookCmd2)
	assert.NoError(t, err, "Should lend book2 to reader2")

	lendBookCmd3 := lendbookcopytoreader.BuildCommand(bookID3, readerID1, fakeClock.Add(3*time.Hour))
	err = lendBookHandler.Handle(ctxWithTimeout, lendBookCmd3)
	assert.NoError(t, err, "Should lend book3 to reader1")

	lendBookCmd4 := lendbookcopytoreader.BuildCommand(bookID4, readerID3, fakeClock.Add(4*time.Hour))
	err = lendBookHandler.Handle(ctxWithTimeout, lendBookCmd4)
	assert.NoError(t, err, "Should lend book4 to reader3")

	// Return some books (book2 and book3) - should not appear in final results
	returnBookCmd2 := returnbookcopyfromreader.BuildCommand(bookID2, readerID2, fakeClock.Add(5*time.Hour))
	err = returnBookHandler.Handle(ctxWithTimeout, returnBookCmd2)
	assert.NoError(t, err, "Should return book2")

	returnBookCmd3 := returnbookcopyfromreader.BuildCommand(bookID3, readerID1, fakeClock.Add(6*time.Hour))
	err = returnBookHandler.Handle(ctxWithTimeout, returnBookCmd3)
	assert.NoError(t, err, "Should return book3")

	// act
	result, err := queryHandler.Handle(ctxWithTimeout)

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 books lent out (book2 and book3 were returned)")
	assert.Len(t, result.Lendings, 2, "Should return 2 lendings in Lendings slice")

	if len(result.Lendings) >= 2 {
		// Should be sorted by LentAt (oldest first)
		assert.Equal(t, bookID1.String(), result.Lendings[0].BookID, "First lending should be book1")
		assert.Equal(t, readerID1.String(), result.Lendings[0].ReaderID, "First lending should be to reader1")
		assert.Equal(t, fakeClock.Add(time.Hour), result.Lendings[0].LentAt, "Should include correct lending time for book1")

		assert.Equal(t, bookID4.String(), result.Lendings[1].BookID, "Second lending should be book4")
		assert.Equal(t, readerID3.String(), result.Lendings[1].ReaderID, "Second lending should be to reader3")
		assert.Equal(t, fakeClock.Add(4*time.Hour), result.Lendings[1].LentAt, "Should include correct lending time for book4")

		// Verify that returned books are not in results and only correct ReaderIDs are present
		for _, lending := range result.Lendings {
			assert.NotEqual(t, bookID2.String(), lending.BookID, "book2 should not be in results (was returned)")
			assert.NotEqual(t, bookID3.String(), lending.BookID, "book3 should not be in results (was returned)")
			assert.Contains(t, []string{readerID1.String(), readerID3.String()}, lending.ReaderID, "ReaderID should be one of the readers with unreturned books")
		}
	}
}
