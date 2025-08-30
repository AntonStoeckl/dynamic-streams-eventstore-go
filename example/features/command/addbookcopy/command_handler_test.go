package addbookcopy_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/addbookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

func Test_CommandHandler_Handle_Success(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	ctx = eventstore.WithStrongConsistency(ctx)
	defer cleanup()

	addBookHandler := createAddBookHandler(t, wrapper)

	fakeClock := time.Unix(0, 0).UTC()
	bookID := uuid.New()

	// act
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
	result, err := addBookHandler.Handle(ctx, addBookCmd)

	// assert
	assert.NoError(t, err, "Should successfully add book to circulation")
	assertNonIdempotentResult(t, result)
	verifyEventsPersisted(ctx, t, wrapper, bookID)
}

func Test_CommandHandler_Handle_Idempotent_BookAlreadyAdded(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	ctx = eventstore.WithStrongConsistency(ctx)
	defer cleanup()

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
	assert.NoError(t, err, "Should successfully add book to circulation first time")

	// act
	result, err := addBookHandler.Handle(ctx, addBookCmd)

	// assert
	assert.NoError(t, err, "Should succeed (idempotent) when book already added to circulation")
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

func createAddBookHandler(t *testing.T, wrapper Wrapper) addbookcopy.CommandHandler {
	t.Helper()

	handler := addbookcopy.NewCommandHandler(wrapper.GetEventStore())

	return handler
}

func verifyEventsPersisted(ctx context.Context, t *testing.T, wrapper Wrapper, bookID uuid.UUID) {
	t.Helper()

	es := wrapper.GetEventStore()
	filter := addbookcopy.BuildEventFilter(bookID)

	events, _, err := es.Query(ctx, filter)
	assert.NoError(t, err, "Should query events successfully")

	assert.GreaterOrEqual(t, len(events), 1, "Should have at least 1 event persisted")

	if len(events) > 0 {
		lastEvent := events[len(events)-1]
		assert.Equal(t, "BookCopyAddedToCirculation", lastEvent.EventType, "Last event should be BookCopyAddedToCirculation")
	}
}

func verifyNoNewEventsAppended(ctx context.Context, t *testing.T, wrapper Wrapper, bookID uuid.UUID) {
	t.Helper()

	es := wrapper.GetEventStore()
	filter := addbookcopy.BuildEventFilter(bookID)

	events, _, err := es.Query(ctx, filter)
	assert.NoError(t, err, "Should query events successfully")

	assert.Equal(t, 1, len(events), "Should have exactly 1 event (no new events for idempotent operation)")

	if len(events) > 0 {
		lastEvent := events[len(events)-1]
		assert.Equal(t, "BookCopyAddedToCirculation", lastEvent.EventType, "Last event should be BookCopyAddedToCirculation")
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
