package postgresengine_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/pgtesthelpers"
)

func Test_SaveAndLoad_Snapshot(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := helper.GivenUniqueID(t)
	filter := helper.FilterAllEventTypesForOneBook(bookID)

	projectionData := `{"books":[{"bookID":"book-123","title":"Test Book","count":1}],"count":1}`
	snapshot, err := eventstore.BuildSnapshot(
		"BooksInCirculation",
		filter.Hash(),
		42,
		[]byte(projectionData),
	)
	assert.NoError(t, err, "Building snapshot should succeed")

	// act
	saveErr := es.SaveSnapshot(ctxWithTimeout, snapshot)

	// assert
	assert.NoError(t, saveErr, "Saving snapshot should succeed")

	loadedSnapshot, loadErr := es.LoadSnapshot(ctxWithTimeout, "BooksInCirculation", filter)

	assert.NoError(t, loadErr, "Loading snapshot should succeed")
	assert.NotNil(t, loadedSnapshot, "Loaded snapshot should not be nil")
	assert.Equal(t, snapshot.ProjectionType, loadedSnapshot.ProjectionType)
	assert.Equal(t, snapshot.FilterHash, loadedSnapshot.FilterHash)
	assert.Equal(t, snapshot.SequenceNumber, loadedSnapshot.SequenceNumber)
	assert.JSONEq(t, string(snapshot.Data), string(loadedSnapshot.Data))
	assert.WithinDuration(t, snapshot.CreatedAt, loadedSnapshot.CreatedAt, time.Second)
}

func Test_LoadSnapshot_IfSnapshotIs_NotFound(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := helper.GivenUniqueID(t)
	filter := helper.FilterAllEventTypesForOneBook(bookID)

	// act
	loadedSnapshot, loadErr := es.LoadSnapshot(ctxWithTimeout, "NonExistentProjection", filter)

	// assert
	assert.NoError(t, loadErr, "LoadSnapshot should not return error for not found")
	assert.Nil(t, loadedSnapshot, "No snapshot should be returned when not found")
}

//nolint:funlen
func Test_Snapshot_SaveSnapshot_ValidatesInput(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	tests := []struct {
		name          string
		snapshot      func() eventstore.Snapshot
		expectedError error
	}{
		{
			name: "empty_projection_type",
			snapshot: func() eventstore.Snapshot {
				return eventstore.Snapshot{
					ProjectionType: "",
					FilterHash:     "sha256:test",
					SequenceNumber: 1,
					Data:           []byte(`{}`),
					CreatedAt:      time.Now(),
				}
			},
			expectedError: eventstore.ErrEmptyProjectionType,
		},
		{
			name: "empty_filter_hash",
			snapshot: func() eventstore.Snapshot {
				return eventstore.Snapshot{
					ProjectionType: "TestProjection",
					FilterHash:     "",
					SequenceNumber: 1,
					Data:           []byte(`{}`),
					CreatedAt:      time.Now(),
				}
			},
			expectedError: eventstore.ErrEmptyFilterHash,
		},
		{
			name: "invalid_json_data",
			snapshot: func() eventstore.Snapshot {
				return eventstore.Snapshot{
					ProjectionType: "TestProjection",
					FilterHash:     "sha256:test",
					SequenceNumber: 1,
					Data:           []byte(`{invalid json`),
					CreatedAt:      time.Now(),
				}
			},
			expectedError: eventstore.ErrInvalidSnapshotJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			pgtesthelpers.CleanUp(t, wrapper) //nolint:contextcheck
			snapshot := tt.snapshot()

			// act
			err := es.SaveSnapshot(ctxWithTimeout, snapshot)

			// assert
			assert.ErrorIs(t, err, tt.expectedError)
		})
	}
}

func Test_Snapshot_PreservesHigherSequence_WhenTryToUpsertWithLowerSequence(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := helper.GivenUniqueID(t)
	filter := helper.FilterAllEventTypesForOneBook(bookID)

	initialSnapshot, err := eventstore.BuildSnapshot(
		"BooksInCirculation",
		filter.Hash(),
		100,
		[]byte(`{"books":[],"count":0}`),
	)
	assert.NoError(t, err)

	err = es.SaveSnapshot(ctxWithTimeout, initialSnapshot)
	assert.NoError(t, err, "Initial snapshot save should succeed")

	// act
	lowerSeqSnapshot, err := eventstore.BuildSnapshot(
		"BooksInCirculation",
		filter.Hash(),
		50,
		[]byte(`{"books":[{"title":"Should not appear"}],"count":1}`),
	)
	assert.NoError(t, err)

	err = es.SaveSnapshot(ctxWithTimeout, lowerSeqSnapshot)

	// assert
	assert.NoError(t, err, "Second snapshot save should succeed without error")
	loadedSnapshot, loadErr := es.LoadSnapshot(ctxWithTimeout, "BooksInCirculation", filter)
	assert.NoError(t, loadErr, "Loading snapshot should succeed")
	assert.Equal(t, eventstore.MaxSequenceNumberUint(100), loadedSnapshot.SequenceNumber,
		"Should preserve higher sequence number")
	assert.JSONEq(t, `{"books":[],"count":0}`, string(loadedSnapshot.Data),
		"Should preserve data from snapshot with higher sequence number")
}

func Test_Snapshot_ConcurrentSave_SequenceProtection(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := helper.GivenUniqueID(t)
	filter := helper.FilterAllEventTypesForOneBook(bookID)

	projectionType := "ConcurrentTest"
	numGoroutines := uint(10)
	var wg sync.WaitGroup
	var successCount atomic.Int32

	// act
	for i := uint(0); i < numGoroutines; i++ {
		wg.Add(1)
		sequenceNumber := i + 1

		go func(seq eventstore.MaxSequenceNumberUint) {
			defer wg.Done()

			snapshot, buildErr := eventstore.BuildSnapshot(
				projectionType,
				filter.Hash(),
				seq,
				[]byte(fmt.Sprintf(`{"sequence":%d}`, seq)),
			)
			if buildErr != nil {
				t.Errorf("Building snapshot failed: %v", buildErr)
				return
			}

			saveErr := es.SaveSnapshot(ctxWithTimeout, snapshot)
			if saveErr != nil {
				t.Errorf("Saving snapshot failed: %v", saveErr)
				return
			}

			successCount.Add(1)
		}(sequenceNumber)
	}

	wg.Wait()

	// assert
	assert.Equal(t, int32(numGoroutines), successCount.Load(),
		"All goroutines should complete successfully")

	loadedSnapshot, loadErr := es.LoadSnapshot(ctxWithTimeout, projectionType, filter)
	assert.NoError(t, loadErr, "Loading final snapshot should succeed")
	assert.Equal(t, numGoroutines, loadedSnapshot.SequenceNumber,
		"Final snapshot should have the highest sequence number")
}

func Test_Snapshots_WithDifferentFilters_CreateDifferentSnapshots(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID1 := helper.GivenUniqueID(t)
	bookID2 := helper.GivenUniqueID(t)
	filter1 := helper.FilterAllEventTypesForOneBook(bookID1)
	filter2 := helper.FilterAllEventTypesForOneBook(bookID2)

	// act
	snapshot1, err := eventstore.BuildSnapshot(
		"BooksInCirculation",
		filter1.Hash(),
		10,
		[]byte(`{"filter":"one"}`),
	)
	assert.NoError(t, err)
	err = es.SaveSnapshot(ctxWithTimeout, snapshot1)
	assert.NoError(t, err)

	snapshot2, err := eventstore.BuildSnapshot(
		"BooksInCirculation",
		filter2.Hash(),
		20,
		[]byte(`{"filter":"two"}`),
	)
	assert.NoError(t, err)
	err = es.SaveSnapshot(ctxWithTimeout, snapshot2)
	assert.NoError(t, err)

	// assert
	loaded1, err := es.LoadSnapshot(ctxWithTimeout, "BooksInCirculation", filter1)
	assert.NoError(t, err)
	assert.Equal(t, eventstore.MaxSequenceNumberUint(10), loaded1.SequenceNumber)
	assert.JSONEq(t, `{"filter":"one"}`, string(loaded1.Data))

	loaded2, err := es.LoadSnapshot(ctxWithTimeout, "BooksInCirculation", filter2)
	assert.NoError(t, err)
	assert.Equal(t, eventstore.MaxSequenceNumberUint(20), loaded2.SequenceNumber)
	assert.JSONEq(t, `{"filter":"two"}`, string(loaded2.Data))
}

func Test_Snapshots_WithSameFilter_UpsertsSnapshot(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := helper.GivenUniqueID(t)
	filter := helper.FilterAllEventTypesForOneBook(bookID)

	initialSnapshot, err := eventstore.BuildSnapshot(
		"BooksInCirculation",
		filter.Hash(),
		10,
		[]byte(`{"version":"initial"}`),
	)
	assert.NoError(t, err)
	err = es.SaveSnapshot(ctxWithTimeout, initialSnapshot)
	assert.NoError(t, err)

	// act
	updatedSnapshot, err := eventstore.BuildSnapshot(
		"BooksInCirculation",
		filter.Hash(),
		20,
		[]byte(`{"version":"updated"}`),
	)
	assert.NoError(t, err)
	err = es.SaveSnapshot(ctxWithTimeout, updatedSnapshot)
	assert.NoError(t, err)

	// assert
	loaded, loadErr := es.LoadSnapshot(ctxWithTimeout, "BooksInCirculation", filter)
	assert.NoError(t, loadErr)
	assert.Equal(t, eventstore.MaxSequenceNumberUint(20), loaded.SequenceNumber)
	assert.JSONEq(t, `{"version":"updated"}`, string(loaded.Data))
}

//nolint:funlen
func Test_SaveSnapshot_WithLargeJSONB_WithinLimits(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := helper.GivenUniqueID(t)
	filter := helper.FilterAllEventTypesForOneBook(bookID)

	// Create large JSON data (simulate ~1MB)
	var books []map[string]interface{}
	for i := 0; i < 2000; i++ {
		book := map[string]interface{}{
			"id":              fmt.Sprintf("book-%d", i),
			"title":           fmt.Sprintf("Book Title Number %d", i),
			"authors":         fmt.Sprintf("Author %d, Co-Author %d", i, i+1),
			"isbn":            fmt.Sprintf("978-0-123456-78-%d", i%10),
			"publisher":       fmt.Sprintf("Publisher %d", i%50),
			"publicationYear": 2000 + (i % 25),
			"isCurrentlyLent": i%3 == 0,
			"metadata": map[string]interface{}{
				"tags":        []string{"fiction", "bestseller", "award-winning"},
				"description": "A very detailed description of this book that contains many words and provides extensive information about the plot, characters, and themes.",
				"reviews":     []string{"Excellent book!", "Highly recommended.", "A masterpiece."},
			},
		}
		books = append(books, book)
	}

	projectionData := map[string]interface{}{
		"books": books,
		"count": len(books),
		"metadata": map[string]interface{}{
			"lastUpdated": time.Now().Format(time.RFC3339),
			"version":     "1.0",
			"performance": map[string]interface{}{
				"queryDurationMs": 150.5,
				"cacheHitRate":    0.95,
			},
		},
	}

	// Marshal to JSON
	jsonData, marshalErr := jsoniter.ConfigFastest.Marshal(projectionData)
	assert.NoError(t, marshalErr, "Marshaling large data should succeed")
	assert.Greater(t, len(jsonData), 500000, "JSON should be substantial size (>500KB)")

	snapshot, err := eventstore.BuildSnapshot(
		"LargeBooksInCirculation",
		filter.Hash(),
		12345,
		jsonData,
	)
	assert.NoError(t, err)

	// act
	saveErr := es.SaveSnapshot(ctxWithTimeout, snapshot)

	// assert (load)
	assert.NoError(t, saveErr, "Saving large snapshot should succeed")
	loaded, loadErr := es.LoadSnapshot(ctxWithTimeout, "LargeBooksInCirculation", filter)
	assert.NoError(t, loadErr, "Loading large snapshot should succeed")
	assert.JSONEq(t, string(jsonData), string(loaded.Data), "Large JSON data should be preserved exactly")
}

func Test_DeleteSnapshot(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := helper.GivenUniqueID(t)
	filter := helper.FilterAllEventTypesForOneBook(bookID)

	snapshot, err := eventstore.BuildSnapshot(
		"BooksInCirculation",
		filter.Hash(),
		100,
		[]byte(`{"books":[],"count":0}`),
	)
	assert.NoError(t, err)
	err = es.SaveSnapshot(ctxWithTimeout, snapshot)
	assert.NoError(t, err)

	loaded, loadErr := es.LoadSnapshot(ctxWithTimeout, "BooksInCirculation", filter)
	assert.NoError(t, loadErr)
	assert.NotNil(t, loaded)

	// act
	deleteErr := es.DeleteSnapshot(ctxWithTimeout, "BooksInCirculation", filter)

	// assert
	assert.NoError(t, deleteErr, "Deleting snapshot should succeed")
	deletedSnapshot, loadErr := es.LoadSnapshot(ctxWithTimeout, "BooksInCirculation", filter)
	assert.NoError(t, loadErr, "LoadSnapshot should not return error for not found")
	assert.Nil(t, deletedSnapshot, "Snapshot should no longer exist")
}

func Test_DeleteSnapshot_Is_Idempotent(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := helper.GivenUniqueID(t)
	filter := helper.FilterAllEventTypesForOneBook(bookID)

	// act
	deleteErr := es.DeleteSnapshot(ctxWithTimeout, "NonExistentProjection", filter)

	// assert
	assert.NoError(t, deleteErr, "Deleting non-existent snapshot should be idempotent")
}

func Test_Snapshot_Context_Cancellation(t *testing.T) {
	// setup
	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := helper.GivenUniqueID(t)
	filter := helper.FilterAllEventTypesForOneBook(bookID)

	snapshot, err := eventstore.BuildSnapshot(
		"BooksInCirculation",
		filter.Hash(),
		1,
		[]byte(`{}`),
	)
	assert.NoError(t, err)

	ctxWithCancel, cancel := context.WithCancel(context.Background())

	// act
	cancel() // Cancel context immediately

	// Test SaveSnapshot with canceled context
	saveErr := es.SaveSnapshot(ctxWithCancel, snapshot)
	assert.ErrorContains(t, saveErr, "context canceled", "Save should fail with cancelled context")

	// Test LoadSnapshot with canceled context
	_, loadErr := es.LoadSnapshot(ctxWithCancel, "BooksInCirculation", filter)
	assert.ErrorContains(t, loadErr, "context canceled", "Load should fail with cancelled context")

	// Test DeleteSnapshot with canceled context
	deleteErr := es.DeleteSnapshot(ctxWithCancel, "BooksInCirculation", filter)
	assert.ErrorContains(t, deleteErr, "context canceled", "Delete should fail with cancelled context")
}
