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

	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/config"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper"
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
			factoryFunc: func(t *testing.T) (EventStore, error) {
				connPool, err := pgxpool.NewWithConfig(context.Background(), PostgresPGXPoolTestConfig())
				assert.NoError(t, err, "error connecting to DB pool in test setup")
				defer connPool.Close()

				return NewEventStoreFromPGXPool(connPool, WithTableName(""))
			},
		},
		{
			name: "NewEventStoreFromSQLDB with empty table name",
			factoryFunc: func(t *testing.T) (EventStore, error) {
				db := PostgresSQLDBTestConfig()
				defer func() { _ = db.Close() }()

				return NewEventStoreFromSQLDB(db, WithTableName(""))
			},
		},
		{
			name: "NewEventStoreFromSQLX with empty table name",
			factoryFunc: func(t *testing.T) (EventStore, error) {
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

	testHandler := NewTestLogHandler()
	logger := slog.New(testHandler)

	wrapper := CreateWrapperWithTestConfig(t, WithSQLQueryLogger(logger))
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
	assert.Equal(t, 1, testHandler.GetRecordCount(), "query should log exactly one SQL statement")
	assert.True(t, testHandler.HasDebugLogWithMessage("executed sql for: query"), "should log with correct message")
	assert.True(t, testHandler.HasDebugLogWithDurationNS("executed sql for: query"), "should log with duration_ns attribute")
}

func Test_Generic_Eventstore_WithLogger_LogsAppends(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testHandler := NewTestLogHandler()
	logger := slog.New(testHandler)

	wrapper := CreateWrapperWithTestConfig(t, WithSQLQueryLogger(logger))
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
	assert.Equal(t, 2, testHandler.GetRecordCount(), "query and append should log exactly one sql statement each")
	assert.True(t, testHandler.HasDebugLogWithMessage("executed sql for: query"), "Should log with correct message")
	assert.True(t, testHandler.HasDebugLogWithMessage("executed sql for: append"), "Should log with correct message")
	assert.True(t, testHandler.HasDebugLogWithDurationNS("executed sql for: query"), "Should log query with duration_ns attribute")
	assert.True(t, testHandler.HasDebugLogWithDurationNS("executed sql for: append"), "Should log append with duration_ns attribute")
}
