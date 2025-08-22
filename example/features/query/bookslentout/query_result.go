package bookslentout

import (
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// LendingInfo represents information about a book currently lent to a reader.
// Optimized for performance - contains only essential data (BookID, ReaderID, and lending timestamp).
// Book metadata (title, authors, etc.) should be retrieved separately if needed.
type LendingInfo struct {
	BookID   core.BookIDString
	ReaderID core.ReaderIDString
	LentAt   time.Time
}

// BooksLentOut represents the query result containing all books currently lent out to readers.
type BooksLentOut struct {
	Lendings       []LendingInfo
	Count          int
	SequenceNumber uint
}

// GetSequenceNumber returns the sequence number of the last event in the event history that was used to build the projection.
func (r BooksLentOut) GetSequenceNumber() uint {
	return r.SequenceNumber
}
