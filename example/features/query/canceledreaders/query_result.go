package canceledreaders

import (
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// ReaderInfo represents information about a canceled reader.
type ReaderInfo struct {
	ReaderID   core.ReaderIDString
	CanceledAt time.Time
}

// CanceledReaders represents the query result containing all canceled readers.
type CanceledReaders struct {
	Readers        []ReaderInfo
	Count          int
	SequenceNumber uint
}

// GetSequenceNumber returns the sequence number of the last event in the event history that was used to build the projection.
func (r CanceledReaders) GetSequenceNumber() uint {
	return r.SequenceNumber
}
