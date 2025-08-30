package registerreader_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/registerreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

func Test_CommandHandler_Handle_Success(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	ctx = eventstore.WithStrongConsistency(ctx)
	defer cleanup()

	registerReaderHandler := createRegisterReaderHandler(t, wrapper)

	fakeClock := time.Unix(0, 0).UTC()
	readerID := uuid.New()

	// act
	registerReaderCmd := registerreader.BuildCommand(
		readerID,
		"John Doe",
		fakeClock.Add(time.Hour),
	)
	result, err := registerReaderHandler.Handle(ctx, registerReaderCmd)

	// assert
	assert.NoError(t, err, "Should successfully register reader")
	assertNonIdempotentResult(t, result)
	verifyEventsPersisted(ctx, t, wrapper, readerID)
}

func Test_CommandHandler_Handle_Idempotent_ReaderAlreadyRegistered(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	ctx = eventstore.WithStrongConsistency(ctx)
	defer cleanup()

	registerReaderHandler := createRegisterReaderHandler(t, wrapper)

	fakeClock := time.Unix(0, 0).UTC()
	readerID := uuid.New()

	// arrange
	registerReaderCmd := registerreader.BuildCommand(
		readerID,
		"John Doe",
		fakeClock.Add(time.Hour),
	)
	_, err := registerReaderHandler.Handle(ctx, registerReaderCmd)
	assert.NoError(t, err, "Should successfully register reader first time")

	// act
	result, err := registerReaderHandler.Handle(ctx, registerReaderCmd)

	// assert
	assert.NoError(t, err, "Should succeed (idempotent) when reader already registered")
	assertIdempotentResult(t, result)
	verifyNoNewEventsAppended(ctx, t, wrapper, readerID)
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

func createRegisterReaderHandler(t *testing.T, wrapper Wrapper) registerreader.CommandHandler {
	t.Helper()

	handler := registerreader.NewCommandHandler(wrapper.GetEventStore())

	return handler
}

func verifyEventsPersisted(ctx context.Context, t *testing.T, wrapper Wrapper, readerID uuid.UUID) {
	t.Helper()

	es := wrapper.GetEventStore()
	filter := registerreader.BuildEventFilter(readerID)

	events, _, err := es.Query(ctx, filter)
	assert.NoError(t, err, "Should query events successfully")

	assert.GreaterOrEqual(t, len(events), 1, "Should have at least 1 event persisted")

	if len(events) > 0 {
		lastEvent := events[len(events)-1]
		assert.Equal(t, "ReaderRegistered", lastEvent.EventType, "Last event should be ReaderRegistered")
	}
}

func verifyNoNewEventsAppended(ctx context.Context, t *testing.T, wrapper Wrapper, readerID uuid.UUID) {
	t.Helper()

	es := wrapper.GetEventStore()
	filter := registerreader.BuildEventFilter(readerID)

	events, _, err := es.Query(ctx, filter)
	assert.NoError(t, err, "Should query events successfully")

	assert.Equal(t, 1, len(events), "Should have exactly 1 event (no new events for idempotent operation)")

	if len(events) > 0 {
		lastEvent := events[len(events)-1]
		assert.Equal(t, "ReaderRegistered", lastEvent.EventType, "Last event should be ReaderRegistered")
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
