package removedbooks

import (
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// BookInfo represents information about a removed book.
type BookInfo struct {
	BookID    core.BookIDString
	RemovedAt time.Time
}

// RemovedBooks represents the query result containing all books that have been removed from circulation.
type RemovedBooks struct {
	Books          []BookInfo
	Count          int
	SequenceNumber uint
}

// GetSequenceNumber returns the sequence number of the last event in the event history that was used to build the projection.
func (r RemovedBooks) GetSequenceNumber() uint {
	return r.SequenceNumber
}
