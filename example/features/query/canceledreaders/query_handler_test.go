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

type testReaders struct {
	reader1, reader2, reader3, reader4 uuid.UUID
}

func Test_QueryHandler_Handle_ReturnsEmptyResult_WhenNoReadersCanceled(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	readers := createTestReaders(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange - register readers but don't cancel any
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

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	readers := createTestReaders(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	registerReadersToLibrary(t, handlers, readers, fakeClock)

	// Cancel reader2 and reader4 contracts at different times
	cancelContractCmd2 := cancelreadercontract.BuildCommand(readers.reader2, fakeClock.Add(20*time.Minute))
	_, err := handlers.cancelContract.Handle(ctx, cancelContractCmd2)
	assert.NoError(t, err, "Should cancel reader2 contract")

	cancelContractCmd4 := cancelreadercontract.BuildCommand(readers.reader4, fakeClock.Add(21*time.Minute))
	_, err = handlers.cancelContract.Handle(ctx, cancelContractCmd4)
	assert.NoError(t, err, "Should cancel reader4 contract")

	// act
	result, err := handlers.query.Handle(ctx, canceledreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 canceled readers")

	// Should be sorted by CanceledAt (oldest first)
	expectedOrder := []string{readers.reader2.String(), readers.reader4.String()}
	assertCanceledReadersSortedCorrectly(t, result.Readers, expectedOrder)

	// Verify canceled readers ARE in results
	assert.Equal(t, readers.reader2.String(), result.Readers[0].ReaderID, "reader2 should be in results (was canceled)")
	assert.Equal(t, readers.reader4.String(), result.Readers[1].ReaderID, "reader4 should be in results (was canceled)")

	// Verify CanceledAt times are correct
	assert.Equal(t, fakeClock.Add(20*time.Minute), result.Readers[0].CanceledAt, "reader2 should have correct canceled time")
	assert.Equal(t, fakeClock.Add(21*time.Minute), result.Readers[1].CanceledAt, "reader4 should have correct canceled time")
}

func Test_QueryHandler_Handle_ReturnsEmptyResult_WhenNoReadersRegistered(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)

	// act - no readers registered
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

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	readers := createTestReaders(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange - register readers and then cancel them all
	registerReadersToLibrary(t, handlers, readers, fakeClock)

	// Cancel all readers at different times
	cancelContractCmd1 := cancelreadercontract.BuildCommand(readers.reader1, fakeClock.Add(10*time.Minute))
	_, err := handlers.cancelContract.Handle(ctx, cancelContractCmd1)
	assert.NoError(t, err, "Should cancel reader1 contract")

	cancelContractCmd2 := cancelreadercontract.BuildCommand(readers.reader2, fakeClock.Add(11*time.Minute))
	_, err = handlers.cancelContract.Handle(ctx, cancelContractCmd2)
	assert.NoError(t, err, "Should cancel reader2 contract")

	cancelContractCmd3 := cancelreadercontract.BuildCommand(readers.reader3, fakeClock.Add(12*time.Minute))
	_, err = handlers.cancelContract.Handle(ctx, cancelContractCmd3)
	assert.NoError(t, err, "Should cancel reader3 contract")

	cancelContractCmd4 := cancelreadercontract.BuildCommand(readers.reader4, fakeClock.Add(13*time.Minute))
	_, err = handlers.cancelContract.Handle(ctx, cancelContractCmd4)
	assert.NoError(t, err, "Should cancel reader4 contract")

	// act
	result, err := handlers.query.Handle(ctx, canceledreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 4, result.Count, "Should have 4 canceled readers")
	assert.Len(t, result.Readers, 4, "Should return 4 canceled readers")

	// Verify all readers are returned in the correct order (sorted by CanceledAt)
	expectedOrder := []string{readers.reader1.String(), readers.reader2.String(), readers.reader3.String(), readers.reader4.String()}
	assertCanceledReadersSortedCorrectly(t, result.Readers, expectedOrder)
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

	// arrange - complex scenario with register/cancel operations

	// Register reader1 and reader2
	registerReaderCmd1 := registerreader.BuildCommand(readers.reader1, "Alice Reader", fakeClock)
	_, err := handlers.registerReader.Handle(ctx, registerReaderCmd1)
	assert.NoError(t, err, "Should register reader1")

	registerReaderCmd2 := registerreader.BuildCommand(readers.reader2, "Bob Reader", fakeClock.Add(time.Minute))
	_, err = handlers.registerReader.Handle(ctx, registerReaderCmd2)
	assert.NoError(t, err, "Should register reader2")

	// Cancel reader1
	cancelContractCmd1 := cancelreadercontract.BuildCommand(readers.reader1, fakeClock.Add(2*time.Minute))
	_, err = handlers.cancelContract.Handle(ctx, cancelContractCmd1)
	assert.NoError(t, err, "Should cancel reader1 contract")

	// Register reader3 (but don't cancel)
	registerReaderCmd3 := registerreader.BuildCommand(readers.reader3, "Charlie Reader", fakeClock.Add(3*time.Minute))
	_, err = handlers.registerReader.Handle(ctx, registerReaderCmd3)
	assert.NoError(t, err, "Should register reader3")

	// Cancel reader2 later
	cancelContractCmd2 := cancelreadercontract.BuildCommand(readers.reader2, fakeClock.Add(4*time.Minute))
	_, err = handlers.cancelContract.Handle(ctx, cancelContractCmd2)
	assert.NoError(t, err, "Should cancel reader2 contract")

	// act
	result, err := handlers.query.Handle(ctx, canceledreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 2, result.Count, "Should have 2 canceled readers (reader1 and reader2)")

	// Should be sorted by CanceledAt (oldest first)
	expectedOrder := []string{readers.reader1.String(), readers.reader2.String()}
	assertCanceledReadersSortedCorrectly(t, result.Readers, expectedOrder)

	// Verify specific canceled readers and their times
	assert.Equal(t, readers.reader1.String(), result.Readers[0].ReaderID, "reader1 should be first (canceled earlier)")
	assert.Equal(t, readers.reader2.String(), result.Readers[1].ReaderID, "reader2 should be second (canceled later)")
	assert.Equal(t, fakeClock.Add(2*time.Minute), result.Readers[0].CanceledAt, "reader1 canceled time")
	assert.Equal(t, fakeClock.Add(4*time.Minute), result.Readers[1].CanceledAt, "reader2 canceled time")
}

func Test_QueryHandler_Handle_IdempotentCancellation_OnlyShowsReaderOnce(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use eventual consistency for query handlers (can tolerate slightly stale data)
	ctx = eventstore.WithEventualConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)
	readers := createTestReaders(t)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange - register reader1
	registerReaderCmd1 := registerreader.BuildCommand(readers.reader1, "Alice Reader", fakeClock)
	_, err := handlers.registerReader.Handle(ctx, registerReaderCmd1)
	assert.NoError(t, err, "Should register reader1")

	// Cancel reader1 multiple times (idempotent operations)
	cancelContractCmd1 := cancelreadercontract.BuildCommand(readers.reader1, fakeClock.Add(10*time.Minute))
	_, err = handlers.cancelContract.Handle(ctx, cancelContractCmd1)
	assert.NoError(t, err, "Should cancel reader1 contract (first time)")

	// Try to cancel the same reader again (should be idempotent)
	cancelContractCmd2 := cancelreadercontract.BuildCommand(readers.reader1, fakeClock.Add(15*time.Minute))
	_, err = handlers.cancelContract.Handle(ctx, cancelContractCmd2)
	assert.NoError(t, err, "Should handle idempotent cancellation (second time)")

	// act
	result, err := handlers.query.Handle(ctx, canceledreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 1, result.Count, "Should have exactly 1 canceled reader (no duplicates)")
	assert.Len(t, result.Readers, 1, "Should return exactly 1 canceled reader")

	// Verify it's the correct reader with the first cancellation time
	assert.Equal(t, readers.reader1.String(), result.Readers[0].ReaderID, "Should be reader1")
	assert.Equal(t, fakeClock.Add(10*time.Minute), result.Readers[0].CanceledAt, "Should show first cancellation time")
}

func Test_QueryHandler_Handle_WithStrongConsistency_WorksCorrectly(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use strong consistency (non-default for query handlers) to verify it still works
	ctx = eventstore.WithStrongConsistency(ctx)

	handlers := createAllHandlers(t, wrapper)

	// act
	query := canceledreaders.BuildQuery()
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
	registerReaderHandler := registerreader.NewCommandHandler(wrapper.GetEventStore())
	cancelContractHandler := cancelreadercontract.NewCommandHandler(wrapper.GetEventStore())

	queryHandler, err := canceledreaders.NewQueryHandler(wrapper.GetEventStore())
	assert.NoError(t, err, "Should create CanceledReaders query handler")

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
	_, err := handlers.registerReader.Handle(context.Background(), registerReaderCmd1)
	assert.NoError(t, err, "Should register reader1")

	registerReaderCmd2 := registerreader.BuildCommand(readers.reader2, "Bob Reader", fakeClock.Add(time.Minute))
	_, err = handlers.registerReader.Handle(context.Background(), registerReaderCmd2)
	assert.NoError(t, err, "Should register reader2")

	registerReaderCmd3 := registerreader.BuildCommand(readers.reader3, "Charlie Reader", fakeClock.Add(2*time.Minute))
	_, err = handlers.registerReader.Handle(context.Background(), registerReaderCmd3)
	assert.NoError(t, err, "Should register reader3")

	registerReaderCmd4 := registerreader.BuildCommand(readers.reader4, "Diana Reader", fakeClock.Add(3*time.Minute))
	_, err = handlers.registerReader.Handle(context.Background(), registerReaderCmd4)
	assert.NoError(t, err, "Should register reader4")
}

// Assertion helpers.
func assertCanceledReadersSortedCorrectly(t *testing.T, readers []canceledreaders.ReaderInfo, expectedOrder []string) {
	assert.Len(t, readers, len(expectedOrder), "Should have correct number of canceled readers")

	for i, reader := range readers {
		assert.Equal(t, expectedOrder[i], reader.ReaderID, "Canceled readers should be sorted by CanceledAt time")
	}

	// Verify timestamps are properly ordered
	for i := 0; i < len(readers)-1; i++ {
		assert.True(t, readers[i].CanceledAt.Before(readers[i+1].CanceledAt) || readers[i].CanceledAt.Equal(readers[i+1].CanceledAt),
			"Canceled readers should be sorted by CanceledAt time (oldest first)")
	}
}
