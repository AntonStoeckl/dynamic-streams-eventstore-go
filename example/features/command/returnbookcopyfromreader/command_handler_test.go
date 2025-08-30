package returnbookcopyfromreader_test

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
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

//nolint:funlen
func Test_CommandHandler_Handle_Success(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	ctx = eventstore.WithStrongConsistency(ctx)
	defer cleanup()

	returnBookHandler := createReturnBookHandler(t, wrapper)
	addBookHandler := createAddBookHandler(t, wrapper)
	lendBookHandler := createLendBookHandler(t, wrapper)
	registerReaderHandler := createRegisterReaderHandler(t, wrapper)

	fakeClock := time.Unix(0, 0).UTC()
	bookID := uuid.New()
	readerID := uuid.New()

	// arrange
	addBookCmd := addbookcopy.BuildCommand(
		bookID,
		"978-1-098-10013-1",
		"Learning Domain-Driven Design",
		"Vlad Khononov",
		"First Edition",
		"O'Reilly Media, Inc.",
		2021,
		fakeClock.Add(time.Hour),
	)
	_, err := addBookHandler.Handle(ctx, addBookCmd)
	assert.NoError(t, err, "Should successfully add book to circulation")

	registerReaderCmd := registerreader.BuildCommand(
		readerID,
		"John Doe",
		fakeClock.Add(2*time.Hour),
	)
	_, err = registerReaderHandler.Handle(ctx, registerReaderCmd)
	assert.NoError(t, err, "Should successfully register reader")

	lendBookCmd := lendbookcopytoreader.BuildCommand(
		bookID,
		readerID,
		fakeClock.Add(3*time.Hour),
	)
	_, err = lendBookHandler.Handle(ctx, lendBookCmd)
	assert.NoError(t, err, "Should successfully lend book to reader")

	// act
	returnBookCmd := returnbookcopyfromreader.BuildCommand(
		bookID,
		readerID,
		fakeClock.Add(4*time.Hour),
	)
	result, err := returnBookHandler.Handle(ctx, returnBookCmd)

	// assert
	assert.NoError(t, err, "Should successfully return book from reader")
	assertNonIdempotentResult(t, result)
	verifyEventsPersisted(ctx, t, wrapper, bookID, readerID)
}

//nolint:funlen
func Test_CommandHandler_Handle_Error_BookNotLentToReader(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	ctx = eventstore.WithStrongConsistency(ctx)
	defer cleanup()

	returnBookHandler := createReturnBookHandler(t, wrapper)
	addBookHandler := createAddBookHandler(t, wrapper)
	registerReaderHandler := createRegisterReaderHandler(t, wrapper)

	fakeClock := time.Unix(0, 0).UTC()
	bookID := uuid.New()
	readerID := uuid.New()

	// arrange
	addBookCmd := addbookcopy.BuildCommand(
		bookID,
		"978-1-098-10013-1",
		"Learning Domain-Driven Design",
		"Vlad Khononov",
		"First Edition",
		"O'Reilly Media, Inc.",
		2021,
		fakeClock.Add(time.Hour),
	)
	_, err := addBookHandler.Handle(ctx, addBookCmd)
	assert.NoError(t, err, "Should successfully add book to circulation")

	registerReaderCmd := registerreader.BuildCommand(
		readerID,
		"John Doe",
		fakeClock.Add(2*time.Hour),
	)
	_, err = registerReaderHandler.Handle(ctx, registerReaderCmd)
	assert.NoError(t, err, "Should successfully register reader")

	// act
	returnBookCmd := returnbookcopyfromreader.BuildCommand(
		bookID,
		readerID,
		fakeClock.Add(3*time.Hour),
	)
	_, err = returnBookHandler.Handle(ctx, returnBookCmd)

	// assert
	assert.Error(t, err, "Should fail to return book that is not lent to this reader")
	assert.ErrorContains(t, err, "book is not lent to this reader", "Error should mention book is not lent to this reader")
	verifyErrorEventPersisted(ctx, t, wrapper, bookID, readerID)
}

//nolint:funlen
func Test_CommandHandler_Handle_Idempotent_BookAlreadyReturned(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	ctx = eventstore.WithStrongConsistency(ctx)
	defer cleanup()

	returnBookHandler := createReturnBookHandler(t, wrapper)
	addBookHandler := createAddBookHandler(t, wrapper)
	lendBookHandler := createLendBookHandler(t, wrapper)
	registerReaderHandler := createRegisterReaderHandler(t, wrapper)

	fakeClock := time.Unix(0, 0).UTC()
	bookID := uuid.New()
	readerID := uuid.New()

	// arrange
	addBookCmd := addbookcopy.BuildCommand(
		bookID,
		"978-1-098-10013-1",
		"Learning Domain-Driven Design",
		"Vlad Khononov",
		"First Edition",
		"O'Reilly Media, Inc.",
		2021,
		fakeClock.Add(time.Hour),
	)
	_, err := addBookHandler.Handle(ctx, addBookCmd)
	assert.NoError(t, err, "Should successfully add book to circulation")

	registerReaderCmd := registerreader.BuildCommand(
		readerID,
		"John Doe",
		fakeClock.Add(2*time.Hour),
	)
	_, err = registerReaderHandler.Handle(ctx, registerReaderCmd)
	assert.NoError(t, err, "Should successfully register reader")

	lendBookCmd := lendbookcopytoreader.BuildCommand(
		bookID,
		readerID,
		fakeClock.Add(3*time.Hour),
	)
	_, err = lendBookHandler.Handle(ctx, lendBookCmd)
	assert.NoError(t, err, "Should successfully lend book to reader")

	returnBookCmd := returnbookcopyfromreader.BuildCommand(
		bookID,
		readerID,
		fakeClock.Add(4*time.Hour),
	)
	_, err = returnBookHandler.Handle(ctx, returnBookCmd)
	assert.NoError(t, err, "Should successfully return book from reader first time")

	// act
	result, err := returnBookHandler.Handle(ctx, returnBookCmd)

	// assert
	assert.NoError(t, err, "Should succeed (idempotent) when book already returned from reader")
	assertIdempotentResult(t, result)
	verifyNoNewEventsAppended(ctx, t, wrapper, bookID, readerID)
}

// Test helper functions

func setupTestEnvironment(t *testing.T) (context.Context, Wrapper, func()) {
	t.Helper()

	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	wrapper := CreateWrapperWithTestConfig(t)

	cleanup := func() {
		cancel()
		wrapper.Close()
	}

	CleanUp(t, wrapper)

	return ctxWithTimeout, wrapper, cleanup
}

func createReturnBookHandler(t *testing.T, wrapper Wrapper) returnbookcopyfromreader.CommandHandler {
	t.Helper()

	handler := returnbookcopyfromreader.NewCommandHandler(wrapper.GetEventStore())

	return handler
}

func createAddBookHandler(t *testing.T, wrapper Wrapper) addbookcopy.CommandHandler {
	t.Helper()

	handler := addbookcopy.NewCommandHandler(wrapper.GetEventStore())

	return handler
}

func createLendBookHandler(t *testing.T, wrapper Wrapper) lendbookcopytoreader.CommandHandler {
	t.Helper()

	handler := lendbookcopytoreader.NewCommandHandler(wrapper.GetEventStore())

	return handler
}

func createRegisterReaderHandler(t *testing.T, wrapper Wrapper) registerreader.CommandHandler {
	t.Helper()

	handler := registerreader.NewCommandHandler(wrapper.GetEventStore())

	return handler
}

func verifyEventsPersisted(ctx context.Context, t *testing.T, wrapper Wrapper, bookID, readerID uuid.UUID) {
	t.Helper()

	es := wrapper.GetEventStore()
	filter := returnbookcopyfromreader.BuildEventFilter(bookID, readerID)

	events, _, err := es.Query(ctx, filter)
	assert.NoError(t, err, "Should query events successfully")

	assert.GreaterOrEqual(t, len(events), 4, "Should have at least 4 events persisted")

	if len(events) > 0 {
		lastEvent := events[len(events)-1]
		assert.Equal(t, "BookCopyReturnedByReader", lastEvent.EventType, "Last event should be BookCopyReturnedByReader")
	}
}

func verifyErrorEventPersisted(ctx context.Context, t *testing.T, wrapper Wrapper, bookID, readerID uuid.UUID) {
	t.Helper()

	es := wrapper.GetEventStore()

	errorFilter := eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf("ReturningBookFromReaderFailed").
		AndAnyPredicateOf(
			eventstore.P("BookID", bookID.String()),
			eventstore.P("ReaderID", readerID.String()),
		).
		Finalize()

	errorEvents, _, err := es.Query(ctx, errorFilter)
	assert.NoError(t, err, "Should query error events successfully")

	assert.Greater(t, len(errorEvents), 0, "Should have error event persisted")

	errorEvent := errorEvents[len(errorEvents)-1]
	assert.Equal(t, "ReturningBookFromReaderFailed", errorEvent.EventType, "Error event should be ReturningBookFromReaderFailed")
}

func verifyNoNewEventsAppended(ctx context.Context, t *testing.T, wrapper Wrapper, bookID, readerID uuid.UUID) {
	t.Helper()

	es := wrapper.GetEventStore()
	filter := returnbookcopyfromreader.BuildEventFilter(bookID, readerID)

	events, _, err := es.Query(ctx, filter)
	assert.NoError(t, err, "Should query events successfully")

	assert.Equal(t, 4, len(events), "Should have exactly 4 events (no new events for idempotent operation)")

	if len(events) > 0 {
		lastEvent := events[len(events)-1]
		assert.Equal(t, "BookCopyReturnedByReader", lastEvent.EventType, "Last event should be BookCopyReturnedByReader")
	}
}

func assertIdempotentResult(t *testing.T, result shell.HandlerResult) {
	t.Helper()
	assert.True(t, result.Idempotent, "Operation should be idempotent")
}

func assertNonIdempotentResult(t *testing.T, result shell.HandlerResult) {
	t.Helper()
	assert.False(t, result.Idempotent, "Operation should not be idempotent")
}
