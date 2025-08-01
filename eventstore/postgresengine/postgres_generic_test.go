package postgresengine_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq" // postgres driver
	"github.com/stretchr/testify/assert"

	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"                                     //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"                      //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/config"                    //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

func Test_Generic_NewEventStore_ShouldPanic_WithUnsupportedAdapterType(t *testing.T) {
	// Save the original env var
	originalAdapterType := os.Getenv("ADAPTER_TYPE")
	defer func() {
		if originalAdapterType == "" {
			err := os.Unsetenv("ADAPTER_TYPE")
			assert.NoError(t, err)
		} else {
			err := os.Setenv("ADAPTER_TYPE", originalAdapterType)
			assert.NoError(t, err)
		}
	}()

	// Set an unsupported adapter type
	err := os.Setenv("ADAPTER_TYPE", "unsupported")
	assert.NoError(t, err)

	assert.Panics(t, func() {
		createErr := TryCreateEventStoreWithTableName(t, WithTableName("event_data"))
		assert.NoError(t, createErr)
	})
}

func Test_Generic_NewEventStoreWithTableName_ShouldPanic_WithUnsupportedAdapterType(t *testing.T) {
	// Save the original env var
	originalAdapterType := os.Getenv("ADAPTER_TYPE")
	defer func() {
		if originalAdapterType == "" {
			err := os.Unsetenv("ADAPTER_TYPE")
			assert.NoError(t, err)
		} else {
			err := os.Setenv("ADAPTER_TYPE", originalAdapterType)
			assert.NoError(t, err)
		}
	}()

	// Set an unsupported adapter type
	err := os.Setenv("ADAPTER_TYPE", "unsupported")
	assert.NoError(t, err)

	assert.Panics(t, func() {
		createErr := TryCreateEventStoreWithTableName(t, WithTableName("event_data"))
		assert.NoError(t, createErr)
	})
}

func Test_Generic_NewEventStore_ShouldFail_WithNilDatabaseConnection(t *testing.T) {
	testCases := []struct {
		name        string
		factoryFunc func() (EventStore, error)
	}{
		{
			name: "NewEventStoreFromPGXPool with nil",
			factoryFunc: func() (EventStore, error) {
				return NewEventStoreFromPGXPool(nil)
			},
		},
		{
			name: "NewEventStoreFromSQLDB with nil",
			factoryFunc: func() (EventStore, error) {
				return NewEventStoreFromSQLDB(nil)
			},
		},
		{
			name: "NewEventStoreFromSQLX with nil",
			factoryFunc: func() (EventStore, error) {
				return NewEventStoreFromSQLX(nil)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// act
			_, err := tc.factoryFunc()

			// assert
			assert.ErrorContains(t, err, ErrNilDatabaseConnection.Error())
		})
	}
}

func Test_Generic_FactoryFunctions_ShouldFail_WithEmptyTableName(t *testing.T) {
	testCases := []struct {
		name        string
		factoryFunc func(t *testing.T) (EventStore, error)
	}{
		{
			name: "NewEventStoreFromPGXPool with empty table name",
			factoryFunc: func(_ *testing.T) (EventStore, error) {
				connPool, err := pgxpool.NewWithConfig(context.Background(), PostgresPGXPoolTestConfig())
				assert.NoError(t, err, "error connecting to DB pool in test setup")
				defer connPool.Close()

				return NewEventStoreFromPGXPool(connPool, WithTableName(""))
			},
		},
		{
			name: "NewEventStoreFromSQLDB with empty table name",
			factoryFunc: func(_ *testing.T) (EventStore, error) {
				db := PostgresSQLDBTestConfig()
				defer func() { _ = db.Close() }()

				return NewEventStoreFromSQLDB(db, WithTableName(""))
			},
		},
		{
			name: "NewEventStoreFromSQLX with empty table name",
			factoryFunc: func(_ *testing.T) (EventStore, error) {
				db := PostgresSQLXTestConfig()
				defer func() { _ = db.Close() }()

				return NewEventStoreFromSQLX(db, WithTableName(""))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// act
			_, err := tc.factoryFunc(t)

			// assert
			assert.ErrorContains(t, err, ErrEmptyEventsTableName.Error())
		})
	}
}

func Test_Generic_Eventstore_WithLogger_LogsQueries(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testHandler := NewTestLogHandler(false)
	logger := slog.New(testHandler)

	wrapper := CreateWrapperWithTestConfig(t, WithLogger(logger))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	_, _, err := es.Query(ctxWithTimeout, filter)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, 2, testHandler.GetRecordCount(), "query should log exactly one SQL statement and one operational statement")
	assert.True(t, testHandler.HasDebugLog("executed sql for: query"), "should log with correct message")
	assert.True(t,
		testHandler.HasDebugLogWithMessage("executed sql for: query").
			WithDurationMS().
			Assert(), "should log with duration_ms attribute",
	)
	assert.True(t,
		testHandler.HasInfoLogWithMessage("eventstore operation: query completed").
			WithDurationMS().
			WithEventCount().
			Assert(), "should log query completion with duration and event count",
	)
}

func Test_Generic_Eventstore_WithLogger_LogsAppends(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testHandler := NewTestLogHandler(false)
	logger := slog.New(testHandler)

	wrapper := CreateWrapperWithTestConfig(t, WithLogger(logger))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	err := es.Append(
		ctxWithTimeout,
		filter,
		QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter),
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock.Add(time.Second))),
	)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, 4, testHandler.GetRecordCount(), "query and append should log exactly one sql statement and one operational statement each")
	assert.True(t, testHandler.HasDebugLog("executed sql for: query"), "Should log with correct message")
	assert.True(t, testHandler.HasDebugLog("executed sql for: append"), "Should log with correct message")
	assert.True(t,
		testHandler.HasDebugLogWithMessage("executed sql for: query").
			WithDurationMS().
			Assert(), "Should log query with duration_ms attribute",
	)
	assert.True(t,
		testHandler.HasDebugLogWithMessage("executed sql for: append").
			WithDurationMS().
			Assert(), "Should log append with duration_ms attribute",
	)
	assert.True(t,
		testHandler.HasInfoLogWithMessage("eventstore operation: query completed").
			WithDurationMS().
			WithEventCount().
			Assert(), "Should log query completion with duration and event count",
	)
	assert.True(t,
		testHandler.HasInfoLogWithMessage("eventstore operation: events appended").
			WithDurationMS().
			WithEventCount().
			Assert(), "Should log append completion with duration and event count",
	)
}

func Test_Generic_Eventstore_WithLogger_LogsOperations(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testHandler := NewTestLogHandler(false)
	logger := slog.New(testHandler)

	wrapper := CreateWrapperWithTestConfig(t, WithLogger(logger))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	_, _, err := es.Query(ctxWithTimeout, filter)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, 2, testHandler.GetRecordCount(), "query should log exactly one sql statement and one operational statement")
	assert.True(t,
		testHandler.HasInfoLogWithMessage("eventstore operation: query completed").
			WithDurationMS().
			WithEventCount().
			Assert(), "should log query completion with duration and event count",
	)
}

func Test_Generic_Eventstore_WithLogger_LogsAppendOperations(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testHandler := NewTestLogHandler(false)
	logger := slog.New(testHandler)

	wrapper := CreateWrapperWithTestConfig(t, WithLogger(logger))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	err := es.Append(
		ctxWithTimeout,
		filter,
		QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter),
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock.Add(time.Second))),
	)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, 4, testHandler.GetRecordCount(), "query and append should log exactly one sql statement and one operational statement each")
	assert.True(t,
		testHandler.HasInfoLogWithMessage("eventstore operation: query completed").
			WithDurationMS().
			WithEventCount().
			Assert(), "Should log query completion with duration and event count",
	)
	assert.True(t,
		testHandler.HasInfoLogWithMessage("eventstore operation: events appended").
			WithDurationMS().
			WithEventCount().
			Assert(), "Should log append completion with duration and event count",
	)
}

func Test_Generic_Eventstore_WithLogger_LogsConcurrencyConflicts(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testHandler := NewTestLogHandler(false)
	logger := slog.New(testHandler)

	wrapper := CreateWrapperWithTestConfig(t, WithLogger(logger))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// First, add an event to establish a sequence number
	err := es.Append(
		ctxWithTimeout,
		filter,
		0, // Start with sequence 0
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock)),
	)
	assert.NoError(t, err)

	// Reset test handler to only capture the conflict
	testHandler.Reset()

	// act - try to append with the wrong expected sequence number (should cause conflict)
	err = es.Append(
		ctxWithTimeout,
		filter,
		0, // Wrong sequence number - should be 1 now
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock.Add(time.Second))),
	)

	// assert
	assert.ErrorContains(t, err, ErrConcurrencyConflict.Error())
	assert.Equal(t, 2, testHandler.GetRecordCount(), "should log exactly one sql statement and one operational statement for query")
	assert.True(t,
		testHandler.HasInfoLogWithMessage("eventstore operation: concurrency conflict detected").
			WithExpectedEvents().
			WithRowsAffected().
			WithExpectedSequence().
			Assert(), "should log concurrency conflict with expected events, rows affected, and expected sequence",
	)
}
