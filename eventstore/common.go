package eventstore

import (
	"errors"
)

var (
	ErrEmptyEventsTableName        = errors.New("events table name must not be empty")
	ErrConcurrencyConflict         = errors.New("concurrency error, no rows were affected")
	ErrNilDatabaseConnection       = errors.New("database connection must not be nil")
	ErrQueryingEventsFailed        = errors.New("querying events failed")
	ErrScanningDBRowFailed         = errors.New("scanning db row failed")
	ErrBuildingStorableEventFailed = errors.New("building storable event failed")
	ErrAppendingEventFailed        = errors.New("appending the event failed")
	ErrGettingRowsAffectedFailed   = errors.New("getting rows affected failed")
	ErrBuildingQueryFailed         = errors.New("building the query failed")
)

// MaxSequenceNumberUint is a type alias for uint, representing the maximum sequence number for a "dynamic event stream".
type MaxSequenceNumberUint = uint
