package finishedlendings

import (
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// LendingInfo represents information about a finished lending cycle.
// Contains the book that was lent, the reader who borrowed it, when it was lent, and when it was returned.
type LendingInfo struct {
	BookID     core.BookIDString
	ReaderID   core.ReaderIDString
	ReturnedAt time.Time
}

// FinishedLendings represents the query result containing all lending cycles that have been completed.
type FinishedLendings struct {
	Lendings       []LendingInfo
	Count          int // Number of items in Lendings slice (after MaxResults limit applied)
	TotalCount     int // Total number of finished lendings available (before MaxResults limit)
	SequenceNumber uint
}

// GetSequenceNumber returns the sequence number of the last event in the event history that was used to build the projection.
func (r FinishedLendings) GetSequenceNumber() uint {
	return r.SequenceNumber
}
