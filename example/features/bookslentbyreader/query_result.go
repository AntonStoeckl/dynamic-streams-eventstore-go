package bookslentbyreader

import (
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// BookInfo represents information about a book currently lent to a reader.
type BookInfo struct {
	BookID    core.BookIDString
	Title     string
	Authors   string
	LentAt    time.Time
}

// BooksCurrentlyLent represents the query result containing books currently lent to a reader.
type BooksCurrentlyLent struct {
	ReaderID core.ReaderIDString
	Books    []BookInfo
	Count    int
}