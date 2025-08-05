package bookslentout

import (
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// LendingInfo represents information about a book currently lent to a reader.
type LendingInfo struct {
	BookID    core.BookIDString
	ReaderID  core.ReaderIDString
	Title     string
	Authors   string
	ISBN      string
	LentAt    time.Time
}

// BooksLentOut represents the query result containing all books currently lent out to readers.
type BooksLentOut struct {
	Lendings []LendingInfo
	Count    int
}
