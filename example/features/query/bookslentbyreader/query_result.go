package bookslentbyreader

import (
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// LendingInfo represents information about a book currently lent to a reader.
// Optimized for performance - contains only essential data (BookID and lending timestamp).
// Book metadata (title, authors, etc.) should be retrieved separately if needed.
type LendingInfo struct {
	BookID core.BookIDString
	LentAt time.Time
}

// BooksCurrentlyLent represents the query result containing books currently lent to a reader.
type BooksCurrentlyLent struct {
	ReaderID       core.ReaderIDString
	Books          []LendingInfo
	Count          int
	SequenceNumber uint
}

// GetSequenceNumber returns the sequence number of the last event in the event history that was used to build the projection.
func (r BooksCurrentlyLent) GetSequenceNumber() uint {
	return r.SequenceNumber
}
