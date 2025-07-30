package eventstore

import (
	"errors"
)

var ErrEmptyEventsTableName = errors.New("events table name must not be empty")
var ErrConcurrencyConflict = errors.New("concurrency error, no rows were affected")
var ErrNilDatabaseConnection = errors.New("database connection must not be nil")

// MaxSequenceNumberUint is a type alias for uint, representing the maximum sequence number for a "dynamic event stream".
type MaxSequenceNumberUint = uint
