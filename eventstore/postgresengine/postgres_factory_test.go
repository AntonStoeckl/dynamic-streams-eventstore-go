package postgresengine_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell/config"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper"
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
		createErr := postgreswrapper.TryCreateEventStoreWithTableName(t, postgresengine.WithTableName("event_data"))
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
		createErr := postgreswrapper.TryCreateEventStoreWithTableName(t, postgresengine.WithTableName("event_data"))
		assert.NoError(t, createErr)
	})
}

//nolint:funlen
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
		{
			name: "NewEventStoreFromPGXPoolAndReplica with nil primary",
			factoryFunc: func() (*postgresengine.EventStore, error) {
				replica, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolReplicaConfig())
				if err != nil {
					return nil, fmt.Errorf("test setup failed: %w", err)
				}
				defer replica.Close()

				return postgresengine.NewEventStoreFromPGXPoolAndReplica(nil, replica)
			},
		},
		{
			name: "NewEventStoreFromSQLDBAndReplica with nil primary",
			factoryFunc: func() (*postgresengine.EventStore, error) {
				replica := config.PostgresSQLDBReplicaConfig()
				defer func() { _ = replica.Close() }()

				return postgresengine.NewEventStoreFromSQLDBAndReplica(nil, replica)
			},
		},
		{
			name: "NewEventStoreFromSQLXAndReplica with nil primary",
			factoryFunc: func() (*postgresengine.EventStore, error) {
				replica := config.PostgresSQLXReplicaConfig()
				defer func() { _ = replica.Close() }()

				return postgresengine.NewEventStoreFromSQLXAndReplica(nil, replica)
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
	wrapper := postgreswrapper.CreateWrapperWithTestConfig(t, postgresengine.WithTableName(customTableName))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	postgreswrapper.CleanUp(t, wrapper)
	bookID := helper.GivenUniqueID(t)
	filter := helper.FilterAllEventTypesForOneBook(bookID)

	err := es.Append(
		ctxWithTimeout,
		filter,
		0,
		helper.ToStorable(t, helper.FixtureBookCopyAddedToCirculation(bookID, fakeClock)),
	)
	assert.NoError(t, err)

	// act
	events, _, queryErr := es.Query(ctxWithTimeout, filter)

	// assert
	assert.NoError(t, queryErr)
	assert.Len(t, events, 1)
}

//nolint:funlen
func Test_FactoryFunctions_FactoryFunctions_ShouldFail_WithEmptyTableName(t *testing.T) {
	testCases := []struct {
		name        string
		factoryFunc func(t *testing.T) (*postgresengine.EventStore, error)
	}{
		{
			name: "NewEventStoreFromPGXPool with empty table name",
			factoryFunc: func(_ *testing.T) (*postgresengine.EventStore, error) {
				connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolSingleConfig())
				if err != nil {
					return nil, fmt.Errorf("test setup failed: %w", err)
				}
				defer connPool.Close()

				return postgresengine.NewEventStoreFromPGXPool(connPool, postgresengine.WithTableName(""))
			},
		},
		{
			name: "NewEventStoreFromSQLDB with empty table name",
			factoryFunc: func(_ *testing.T) (*postgresengine.EventStore, error) {
				db := config.PostgresSQLDBSingleConfig()
				defer func() { _ = db.Close() }()

				return postgresengine.NewEventStoreFromSQLDB(db, postgresengine.WithTableName(""))
			},
		},
		{
			name: "NewEventStoreFromSQLX with empty table name",
			factoryFunc: func(_ *testing.T) (*postgresengine.EventStore, error) {
				db := config.PostgresSQLXSingleConfig()
				defer func() { _ = db.Close() }()

				return postgresengine.NewEventStoreFromSQLX(db, postgresengine.WithTableName(""))
			},
		},
		{
			name: "NewEventStoreFromPGXPoolAndReplica with empty table name",
			factoryFunc: func(_ *testing.T) (*postgresengine.EventStore, error) {
				primary, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolPrimaryConfig())
				if err != nil {
					return nil, fmt.Errorf("test setup failed: %w", err)
				}
				defer primary.Close()

				replica, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolReplicaConfig())
				if err != nil {
					return nil, fmt.Errorf("test setup failed: %w", err)
				}
				defer replica.Close()

				return postgresengine.NewEventStoreFromPGXPoolAndReplica(primary, replica, postgresengine.WithTableName(""))
			},
		},
		{
			name: "NewEventStoreFromSQLDBAndReplica with empty table name",
			factoryFunc: func(_ *testing.T) (*postgresengine.EventStore, error) {
				primary := config.PostgresSQLDBPrimaryConfig()
				defer func() { _ = primary.Close() }()

				replica := config.PostgresSQLDBReplicaConfig()
				defer func() { _ = replica.Close() }()

				return postgresengine.NewEventStoreFromSQLDBAndReplica(primary, replica, postgresengine.WithTableName(""))
			},
		},
		{
			name: "NewEventStoreFromSQLXAndReplica with empty table name",
			factoryFunc: func(_ *testing.T) (*postgresengine.EventStore, error) {
				primary := config.PostgresSQLXPrimaryConfig()
				defer func() { _ = primary.Close() }()

				replica := config.PostgresSQLXReplicaConfig()
				defer func() { _ = replica.Close() }()

				return postgresengine.NewEventStoreFromSQLXAndReplica(primary, replica, postgresengine.WithTableName(""))
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

	wrapper := postgreswrapper.CreateWrapperWithTestConfig(t, postgresengine.WithTableName("non_existent_table_1"))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	bookID := helper.GivenUniqueID(t)
	filter := helper.FilterAllEventTypesForOneBook(bookID)

	// act
	_, _, err := es.Query(ctxWithTimeout, filter)

	// assert
	assert.ErrorContains(t, err, "does not exist")
}

func Test_FactoryFunctions_SingleConnectionMethods_ShouldWorkCorrectly(t *testing.T) {
	testCases := []struct {
		name        string
		factoryFunc func() (*postgresengine.EventStore, error)
	}{
		{
			name: "NewEventStoreFromPGXPool with single connection",
			factoryFunc: func() (*postgresengine.EventStore, error) {
				pool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolSingleConfig())
				if err != nil {
					return nil, fmt.Errorf("test setup failed: %w", err)
				}

				// Note: Close calls will be handled by the EventStore cleanup
				return postgresengine.NewEventStoreFromPGXPool(pool)
			},
		},
		{
			name: "NewEventStoreFromSQLDB with single connection",
			factoryFunc: func() (*postgresengine.EventStore, error) {
				db := config.PostgresSQLDBSingleConfig()

				// Note: Close calls will be handled by the EventStore cleanup
				return postgresengine.NewEventStoreFromSQLDB(db)
			},
		},
		{
			name: "NewEventStoreFromSQLX with single connection",
			factoryFunc: func() (*postgresengine.EventStore, error) {
				db := config.PostgresSQLXSingleConfig()

				// Note: Close calls will be handled by the EventStore cleanup
				return postgresengine.NewEventStoreFromSQLX(db)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// setup
			ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// act
			es, err := tc.factoryFunc()

			// assert
			assert.NoError(t, err)
			assert.NotNil(t, es)

			// Test basic functionality - query should work
			bookID := helper.GivenUniqueID(t)
			filter := helper.FilterAllEventTypesForOneBook(bookID)

			// Query should work (even with no data)
			events, maxSeq, queryErr := es.Query(ctxWithTimeout, filter)
			assert.NoError(t, queryErr)
			assert.Empty(t, events)
			assert.Equal(t, eventstore.MaxSequenceNumberUint(0), maxSeq)
		})
	}
}

func Test_FactoryFunctions_ReplicaFactoryMethods_ShouldWorkCorrectly(t *testing.T) {
	testCases := []struct {
		name        string
		factoryFunc func() (*postgresengine.EventStore, error)
	}{
		{
			name: "NewEventStoreFromPGXPoolAndReplica with valid connections",
			factoryFunc: func() (*postgresengine.EventStore, error) {
				primary, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolPrimaryConfig())
				if err != nil {
					return nil, fmt.Errorf("test setup failed: %w", err)
				}

				replica, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolReplicaConfig())
				if err != nil {
					return nil, fmt.Errorf("test setup failed: %w", err)
				}

				// Note: Close calls will be handled by the EventStore cleanup
				return postgresengine.NewEventStoreFromPGXPoolAndReplica(primary, replica)
			},
		},
		{
			name: "NewEventStoreFromSQLDBAndReplica with valid connections",
			factoryFunc: func() (*postgresengine.EventStore, error) {
				primary := config.PostgresSQLDBPrimaryConfig()
				replica := config.PostgresSQLDBReplicaConfig()

				// Note: Close calls will be handled by the EventStore cleanup
				return postgresengine.NewEventStoreFromSQLDBAndReplica(primary, replica)
			},
		},
		{
			name: "NewEventStoreFromSQLXAndReplica with valid connections",
			factoryFunc: func() (*postgresengine.EventStore, error) {
				primary := config.PostgresSQLXPrimaryConfig()
				replica := config.PostgresSQLXReplicaConfig()

				// Note: Close calls will be handled by the EventStore cleanup
				return postgresengine.NewEventStoreFromSQLXAndReplica(primary, replica)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// setup
			ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// act
			es, err := tc.factoryFunc()

			// assert
			assert.NoError(t, err)
			assert.NotNil(t, es)

			// Test basic functionality - query should work
			bookID := helper.GivenUniqueID(t)
			filter := helper.FilterAllEventTypesForOneBook(bookID)

			// Query should work (even with no data)
			events, maxSeq, queryErr := es.Query(ctxWithTimeout, filter)
			assert.NoError(t, queryErr)
			assert.Empty(t, events)
			assert.Equal(t, eventstore.MaxSequenceNumberUint(0), maxSeq)
		})
	}
}

func Test_FactoryFunctions_ReplicaFactoryMethods_WithNilReplica_ShouldWorkCorrectly(t *testing.T) {
	testCases := []struct {
		name        string
		factoryFunc func() (*postgresengine.EventStore, error)
	}{
		{
			name: "NewEventStoreFromPGXPoolAndReplica with nil replica",
			factoryFunc: func() (*postgresengine.EventStore, error) {
				primary, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolPrimaryConfig())
				if err != nil {
					return nil, fmt.Errorf("test setup failed: %w", err)
				}

				// Pass nil replica - should fallback to primary for reads
				return postgresengine.NewEventStoreFromPGXPoolAndReplica(primary, nil)
			},
		},
		{
			name: "NewEventStoreFromSQLDBAndReplica with nil replica",
			factoryFunc: func() (*postgresengine.EventStore, error) {
				primary := config.PostgresSQLDBPrimaryConfig()

				// Pass nil replica - should fallback to primary for reads
				return postgresengine.NewEventStoreFromSQLDBAndReplica(primary, nil)
			},
		},
		{
			name: "NewEventStoreFromSQLXAndReplica with nil replica",
			factoryFunc: func() (*postgresengine.EventStore, error) {
				primary := config.PostgresSQLXPrimaryConfig()

				// Pass nil replica - should fallback to primary for reads
				return postgresengine.NewEventStoreFromSQLXAndReplica(primary, nil)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// setup
			ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// act
			es, err := tc.factoryFunc()

			// assert
			assert.NoError(t, err)
			assert.NotNil(t, es)

			// Test that it can still query and append (using primary for both)
			bookID := helper.GivenUniqueID(t)
			filter := helper.FilterAllEventTypesForOneBook(bookID)

			// Query should work (fallback to primary)
			events, maxSeq, queryErr := es.Query(ctxWithTimeout, filter)
			assert.NoError(t, queryErr)
			assert.Empty(t, events)
			assert.Equal(t, eventstore.MaxSequenceNumberUint(0), maxSeq)
		})
	}
}
