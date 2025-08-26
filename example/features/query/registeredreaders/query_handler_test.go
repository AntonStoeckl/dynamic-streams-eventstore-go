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

type testReaders struct {
	reader1, reader2, reader3, reader4 uuid.UUID
}

func Test_QueryHandler_Handle_ReturnsCorrectReadersSortedByRegisteredAt(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	readers := createTestReaders(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange - register readers in chronological order
	registerReadersToLibrary(t, handlers, readers, fakeClock)

	// act
	result, err := handlers.query.Handle(ctx, registeredreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 4, result.Count, "Should have 4 registered readers")

	expectedOrder := []string{readers.reader1.String(), readers.reader2.String(), readers.reader3.String(), readers.reader4.String()}
	assertReadersSortedCorrectly(t, result.Readers, expectedOrder)

	// Verify specific reader details for the first reader
	assertSpecificReaderDetails(t, result.Readers[0], "Alice Reader", fakeClock)
}

func Test_QueryHandler_Handle_ExcludesCanceledReaders(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	readers := createTestReaders(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	registerReadersToLibrary(t, handlers, readers, fakeClock)

	// Cancel reader2 and reader4 contracts
	cancelContractCmd2 := cancelreadercontract.BuildCommand(readers.reader2, fakeClock.Add(20*time.Minute))
	err := handlers.cancelContract.Handle(ctx, cancelContractCmd2)
	assert.NoError(t, err, "Should cancel reader2 contract")

	cancelContractCmd4 := cancelreadercontract.BuildCommand(readers.reader4, fakeClock.Add(21*time.Minute))
	err = handlers.cancelContract.Handle(ctx, cancelContractCmd4)
	assert.NoError(t, err, "Should cancel reader4 contract")

	// act
	result, err := handlers.query.Handle(ctx, registeredreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 readers remaining (reader2 and reader4 were canceled)")

	expectedOrder := []string{readers.reader1.String(), readers.reader3.String()}
	assertReadersSortedCorrectly(t, result.Readers, expectedOrder)

	// Verify canceled readers are not in results
	for _, reader := range result.Readers {
		assert.NotEqual(t, readers.reader2.String(), reader.ReaderID, "reader2 should not be in results (was canceled)")
		assert.NotEqual(t, readers.reader4.String(), reader.ReaderID, "reader4 should not be in results (was canceled)")
	}
}

func Test_QueryHandler_Handle_ReturnsEmptyResult_WhenNoReadersRegistered(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)

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

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	readers := createTestReaders(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange - register readers and then cancel them all
	registerReadersToLibrary(t, handlers, readers, fakeClock)

	// Cancel all readers
	cancelContractCmd1 := cancelreadercontract.BuildCommand(readers.reader1, fakeClock.Add(10*time.Minute))
	err := handlers.cancelContract.Handle(ctx, cancelContractCmd1)
	assert.NoError(t, err, "Should cancel reader1 contract")

	cancelContractCmd2 := cancelreadercontract.BuildCommand(readers.reader2, fakeClock.Add(11*time.Minute))
	err = handlers.cancelContract.Handle(ctx, cancelContractCmd2)
	assert.NoError(t, err, "Should cancel reader2 contract")

	cancelContractCmd3 := cancelreadercontract.BuildCommand(readers.reader3, fakeClock.Add(12*time.Minute))
	err = handlers.cancelContract.Handle(ctx, cancelContractCmd3)
	assert.NoError(t, err, "Should cancel reader3 contract")

	cancelContractCmd4 := cancelreadercontract.BuildCommand(readers.reader4, fakeClock.Add(13*time.Minute))
	err = handlers.cancelContract.Handle(ctx, cancelContractCmd4)
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

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	readers := createTestReaders(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange - complex scenario with register/cancel/register operations

	// Register reader1 and reader2
	registerReaderCmd1 := registerreader.BuildCommand(readers.reader1, "Alice Reader", fakeClock)
	err := handlers.registerReader.Handle(ctx, registerReaderCmd1)
	assert.NoError(t, err, "Should register reader1")

	registerReaderCmd2 := registerreader.BuildCommand(readers.reader2, "Bob Reader", fakeClock.Add(time.Minute))
	err = handlers.registerReader.Handle(ctx, registerReaderCmd2)
	assert.NoError(t, err, "Should register reader2")

	// Cancel reader1
	cancelContractCmd1 := cancelreadercontract.BuildCommand(readers.reader1, fakeClock.Add(2*time.Minute))
	err = handlers.cancelContract.Handle(ctx, cancelContractCmd1)
	assert.NoError(t, err, "Should cancel reader1 contract")

	// Register reader3
	registerReaderCmd3 := registerreader.BuildCommand(readers.reader3, "Charlie Reader", fakeClock.Add(3*time.Minute))
	err = handlers.registerReader.Handle(ctx, registerReaderCmd3)
	assert.NoError(t, err, "Should register reader3")

	// act
	result, err := handlers.query.Handle(ctx, registeredreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 registered readers (reader2 and reader3)")

	expectedOrder := []string{readers.reader2.String(), readers.reader3.String()}
	assertReadersSortedCorrectly(t, result.Readers, expectedOrder)

	// Verify reader1 is not in results (was canceled)
	for _, reader := range result.Readers {
		assert.NotEqual(t, readers.reader1.String(), reader.ReaderID, "reader1 should not be in results (was canceled)")
	}
}

func Test_QueryHandler_Handle_WithStrongConsistency_WorksCorrectly(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use strong consistency (non-default for query handlers) to verify it still works
	ctx = eventstore.WithStrongConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)

	// act
	query := registeredreaders.BuildQuery()
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
	registerReaderHandler, err := registerreader.NewCommandHandler(wrapper.GetEventStore())
	assert.NoError(t, err, "Should create RegisterReader handler")

	cancelContractHandler, err := cancelreadercontract.NewCommandHandler(wrapper.GetEventStore())
	assert.NoError(t, err, "Should create CancelReaderContract handler")

	queryHandler, err := registeredreaders.NewQueryHandler(wrapper.GetEventStore())
	assert.NoError(t, err, "Should create RegisteredReaders query handler")

	return testHandlers{
		registerReader: registerReaderHandler,
		cancelContract: cancelContractHandler,
		query:          queryHandler,
	}
}

func createTestReaders(t *testing.T) testReaders {
	return testReaders{
		reader1: GivenUniqueID(t),
		reader2: GivenUniqueID(t),
		reader3: GivenUniqueID(t),
		reader4: GivenUniqueID(t),
	}
}

func registerReadersToLibrary(t *testing.T, handlers testHandlers, readers testReaders, fakeClock time.Time) {
	registerReaderCmd1 := registerreader.BuildCommand(readers.reader1, "Alice Reader", fakeClock)
	err := handlers.registerReader.Handle(context.Background(), registerReaderCmd1)
	assert.NoError(t, err, "Should register reader1")

	registerReaderCmd2 := registerreader.BuildCommand(readers.reader2, "Bob Reader", fakeClock.Add(time.Minute))
	err = handlers.registerReader.Handle(context.Background(), registerReaderCmd2)
	assert.NoError(t, err, "Should register reader2")

	registerReaderCmd3 := registerreader.BuildCommand(readers.reader3, "Charlie Reader", fakeClock.Add(2*time.Minute))
	err = handlers.registerReader.Handle(context.Background(), registerReaderCmd3)
	assert.NoError(t, err, "Should register reader3")

	registerReaderCmd4 := registerreader.BuildCommand(readers.reader4, "Diana Reader", fakeClock.Add(3*time.Minute))
	err = handlers.registerReader.Handle(context.Background(), registerReaderCmd4)
	assert.NoError(t, err, "Should register reader4")
}

// Assertion helpers.
func assertReadersSortedCorrectly(t *testing.T, readers []registeredreaders.ReaderInfo, expectedOrder []string) {
	assert.Len(t, readers, len(expectedOrder), "Should have correct number of readers")

	for i, reader := range readers {
		assert.Equal(t, expectedOrder[i], reader.ReaderID, "Readers should be sorted by RegisteredAt time")
	}

	// Verify timestamps are properly ordered
	for i := 0; i < len(readers)-1; i++ {
		assert.True(t, readers[i].RegisteredAt.Before(readers[i+1].RegisteredAt) || readers[i].RegisteredAt.Equal(readers[i+1].RegisteredAt),
			"Readers should be sorted by RegisteredAt time (oldest first)")
	}
}

func assertSpecificReaderDetails(t *testing.T, reader registeredreaders.ReaderInfo, expectedName string, expectedRegisteredAt time.Time) {
	assert.Equal(t, expectedName, reader.Name, "Reader should have correct name")
	assert.Equal(t, expectedRegisteredAt, reader.RegisteredAt, "Reader should have correct registered at time")
}
