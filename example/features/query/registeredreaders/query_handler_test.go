package registeredreaders_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/cancelreadercontract"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/registerreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/registeredreaders"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

type testHandlers struct {
	registerReader registerreader.CommandHandler
	cancelContract cancelreadercontract.CommandHandler
	query          registeredreaders.QueryHandler
}

type testReaderIDs struct {
	reader1ID, reader2ID, reader3ID, reader4ID uuid.UUID
}

func Test_QueryHandler_Handle_ReturnsCorrectReadersSortedByRegisteredAt(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	readers := createTestReaders(t)
	registerReadersToLibrary(t, handlers, readers, fakeClock)

	// act
	result, err := handlers.query.Handle(ctx, registeredreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 4, result.Count, "Should have 4 registered readers")

	expectedOrder := []string{readers.reader1ID.String(), readers.reader2ID.String(), readers.reader3ID.String(), readers.reader4ID.String()}
	assertReadersSortedCorrectly(t, result.Readers, expectedOrder)
	assertSpecificReaderDetails(t, result.Readers[0], "Alice Reader", fakeClock)
}

func Test_QueryHandler_Handle_ExcludesCanceledReaders(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	readers := createTestReaders(t)
	registerReadersToLibrary(t, handlers, readers, fakeClock)

	cancelContractCmd2 := cancelreadercontract.BuildCommand(readers.reader2ID, fakeClock.Add(20*time.Minute))
	_, err := handlers.cancelContract.Handle(ctx, cancelContractCmd2)
	assert.NoError(t, err, "Should cancel reader2 contract")

	cancelContractCmd4 := cancelreadercontract.BuildCommand(readers.reader4ID, fakeClock.Add(21*time.Minute))
	_, err = handlers.cancelContract.Handle(ctx, cancelContractCmd4)
	assert.NoError(t, err, "Should cancel reader4 contract")

	// act
	result, err := handlers.query.Handle(ctx, registeredreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 readers remaining (reader2 and reader4 were canceled)")

	expectedOrder := []string{readers.reader1ID.String(), readers.reader3ID.String()}
	assertReadersSortedCorrectly(t, result.Readers, expectedOrder)

	for _, reader := range result.Readers {
		assert.NotEqual(t, readers.reader2ID.String(), reader.ReaderID, "reader2 should not be in results (was canceled)")
		assert.NotEqual(t, readers.reader4ID.String(), reader.ReaderID, "reader4 should not be in results (was canceled)")
	}
}

func Test_QueryHandler_Handle_ReturnsEmptyResult_WhenNoReadersRegistered(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(wrapper)

	// act
	result, err := handlers.query.Handle(ctx, registeredreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 0, result.Count, "Should have 0 registered readers")
	assert.Len(t, result.Readers, 0, "Should return empty Readers slice")
}

func Test_QueryHandler_Handle_ReturnsCorrectResult_WhenAllReadersAreCanceled(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	readers := createTestReaders(t)
	registerReadersToLibrary(t, handlers, readers, fakeClock)

	cancelContractCmd1 := cancelreadercontract.BuildCommand(readers.reader1ID, fakeClock.Add(10*time.Minute))
	_, err := handlers.cancelContract.Handle(ctx, cancelContractCmd1)
	assert.NoError(t, err, "Should cancel reader1 contract")

	cancelContractCmd2 := cancelreadercontract.BuildCommand(readers.reader2ID, fakeClock.Add(11*time.Minute))
	_, err = handlers.cancelContract.Handle(ctx, cancelContractCmd2)
	assert.NoError(t, err, "Should cancel reader2 contract")

	cancelContractCmd3 := cancelreadercontract.BuildCommand(readers.reader3ID, fakeClock.Add(12*time.Minute))
	_, err = handlers.cancelContract.Handle(ctx, cancelContractCmd3)
	assert.NoError(t, err, "Should cancel reader3 contract")

	cancelContractCmd4 := cancelreadercontract.BuildCommand(readers.reader4ID, fakeClock.Add(13*time.Minute))
	_, err = handlers.cancelContract.Handle(ctx, cancelContractCmd4)
	assert.NoError(t, err, "Should cancel reader4 contract")

	// act
	result, err := handlers.query.Handle(ctx, registeredreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 0, result.Count, "Should have 0 registered readers after all are canceled")
	assert.Len(t, result.Readers, 0, "Should return empty Readers slice after all are canceled")
}

func Test_QueryHandler_Handle_HandlesMixedRegisterAndCancelOperations(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	readers := createTestReaders(t)

	registerReaderCmd1 := registerreader.BuildCommand(readers.reader1ID, "Alice Reader", fakeClock)
	_, err := handlers.registerReader.Handle(ctx, registerReaderCmd1)
	assert.NoError(t, err, "Should register reader1")

	registerReaderCmd2 := registerreader.BuildCommand(readers.reader2ID, "Bob Reader", fakeClock.Add(time.Minute))
	_, err = handlers.registerReader.Handle(ctx, registerReaderCmd2)
	assert.NoError(t, err, "Should register reader2")

	cancelContractCmd1 := cancelreadercontract.BuildCommand(readers.reader1ID, fakeClock.Add(2*time.Minute))
	_, err = handlers.cancelContract.Handle(ctx, cancelContractCmd1)
	assert.NoError(t, err, "Should cancel reader1 contract")

	registerReaderCmd3 := registerreader.BuildCommand(readers.reader3ID, "Charlie Reader", fakeClock.Add(3*time.Minute))
	_, err = handlers.registerReader.Handle(ctx, registerReaderCmd3)
	assert.NoError(t, err, "Should register reader3")

	// act
	result, err := handlers.query.Handle(ctx, registeredreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 registered readers (reader2 and reader3)")

	expectedOrder := []string{readers.reader2ID.String(), readers.reader3ID.String()}
	assertReadersSortedCorrectly(t, result.Readers, expectedOrder)

	for _, reader := range result.Readers {
		assert.NotEqual(t, readers.reader1ID.String(), reader.ReaderID, "reader1 should not be in results (was canceled)")
	}
}

func Test_QueryHandler_Handle_WithStrongConsistency_WorksCorrectly(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithStrongConsistency(ctx)
	handlers := createAllHandlers(wrapper)

	// act
	query := registeredreaders.BuildQuery()
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

func createAllHandlers(wrapper Wrapper) testHandlers {
	registerReaderHandler := registerreader.NewCommandHandler(wrapper.GetEventStore())
	cancelContractHandler := cancelreadercontract.NewCommandHandler(wrapper.GetEventStore())

	queryHandler := registeredreaders.NewQueryHandler(wrapper.GetEventStore())

	return testHandlers{
		registerReader: registerReaderHandler,
		cancelContract: cancelContractHandler,
		query:          queryHandler,
	}
}

func createTestReaders(t *testing.T) testReaderIDs {
	t.Helper()

	return testReaderIDs{
		reader1ID: GivenUniqueID(t),
		reader2ID: GivenUniqueID(t),
		reader3ID: GivenUniqueID(t),
		reader4ID: GivenUniqueID(t),
	}
}

func registerReadersToLibrary(
	t *testing.T,
	handlers testHandlers,
	readers testReaderIDs,
	fakeClock time.Time,
) {
	t.Helper()

	registerReaderCmd1 := registerreader.BuildCommand(readers.reader1ID, "Alice Reader", fakeClock)
	_, err := handlers.registerReader.Handle(context.Background(), registerReaderCmd1)
	assert.NoError(t, err, "Should register reader1")

	registerReaderCmd2 := registerreader.BuildCommand(readers.reader2ID, "Bob Reader", fakeClock.Add(time.Minute))
	_, err = handlers.registerReader.Handle(context.Background(), registerReaderCmd2)
	assert.NoError(t, err, "Should register reader2")

	registerReaderCmd3 := registerreader.BuildCommand(readers.reader3ID, "Charlie Reader", fakeClock.Add(2*time.Minute))
	_, err = handlers.registerReader.Handle(context.Background(), registerReaderCmd3)
	assert.NoError(t, err, "Should register reader3")

	registerReaderCmd4 := registerreader.BuildCommand(readers.reader4ID, "Diana Reader", fakeClock.Add(3*time.Minute))
	_, err = handlers.registerReader.Handle(context.Background(), registerReaderCmd4)
	assert.NoError(t, err, "Should register reader4")
}

// Assertion helpers

func assertReadersSortedCorrectly(
	t *testing.T,
	readers []registeredreaders.ReaderInfo,
	expectedOrder []string,
) {
	t.Helper()

	assert.Len(t, readers, len(expectedOrder), "Should have correct number of readers")

	for i, reader := range readers {
		assert.Equal(t, expectedOrder[i], reader.ReaderID, "Readers should be sorted by RegisteredAt time")
	}

	for i := 0; i < len(readers)-1; i++ {
		assert.True(t, readers[i].RegisteredAt.Before(readers[i+1].RegisteredAt) || readers[i].RegisteredAt.Equal(readers[i+1].RegisteredAt),
			"Readers should be sorted by RegisteredAt time (oldest first)")
	}
}

func assertSpecificReaderDetails(
	t *testing.T,
	reader registeredreaders.ReaderInfo,
	expectedName string,
	expectedRegisteredAt time.Time,
) {
	t.Helper()

	assert.Equal(t, expectedName, reader.Name, "Reader should have correct name")
	assert.Equal(t, expectedRegisteredAt, reader.RegisteredAt, "Reader should have correct registered at time")
}
