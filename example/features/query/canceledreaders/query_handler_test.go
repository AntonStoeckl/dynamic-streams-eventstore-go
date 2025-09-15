package canceledreaders_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/cancelreadercontract"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/registerreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/canceledreaders"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

type testHandlers struct {
	registerReader registerreader.CommandHandler
	cancelContract cancelreadercontract.CommandHandler
	query          canceledreaders.QueryHandler
}

type testReaderIDs struct {
	reader1ID, reader2ID, reader3ID, reader4ID uuid.UUID
}

func Test_QueryHandler_Handle_ReturnsEmptyResult_WhenNoReadersCanceled(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	readers := createTestReaders(t)
	registerReadersToLibrary(t, handlers, readers, fakeClock)

	// act
	result, err := handlers.query.Handle(ctx, canceledreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 0, result.Count, "Should have 0 canceled readers")
	assert.Len(t, result.Readers, 0, "Should return empty Readers slice")
}

func Test_QueryHandler_Handle_IncludesCanceledReadersSortedByCanceledAt(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)
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
	result, err := handlers.query.Handle(ctx, canceledreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 canceled readers")
	expectedOrder := []string{readers.reader2ID.String(), readers.reader4ID.String()}
	assertCanceledReadersSortedCorrectly(t, result.Readers, expectedOrder)
	assert.Equal(t, readers.reader2ID.String(), result.Readers[0].ReaderID, "reader2 should be in results (was canceled)")
	assert.Equal(t, readers.reader4ID.String(), result.Readers[1].ReaderID, "reader4 should be in results (was canceled)")
	assert.Equal(t, fakeClock.Add(20*time.Minute), result.Readers[0].CanceledAt, "reader2 should have correct canceled time")
	assert.Equal(t, fakeClock.Add(21*time.Minute), result.Readers[1].CanceledAt, "reader4 should have correct canceled time")
}

func Test_QueryHandler_Handle_ReturnsEmptyResult_WhenNoReadersRegistered(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)

	// act
	result, err := handlers.query.Handle(ctx, canceledreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 0, result.Count, "Should have 0 canceled readers")
	assert.Len(t, result.Readers, 0, "Should return empty Readers slice")
}

func Test_QueryHandler_Handle_ReturnsAllCanceledReaders_WhenAllReadersAreCanceled(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)
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
	result, err := handlers.query.Handle(ctx, canceledreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 4, result.Count, "Should have 4 canceled readers")
	assert.Len(t, result.Readers, 4, "Should return 4 canceled readers")
	expectedOrder := []string{readers.reader1ID.String(), readers.reader2ID.String(), readers.reader3ID.String(), readers.reader4ID.String()}
	assertCanceledReadersSortedCorrectly(t, result.Readers, expectedOrder)
}

func Test_QueryHandler_Handle_HandlesMixedRegisterAndCancelOperations(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	readers := createTestReaders(t)

	// Register reader1 and reader2
	registerReaderCmd1 := registerreader.BuildCommand(readers.reader1ID, "Alice Reader", fakeClock)
	_, err := handlers.registerReader.Handle(ctx, registerReaderCmd1)
	assert.NoError(t, err, "Should register reader1")

	registerReaderCmd2 := registerreader.BuildCommand(readers.reader2ID, "Bob Reader", fakeClock.Add(time.Minute))
	_, err = handlers.registerReader.Handle(ctx, registerReaderCmd2)
	assert.NoError(t, err, "Should register reader2")

	// Cancel reader1
	cancelContractCmd1 := cancelreadercontract.BuildCommand(readers.reader1ID, fakeClock.Add(2*time.Minute))
	_, err = handlers.cancelContract.Handle(ctx, cancelContractCmd1)
	assert.NoError(t, err, "Should cancel reader1 contract")

	// Register reader3 (but don't cancel)
	registerReaderCmd3 := registerreader.BuildCommand(readers.reader3ID, "Charlie Reader", fakeClock.Add(3*time.Minute))
	_, err = handlers.registerReader.Handle(ctx, registerReaderCmd3)
	assert.NoError(t, err, "Should register reader3")

	// Cancel reader2 later
	cancelContractCmd2 := cancelreadercontract.BuildCommand(readers.reader2ID, fakeClock.Add(4*time.Minute))
	_, err = handlers.cancelContract.Handle(ctx, cancelContractCmd2)
	assert.NoError(t, err, "Should cancel reader2 contract")

	// act
	result, err := handlers.query.Handle(ctx, canceledreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 canceled readers (reader1 and reader2)")
	expectedOrder := []string{readers.reader1ID.String(), readers.reader2ID.String()}
	assertCanceledReadersSortedCorrectly(t, result.Readers, expectedOrder)
	assert.Equal(t, readers.reader1ID.String(), result.Readers[0].ReaderID, "reader1 should be first (canceled earlier)")
	assert.Equal(t, readers.reader2ID.String(), result.Readers[1].ReaderID, "reader2 should be second (canceled later)")
	assert.Equal(t, fakeClock.Add(2*time.Minute), result.Readers[0].CanceledAt, "reader1 canceled time")
	assert.Equal(t, fakeClock.Add(4*time.Minute), result.Readers[1].CanceledAt, "reader2 canceled time")
}

func Test_QueryHandler_Handle_IdempotentCancellation_OnlyShowsReaderOnce(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	readers := createTestReaders(t)
	registerReaderCmd1 := registerreader.BuildCommand(readers.reader1ID, "Alice Reader", fakeClock)
	_, err := handlers.registerReader.Handle(ctx, registerReaderCmd1)
	assert.NoError(t, err, "Should register reader1")

	cancelContractCmd1 := cancelreadercontract.BuildCommand(readers.reader1ID, fakeClock.Add(10*time.Minute))
	_, err = handlers.cancelContract.Handle(ctx, cancelContractCmd1)
	assert.NoError(t, err, "Should cancel reader1 contract (first time)")

	cancelContractCmd2 := cancelreadercontract.BuildCommand(readers.reader1ID, fakeClock.Add(15*time.Minute))
	_, err = handlers.cancelContract.Handle(ctx, cancelContractCmd2)
	assert.NoError(t, err, "Should handle idempotent cancellation (second time)")

	// act
	result, err := handlers.query.Handle(ctx, canceledreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 1, result.Count, "Should have exactly 1 canceled reader (no duplicates)")
	assert.Len(t, result.Readers, 1, "Should return exactly 1 canceled reader")
	assert.Equal(t, readers.reader1ID.String(), result.Readers[0].ReaderID, "Should be reader1")
	assert.Equal(t, fakeClock.Add(10*time.Minute), result.Readers[0].CanceledAt, "Should show first cancellation time")
}

func Test_QueryHandler_Handle_WithStrongConsistency_WorksCorrectly(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx = eventstore.WithStrongConsistency(ctx)
	handlers := createAllHandlers(t, wrapper)

	// act
	query := canceledreaders.BuildQuery()
	result, err := handlers.query.Handle(ctx, query)

	// assert
	assert.NoError(t, err, "Query should succeed with strong consistency")
	assert.NotNil(t, result, "Should return result")
}

// Test setup helpers

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

	registerReaderHandler := registerreader.NewCommandHandler(wrapper.GetEventStore())
	cancelContractHandler := cancelreadercontract.NewCommandHandler(wrapper.GetEventStore())
	queryHandler := canceledreaders.NewQueryHandler(wrapper.GetEventStore())

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

func assertCanceledReadersSortedCorrectly(
	t *testing.T,
	readers []canceledreaders.ReaderInfo,
	expectedOrder []string,
) {
	t.Helper()

	assert.Len(t, readers, len(expectedOrder), "Should have correct number of canceled readers")

	for i, reader := range readers {
		assert.Equal(t, expectedOrder[i], reader.ReaderID, "Canceled readers should be sorted by CanceledAt time")
	}

	for i := 0; i < len(readers)-1; i++ {
		assert.True(t, readers[i].CanceledAt.Before(readers[i+1].CanceledAt) || readers[i].CanceledAt.Equal(readers[i+1].CanceledAt),
			"Canceled readers should be sorted by CanceledAt time (oldest first)")
	}
}
