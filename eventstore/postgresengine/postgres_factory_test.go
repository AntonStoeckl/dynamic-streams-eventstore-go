package postgresengine_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq" // postgres driver
	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/config"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

func Test_FactoryFunctions_NewEventStore_ShouldPanic_WithUnsupportedAdapterType(t *testing.T) {
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
		createErr := TryCreateEventStoreWithTableName(t, postgresengine.WithTableName("event_data"))
		assert.NoError(t, createErr)
	})
}

func Test_FactoryFunctions_NewEventStoreWithTableName_ShouldPanic_WithUnsupportedAdapterType(t *testing.T) {
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
		createErr := TryCreateEventStoreWithTableName(t, postgresengine.WithTableName("event_data"))
		assert.NoError(t, createErr)
	})
}

func Test_FactoryFunctions_NewEventStore_ShouldFail_WithNilDatabaseConnection(t *testing.T) {
	testCases := []struct {
		name        string
		factoryFunc func() (*postgresengine.EventStore, error)
	}{
		{
			name: "NewEventStoreFromPGXPool with nil",
			factoryFunc: func() (*postgresengine.EventStore, error) {
				return postgresengine.NewEventStoreFromPGXPool(nil)
			},
		},
		{
			name: "NewEventStoreFromSQLDB with nil",
			factoryFunc: func() (*postgresengine.EventStore, error) {
				return postgresengine.NewEventStoreFromSQLDB(nil)
			},
		},
		{
			name: "NewEventStoreFromSQLX with nil",
			factoryFunc: func() (*postgresengine.EventStore, error) {
				return postgresengine.NewEventStoreFromSQLX(nil)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// act
			_, err := tc.factoryFunc()

			// assert
			assert.ErrorContains(t, err, eventstore.ErrNilDatabaseConnection.Error())
		})
	}
}

func Test_FactoryFunctions_EventStore_WithTableName_ShouldWorkCorrectly(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	customTableName := "events"
	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithTableName(customTableName))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUp(t, wrapper)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	err := es.Append(
		ctxWithTimeout,
		filter,
		0,
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock)),
	)
	assert.NoError(t, err)

	// act
	events, _, queryErr := es.Query(ctxWithTimeout, filter)

	// assert
	assert.NoError(t, queryErr)
	assert.Len(t, events, 1)
}

func Test_FactoryFunctions_FactoryFunctions_ShouldFail_WithEmptyTableName(t *testing.T) {
	testCases := []struct {
		name        string
		factoryFunc func(t *testing.T) (*postgresengine.EventStore, error)
	}{
		{
			name: "NewEventStoreFromPGXPool with empty table name",
			factoryFunc: func(_ *testing.T) (*postgresengine.EventStore, error) {
				connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolTestConfig())
				assert.NoError(t, err, "error connecting to DB pool in test setup")
				defer connPool.Close()

				return postgresengine.NewEventStoreFromPGXPool(connPool, postgresengine.WithTableName(""))
			},
		},
		{
			name: "NewEventStoreFromSQLDB with empty table name",
			factoryFunc: func(_ *testing.T) (*postgresengine.EventStore, error) {
				db := config.PostgresSQLDBTestConfig()
				defer func() { _ = db.Close() }()

				return postgresengine.NewEventStoreFromSQLDB(db, postgresengine.WithTableName(""))
			},
		},
		{
			name: "NewEventStoreFromSQLX with empty table name",
			factoryFunc: func(_ *testing.T) (*postgresengine.EventStore, error) {
				db := config.PostgresSQLXTestConfig()
				defer func() { _ = db.Close() }()

				return postgresengine.NewEventStoreFromSQLX(db, postgresengine.WithTableName(""))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// act
			_, err := tc.factoryFunc(t)

			// assert
			assert.ErrorContains(t, err, eventstore.ErrEmptyEventsTableName.Error())
		})
	}
}

func Test_FactoryFunctions_EventStore_WithTableName_ShouldFail_WithNonExistentTable(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wrapper := CreateWrapperWithTestConfig(t, postgresengine.WithTableName("non_existent_table_1"))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	_, _, err := es.Query(ctxWithTimeout, filter)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}
