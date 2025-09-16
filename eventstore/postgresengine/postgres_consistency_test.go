package postgresengine_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper"
)

func Test_ConsistencyRouting_DefaultsToStrongConsistency(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupConsistencyTestEnvironment(t)
	defer cleanup()

	eventStore := wrapper.GetEventStore()
	bookID := helper.GivenUniqueID(t)
	filter := helper.FilterAllEventTypesForOneBook(bookID)

	// Create a test event first
	testEvent := helper.ToStorable(t, helper.FixtureBookCopyAddedToCirculation(bookID, time.Now()))
	appendErr := eventStore.Append(ctx, filter, 0, testEvent)
	assert.NoError(t, appendErr, "Should append test event")

	// act - Query without explicit consistency context (should default to strong consistency)
	events, maxSeq, err := eventStore.Query(ctx, filter)

	// assert - Should work fine with default routing (strong consistency to primary)
	assert.NoError(t, err, "Query should succeed with default consistency")
	assert.NotNil(t, events, "Should return events")
	assert.Equal(t, eventstore.MaxSequenceNumberUint(1), maxSeq, "Should return correct max sequence")
	assert.Len(t, events, 1, "Should find the appended event")
}

func Test_ConsistencyRouting_RespectsExplicitConsistency(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupConsistencyTestEnvironment(t)
	defer cleanup()

	eventStore := wrapper.GetEventStore()
	bookID := helper.GivenUniqueID(t)
	filter := helper.FilterAllEventTypesForOneBook(bookID)

	// Create a test event first
	testEvent := helper.ToStorable(t, helper.FixtureBookCopyAddedToCirculation(bookID, time.Now()))
	appendErr := eventStore.Append(ctx, filter, 0, testEvent)
	assert.NoError(t, appendErr, "Should append test event")

	// Test 1: Explicit strong consistency
	strongCtx := eventstore.WithStrongConsistency(ctx)
	strongEvents, strongMaxSeq, strongErr := eventStore.Query(strongCtx, filter)

	assert.NoError(t, strongErr, "Query should succeed with explicit strong consistency")
	assert.NotNil(t, strongEvents, "Should return events with strong consistency")
	assert.Equal(t, eventstore.MaxSequenceNumberUint(1), strongMaxSeq, "Should return correct max sequence")
	assert.Len(t, strongEvents, 1, "Should find the appended event with strong consistency")

	// Test 2: Explicit eventual consistency
	eventualCtx := eventstore.WithEventualConsistency(ctx)
	eventualEvents, eventualMaxSeq, eventualErr := eventStore.Query(eventualCtx, filter)

	assert.NoError(t, eventualErr, "Query should succeed with explicit eventual consistency")
	assert.NotNil(t, eventualEvents, "Should return events with eventual consistency")
	assert.Equal(t, eventstore.MaxSequenceNumberUint(1), eventualMaxSeq, "Should return correct max sequence")
	assert.Len(t, eventualEvents, 1, "Should find the appended event with eventual consistency")

	// Both approaches should return the same data (since there is no replica lag in the test environment)
	assert.Equal(t, len(strongEvents), len(eventualEvents), "Both consistency levels should return same number of events")
}

func Test_ConsistencyRouting_SnapshotOperationsWorkCorrectly(t *testing.T) {
	// setup
	ctx, wrapper, cleanup := setupConsistencyTestEnvironment(t)
	defer cleanup()

	eventStore := wrapper.GetEventStore()
	bookID := helper.GivenUniqueID(t)
	filter := helper.FilterAllEventTypesForOneBook(bookID)

	// Create a test snapshot
	snapshot := eventstore.Snapshot{
		ProjectionType: "TestProjection",
		FilterHash:     filter.Hash(),
		SequenceNumber: 1,
		Data:           []byte(`{"test": "data"}`),
		CreatedAt:      time.Now(),
	}

	// Test 1: Save snapshot (should always use primary)
	saveErr := eventStore.SaveSnapshot(ctx, snapshot)
	assert.NoError(t, saveErr, "Should save snapshot successfully")

	// Test 2: Load snapshot with default consistency (should use primary by default)
	loadedSnapshot, loadErr := eventStore.LoadSnapshot(ctx, "TestProjection", filter)
	assert.NoError(t, loadErr, "Should load snapshot with default consistency")
	assert.NotNil(t, loadedSnapshot, "Should return snapshot")
	assert.Equal(t, snapshot.ProjectionType, loadedSnapshot.ProjectionType, "Should load correct snapshot")

	// Test 3: Load snapshot with explicit eventual consistency (should work with replica if available)
	eventualCtx := eventstore.WithEventualConsistency(ctx)
	eventualSnapshot, eventualErr := eventStore.LoadSnapshot(eventualCtx, "TestProjection", filter)
	assert.NoError(t, eventualErr, "Should load snapshot with eventual consistency")
	assert.NotNil(t, eventualSnapshot, "Should return snapshot with eventual consistency")
	assert.Equal(t, snapshot.ProjectionType, eventualSnapshot.ProjectionType, "Should load same snapshot")

	// Test 4: Load snapshot with explicit strong consistency
	strongCtx := eventstore.WithStrongConsistency(ctx)
	strongSnapshot, strongErr := eventStore.LoadSnapshot(strongCtx, "TestProjection", filter)
	assert.NoError(t, strongErr, "Should load snapshot with strong consistency")
	assert.NotNil(t, strongSnapshot, "Should return snapshot with strong consistency")
	assert.Equal(t, snapshot.ProjectionType, strongSnapshot.ProjectionType, "Should load same snapshot")
}

// Test setup helpers.
func setupConsistencyTestEnvironment(t *testing.T) (context.Context, postgreswrapper.Wrapper, func()) {
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	wrapper := postgreswrapper.CreateWrapperWithTestConfig(t)
	postgreswrapper.CleanUp(t, wrapper)

	cleanup := func() {
		cancel()
		wrapper.Close()
	}

	return ctxWithTimeout, wrapper, cleanup
}
