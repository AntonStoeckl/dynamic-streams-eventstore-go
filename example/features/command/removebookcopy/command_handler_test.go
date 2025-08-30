package removebookcopy_test

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
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

func Test_CommandHandler_Handle_Success(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	ctx = eventstore.WithStrongConsistency(ctx)
	defer cleanup()

	removeBookHandler := createRemoveBookHandler(t, wrapper)
	addBookHandler := createAddBookHandler(t, wrapper)

	fakeClock := time.Unix(0, 0).UTC()
	bookID := uuid.New()

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

	// act
	removeBookCmd := removebookcopy.BuildCommand(
		bookID,
		fakeClock.Add(2*time.Hour),
	)
	result, err := removeBookHandler.Handle(ctx, removeBookCmd)

	// assert
	assert.NoError(t, err, "Should successfully remove book from circulation")
	assertNonIdempotentResult(t, result)
	verifyEventsPersisted(ctx, t, wrapper, bookID)
}

//nolint:funlen
func Test_CommandHandler_Handle_Error_BookCurrentlyLent(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	ctx = eventstore.WithStrongConsistency(ctx)
	defer cleanup()

	removeBookHandler := createRemoveBookHandler(t, wrapper)
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
	removeBookCmd := removebookcopy.BuildCommand(
		bookID,
		fakeClock.Add(4*time.Hour),
	)
	_, err = removeBookHandler.Handle(ctx, removeBookCmd)

	// assert
	assert.Error(t, err, "Should fail to remove book that is currently lent")
	assert.ErrorContains(t, err, "book is currently lent", "Error should mention book is currently lent")
	verifyErrorEventPersisted(ctx, t, wrapper, bookID)
}

func Test_CommandHandler_Handle_Idempotent_BookAlreadyRemoved(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	ctx = eventstore.WithStrongConsistency(ctx)
	defer cleanup()

	removeBookHandler := createRemoveBookHandler(t, wrapper)
	addBookHandler := createAddBookHandler(t, wrapper)

	fakeClock := time.Unix(0, 0).UTC()
	bookID := uuid.New()

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

	removeBookCmd := removebookcopy.BuildCommand(
		bookID,
		fakeClock.Add(2*time.Hour),
	)
	_, err = removeBookHandler.Handle(ctx, removeBookCmd)
	assert.NoError(t, err, "Should successfully remove book from circulation first time")

	// act
	result, err := removeBookHandler.Handle(ctx, removeBookCmd)

	// assert
	assert.NoError(t, err, "Should succeed (idempotent) when book already removed from circulation")
	assertIdempotentResult(t, result)
	verifyNoNewEventsAppended(ctx, t, wrapper, bookID)
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

func createRemoveBookHandler(t *testing.T, wrapper Wrapper) removebookcopy.CommandHandler {
	t.Helper()

	handler := removebookcopy.NewCommandHandler(wrapper.GetEventStore())

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

func verifyEventsPersisted(ctx context.Context, t *testing.T, wrapper Wrapper, bookID uuid.UUID) {
	t.Helper()

	es := wrapper.GetEventStore()
	filter := removebookcopy.BuildEventFilter(bookID)

	events, _, err := es.Query(ctx, filter)
	assert.NoError(t, err, "Should query events successfully")

	assert.GreaterOrEqual(t, len(events), 2, "Should have at least 2 events persisted")

	if len(events) > 0 {
		lastEvent := events[len(events)-1]
		assert.Equal(t, "BookCopyRemovedFromCirculation", lastEvent.EventType, "Last event should be BookCopyRemovedFromCirculation")
	}
}

func verifyErrorEventPersisted(ctx context.Context, t *testing.T, wrapper Wrapper, bookID uuid.UUID) {
	t.Helper()

	es := wrapper.GetEventStore()

	errorFilter := eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf("RemovingBookFromCirculationFailed").
		AndAnyPredicateOf(
			eventstore.P("BookID", bookID.String()),
		).
		Finalize()

	errorEvents, _, err := es.Query(ctx, errorFilter)
	assert.NoError(t, err, "Should query error events successfully")

	assert.Greater(t, len(errorEvents), 0, "Should have error event persisted")

	errorEvent := errorEvents[len(errorEvents)-1]
	assert.Equal(t, "RemovingBookFromCirculationFailed", errorEvent.EventType, "Error event should be RemovingBookFromCirculationFailed")
}

func verifyNoNewEventsAppended(ctx context.Context, t *testing.T, wrapper Wrapper, bookID uuid.UUID) {
	t.Helper()

	es := wrapper.GetEventStore()
	filter := removebookcopy.BuildEventFilter(bookID)

	events, _, err := es.Query(ctx, filter)
	assert.NoError(t, err, "Should query events successfully")

	assert.Equal(t, 2, len(events), "Should have exactly 2 events (no new events for idempotent operation)")

	if len(events) > 0 {
		lastEvent := events[len(events)-1]
		assert.Equal(t, "BookCopyRemovedFromCirculation", lastEvent.EventType, "Last event should be BookCopyRemovedFromCirculation")
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
