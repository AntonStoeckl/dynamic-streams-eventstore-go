package bookslentbyreader

import (
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// LendingInfo represents information about a book currently lent to a reader.
type LendingInfo struct {
	BookID          core.BookIDString
	Title           string
	Authors         string
	ISBN            string
	Edition         string
	Publisher       string
	PublicationYear uint
	LentAt          time.Time
}

// BooksCurrentlyLent represents the query result containing books currently lent to a reader.
type BooksCurrentlyLent struct {
	ReaderID core.ReaderIDString
	Books    []LendingInfo
	Count    int
}
