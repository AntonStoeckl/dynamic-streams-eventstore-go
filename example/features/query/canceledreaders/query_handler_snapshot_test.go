package canceledreaders_test

import (
	"context"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/cancelreadercontract"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/registerreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/canceledreaders"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/snapshot"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

func Test_SnapshotAwareQueryHandler_Handle_SnapshotMiss(t *testing.T) {
	// setup
	ctx, snapshotHandler, wrapper := setupSnapshotTest(t)

	// Create test data (1 canceled reader event)
	createTestCanceledReader(ctx, t, wrapper)

	// act
	result, err := snapshotHandler.Handle(ctx, canceledreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Snapshot handler should work")
	assert.Equal(t, 1, result.Count, "Should have 1 canceled reader")
}

func Test_SnapshotAwareQueryHandler_Handle_SnapshotCreationAndHitWithNoNewEvents(t *testing.T) {
	// setup
	ctx, snapshotHandler, wrapper := setupSnapshotTest(t)

	// Create test data (1 canceled reader event)
	createTestCanceledReader(ctx, t, wrapper)

	// act - first query (should miss snapshot and create one)
	result, err := snapshotHandler.Handle(ctx, canceledreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "First query should work")
	assert.Equal(t, 1, result.Count, "Should have 1 canceled reader")

	// Give async snapshot saving some time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that a snapshot was created in the database
	filter := canceledreaders.BuildEventFilter()
	savedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, canceledreaders.BuildQuery().SnapshotType(), filter)
	assert.NoError(t, err, "Should be able to load saved snapshot")
	assert.NotNil(t, savedSnapshot, "Snapshot should exist after first query")

	// act - second query (should hit the snapshot created by the first query)
	hitResult, err := snapshotHandler.Handle(ctx, canceledreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Second query should work")
	assert.Equal(t, 1, hitResult.Count, "Should have 1 canceled reader")
	assert.Equal(t, result, hitResult, "Results should be identical")
}

func Test_SnapshotAwareQueryHandler_Handle_SnapshotHitWithNewEvents(t *testing.T) {
	// setup
	ctx, snapshotHandler, wrapper := setupSnapshotTest(t)

	// Create the first canceled reader (will create sequence=2, since we need to register and cancel)
	createTestCanceledReader(ctx, t, wrapper)

	// First query: Should miss snapshot and fall back to base handler, then create snapshot
	result1, err := snapshotHandler.Handle(ctx, canceledreaders.BuildQuery())
	assert.NoError(t, err, "First query should work")
	assert.Equal(t, 1, result1.Count, "Should have 1 canceled reader initially")
	assert.Equal(t, uint(2), result1.SequenceNumber, "Should have sequence=2 (register + cancel)")

	// Give async snapshot saving some time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that a snapshot was created in the database
	filter := canceledreaders.BuildEventFilter()
	savedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, canceledreaders.BuildQuery().SnapshotType(), filter)
	assert.NoError(t, err, "Should be able to load saved snapshot")
	assert.NotNil(t, savedSnapshot, "Snapshot should exist after first query")
	assert.Equal(t, uint(2), savedSnapshot.SequenceNumber, "Snapshot should have sequence=2")

	// Add a SECOND canceled reader (this will create sequence=4, register + cancel)
	createSecondTestCanceledReader(ctx, t, wrapper)

	// act - second query (should hit snapshot and process incremental events)
	result2, err := snapshotHandler.Handle(ctx, canceledreaders.BuildQuery())

	// assert
	assert.NoError(t, err, "Second query should work")
	assert.Equal(t, 2, result2.Count, "Should have 2 canceled readers after incremental processing")
	assert.Equal(t, uint(4), result2.SequenceNumber, "Should have sequence=4 after processing new events")

	// Verify we have both canceled readers (the first reader should be at index 0 due to earlier timestamp)
	assert.Equal(t, result1.Readers[0].ReaderID, result2.Readers[0].ReaderID, "First canceled reader should still be present")
	assert.Len(t, result2.Readers, 2, "Should have exactly 2 canceled readers")
	// The second canceled reader should be different from the first
	assert.NotEqual(t, result2.Readers[0].ReaderID, result2.Readers[1].ReaderID, "Second canceled reader should be different")

	// Wait for the async snapshot update to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that the snapshot was updated with incremental changes
	filter = canceledreaders.BuildEventFilter()
	updatedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, canceledreaders.BuildQuery().SnapshotType(), filter)
	assert.NoError(t, err, "Should be able to load updated snapshot")
	assert.NotNil(t, updatedSnapshot, "Updated snapshot should exist")
	assert.Equal(t, uint(4), updatedSnapshot.SequenceNumber, "Updated snapshot should have sequence=4")

	// Deserialize and verify the updated snapshot contains incremental data (2 canceled readers)
	var updatedProjection canceledreaders.CanceledReaders
	err = jsoniter.ConfigFastest.Unmarshal(updatedSnapshot.Data, &updatedProjection)
	assert.NoError(t, err, "Should be able to deserialize updated snapshot")
	assert.Equal(t, 2, updatedProjection.Count, "Updated snapshot should contain 2 canceled readers")
	assert.Equal(t, uint(4), updatedProjection.SequenceNumber, "Updated snapshot projection should have sequence=4")
}

func setupSnapshotTest(t *testing.T) (
	context.Context,
	*snapshot.QueryWrapper[canceledreaders.Query, canceledreaders.CanceledReaders],
	Wrapper,
) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	wrapper := CreateWrapperWithTestConfig(t)
	t.Cleanup(wrapper.Close)
	CleanUp(t, wrapper)

	coreHandler := canceledreaders.NewQueryHandler(wrapper.GetEventStore())

	snapshotHandler, err := snapshot.NewQueryWrapper(
		coreHandler,
		wrapper.GetEventStore(),
		canceledreaders.Project,
		func(_ canceledreaders.Query) eventstore.Filter {
			return canceledreaders.BuildEventFilter()
		},
	)
	assert.NoError(t, err, "Should create snapshot-aware query handler")

	return ctx, snapshotHandler, wrapper
}

// Helper function to create a canceled reader (register and cancel events).
func createTestCanceledReader(ctx context.Context, t *testing.T, wrapper Wrapper) {
	t.Helper()

	readerID := GivenUniqueID(t)
	fakeClock := time.Unix(0, 0).UTC()

	// Register a reader first
	registerReaderCmd := registerreader.BuildCommand(readerID, "Test Reader", fakeClock)
	readerHandler := registerreader.NewCommandHandler(wrapper.GetEventStore())
	_, err := readerHandler.Handle(ctx, registerReaderCmd)
	assert.NoError(t, err, "Should register reader")

	// Then cancel the reader
	cancelReaderCmd := cancelreadercontract.BuildCommand(readerID, fakeClock.Add(time.Minute))
	cancelHandler := cancelreadercontract.NewCommandHandler(wrapper.GetEventStore())
	_, err = cancelHandler.Handle(ctx, cancelReaderCmd)
	assert.NoError(t, err, "Should cancel reader")
}

// Helper function to create a second canceled reader for testing incremental snapshot updates.
func createSecondTestCanceledReader(ctx context.Context, t *testing.T, wrapper Wrapper) {
	t.Helper()

	readerID := GivenUniqueID(t)
	fakeClock := time.Unix(1, 0).UTC() // Slightly different timestamp

	// Register a second reader first
	registerReaderCmd := registerreader.BuildCommand(readerID, "Second Reader", fakeClock)
	readerHandler := registerreader.NewCommandHandler(wrapper.GetEventStore())
	_, err := readerHandler.Handle(ctx, registerReaderCmd)
	assert.NoError(t, err, "Should register second reader")

	// Then cancel the second reader
	cancelReaderCmd := cancelreadercontract.BuildCommand(readerID, fakeClock.Add(time.Minute))
	cancelHandler := cancelreadercontract.NewCommandHandler(wrapper.GetEventStore())
	_, err = cancelHandler.Handle(ctx, cancelReaderCmd)
	assert.NoError(t, err, "Should cancel second reader")
}
