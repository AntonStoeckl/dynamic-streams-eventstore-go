package postgresengine_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
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

func Test_Generic_NewEventStoreWithTableName_Success(t *testing.T) {
	var err error
	assert.NotPanics(t, func() {
		err = TryCreateEventStoreWithTableName(t, "event_data")
	})
	assert.NoError(t, err)
}

func Test_Generic_NewEventStoreWithTableName_ShouldFail_WithEmptyTableName(t *testing.T) {
	var err error
	assert.NotPanics(t, func() {
		err = TryCreateEventStoreWithTableName(t, "")
	})
	assert.ErrorContains(t, err, ErrEmptyEventsTableName.Error())
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
