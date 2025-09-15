package registeredreaders_test

import (
	"context"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/registerreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/query/registeredreaders"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/snapshot"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

func Test_SnapshotAwareQueryHandler_Handle_SnapshotMiss(t *testing.T) {
	// setup
	ctx, snapshotHandler, wrapper := setupSnapshotTest(t)

	// arrange
	createTestReader(ctx, t, wrapper)

	// act
	result, err := snapshotHandler.Handle(ctx, registeredreaders.BuildQuery())
	assert.NoError(t, err, "Snapshot handler should work")
	assert.Equal(t, 1, result.Count, "Should have 1 reader")
}

func Test_SnapshotAwareQueryHandler_Handle_SnapshotCreationAndHitWithNoNewEvents(t *testing.T) {
	// setup
	ctx, snapshotHandler, wrapper := setupSnapshotTest(t)

	// arrange
	createTestReader(ctx, t, wrapper)

	// act / assert

	// First query: Should miss snapshot and fall back to base handler
	// If wrapper works correctly, it should create a snapshot after a successful fallback
	query := registeredreaders.BuildQuery()

	result, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "First query should work")
	assert.Equal(t, 1, result.Count, "Should have 1 reader")

	// Give snapshot saving some time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that a snapshot was created in the database
	filter := registeredreaders.BuildEventFilter()
	savedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, query.SnapshotType(), filter)
	assert.NoError(t, err, "Should be able to load saved snapshot")
	assert.NotNil(t, savedSnapshot, "Snapshot should exist after first query")

	// Second query: Should hit snapshot (no new events since last query)
	result2, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "Second query should work")
	assert.Equal(t, 1, result2.Count, "Should still have 1 reader")
	assert.Equal(t, result.SequenceNumber, result2.SequenceNumber, "Both results should have same sequence number")
}

func Test_SnapshotAwareQueryHandler_Handle_IncrementalUpdateWithNewEvents(t *testing.T) {
	// setup
	ctx, snapshotHandler, wrapper := setupSnapshotTest(t)

	// arrange: create the first test reader (will create sequence=1)
	createTestReader(ctx, t, wrapper)

	// act / assert

	// First query: Should miss snapshot and fall back to base handler, then create snapshot
	query := registeredreaders.BuildQuery()

	result1, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "First query should work")
	assert.Equal(t, 1, result1.Count, "Should have 1 reader initially")
	assert.Equal(t, uint(1), result1.SequenceNumber, "Should have sequence=1")

	// Give snapshot saving some time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that a snapshot was created in the database
	filter := registeredreaders.BuildEventFilter()
	savedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, query.SnapshotType(), filter)
	assert.NoError(t, err, "Should be able to load saved snapshot")
	assert.NotNil(t, savedSnapshot, "Snapshot should exist after first query")

	// arrange: create a second test reader (will create sequence=2)
	createSecondTestReader(ctx, t, wrapper)

	// act / assert

	// Second query: Should hit snapshot and do incremental update with new events
	result2, err := snapshotHandler.Handle(ctx, query)
	assert.NoError(t, err, "Second query should work")
	assert.Equal(t, 2, result2.Count, "Should have 2 readers after incremental update")
	assert.Equal(t, uint(2), result2.SequenceNumber, "Should have sequence=2 after processing new events")

	// Verify we have both readers (the first reader should be at index 0 due to earlier timestamp)
	assert.Equal(t, result1.Readers[0].ReaderID, result2.Readers[0].ReaderID, "First reader should still be present")
	assert.Len(t, result2.Readers, 2, "Should have exactly 2 readers")
	// The second reader should be different from the first
	assert.NotEqual(t, result2.Readers[0].ReaderID, result2.Readers[1].ReaderID, "Second reader should be different reader")

	// Verify the snapshot was updated with incremental changes
	time.Sleep(100 * time.Millisecond)
	updatedSnapshot, err := wrapper.GetEventStore().LoadSnapshot(ctx, query.SnapshotType(), filter)
	assert.NoError(t, err, "Should be able to load updated snapshot")
	assert.NotNil(t, updatedSnapshot, "Updated snapshot should exist")
	assert.Equal(t, uint(2), updatedSnapshot.SequenceNumber, "Updated snapshot should have sequence=2")

	// Verify the updated snapshot contains the correct data
	var snapshotData registeredreaders.RegisteredReaders
	err = jsoniter.ConfigFastest.Unmarshal(updatedSnapshot.Data, &snapshotData)
	assert.NoError(t, err, "Should unmarshal snapshot data")
	assert.Equal(t, 2, snapshotData.Count, "Snapshot should contain 2 readers")
	assert.Len(t, snapshotData.Readers, 2, "Snapshot should have 2 reader entries")
}

// Test helpers

func setupSnapshotTest(t *testing.T) (
	context.Context,
	*snapshot.QueryWrapper[registeredreaders.Query, registeredreaders.RegisteredReaders],
	Wrapper,
) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	wrapper := CreateWrapperWithTestConfig(t)
	t.Cleanup(wrapper.Close)
	CleanUp(t, wrapper)

	baseHandler := registeredreaders.NewQueryHandler(wrapper.GetEventStore())

	snapshotHandler, err := snapshot.NewQueryWrapper[
		registeredreaders.Query,
		registeredreaders.RegisteredReaders,
	](
		baseHandler,
		wrapper.GetEventStore(),
		registeredreaders.Project,
		func(_ registeredreaders.Query) eventstore.Filter {
			return registeredreaders.BuildEventFilter()
		},
	)
	assert.NoError(t, err, "Should create snapshot-aware query handler")

	return ctx, snapshotHandler, wrapper
}

func createTestReader(ctx context.Context, t *testing.T, wrapper Wrapper) {
	t.Helper()

	readerID := GivenUniqueID(t)
	fakeClock := time.Unix(0, 0).UTC()

	registerReaderCmd := registerreader.BuildCommand(readerID, "Test Reader", fakeClock)
	readerHandler := registerreader.NewCommandHandler(wrapper.GetEventStore())
	_, err := readerHandler.Handle(ctx, registerReaderCmd)
	assert.NoError(t, err, "Should register reader")
}

func createSecondTestReader(ctx context.Context, t *testing.T, wrapper Wrapper) {
	t.Helper()

	readerID := GivenUniqueID(t)
	fakeClock := time.Unix(1, 0).UTC() // Slightly different timestamp

	registerReaderCmd := registerreader.BuildCommand(readerID, "Second Reader", fakeClock)
	readerHandler := registerreader.NewCommandHandler(wrapper.GetEventStore())
	_, err := readerHandler.Handle(ctx, registerReaderCmd)
	assert.NoError(t, err, "Should register second reader")
}
