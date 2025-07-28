package eventstore

import (
	"errors"
)

var ErrEmptyTableNameSupplied = errors.New("empty eventTableName supplied")
var ErrConcurrencyConflict = errors.New("concurrency error, no rows were affected")

// MaxSequenceNumberUint is a type alias for uint, representing the maximum sequence number for a "dynamic event stream".
type MaxSequenceNumberUint = uint
