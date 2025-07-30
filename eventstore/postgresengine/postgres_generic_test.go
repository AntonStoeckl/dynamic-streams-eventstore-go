package postgresengine_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq" // postgres driver
	"github.com/stretchr/testify/assert"

	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/config"
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
		createErr := TryCreateEventStoreWithTableName(t, "event_data")
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
		createErr := TryCreateEventStoreWithTableName(t, "event_data")
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
				connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolTestConfig())
				assert.NoError(t, err, "error connecting to DB pool in test setup")
				defer connPool.Close()

				return NewEventStoreFromPGXPool(connPool, WithTableName(""))
			},
		},
		{
			name: "NewEventStoreFromSQLDB with empty table name",
			factoryFunc: func(t *testing.T) (EventStore, error) {
				db := config.PostgresSQLDBTestConfig()
				defer func() { _ = db.Close() }()

				return NewEventStoreFromSQLDB(db, WithTableName(""))
			},
		},
		{
			name: "NewEventStoreFromSQLX with empty table name",
			factoryFunc: func(t *testing.T) (EventStore, error) {
				db := config.PostgresSQLXTestConfig()
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
