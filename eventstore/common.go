package eventstore

import (
	"errors"
)

var (
	// ErrEmptyEventsTableName is returned when an empty table name is provided during event store configuration.
	ErrEmptyEventsTableName = errors.New("events table name must not be empty")

	// ErrConcurrencyConflict is returned when an optimistic concurrency check fails during event appending.
	ErrConcurrencyConflict = errors.New("concurrency error, no rows were affected")

	// ErrCantBuildQueryForZeroEvents is returned when zero events were supplied to build an append query.
	ErrCantBuildQueryForZeroEvents = errors.New("can't build query for zero events")

	// ErrNilDatabaseConnection is returned when a nil database connection is provided to event store constructors.
	ErrNilDatabaseConnection = errors.New("database connection must not be nil")

	// ErrQueryingEventsFailed is returned when the underlying database query operation fails during event retrieval.
	ErrQueryingEventsFailed = errors.New("querying events failed")

	// ErrScanningDBRowFailed is returned when database row scanning fails during event retrieval.
	ErrScanningDBRowFailed = errors.New("scanning db row failed")

	// ErrBuildingStorableEventFailed is returned when construction of a StorableEvent from database data fails.
	ErrBuildingStorableEventFailed = errors.New("building storable event failed")

	// ErrAppendingEventFailed is returned when the underlying database execution fails during event appending.
	ErrAppendingEventFailed = errors.New("appending the event failed")

	// ErrGettingRowsAffectedFailed is returned when retrieving the affected row count after database execution fails.
	ErrGettingRowsAffectedFailed = errors.New("getting rows affected failed")

	// ErrBuildingQueryFailed is returned when SQL query construction fails during event store operations.
	ErrBuildingQueryFailed = errors.New("building the query failed")

	// ErrRowsIterationFailed is returned when iterating over database result rows fails during event retrieval.
	ErrRowsIterationFailed = errors.New("database rows iteration failed")
)

// MaxSequenceNumberUint is a type alias for uint, representing the maximum sequence number for a "dynamic event stream".
type MaxSequenceNumberUint = uint
