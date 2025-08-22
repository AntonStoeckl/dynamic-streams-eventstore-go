package registeredreaders

import (
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// ReaderInfo represents information about a registered reader.
type ReaderInfo struct {
	ReaderID     core.ReaderIDString
	Name         string
	RegisteredAt time.Time
}

// RegisteredReaders represents the query result containing all registered (non-canceled) readers.
type RegisteredReaders struct {
	Readers        []ReaderInfo
	Count          int
	SequenceNumber uint
}

// GetSequenceNumber returns the sequence number of the last event in the event history that was used to build the projection.
func (r RegisteredReaders) GetSequenceNumber() uint {
	return r.SequenceNumber
}
