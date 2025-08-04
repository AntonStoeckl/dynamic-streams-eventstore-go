package bookscurrentlylentbyreader

import (
	"time"

	"github.com/google/uuid"

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

// Query represents the intent to query books currently lent by a reader.
type Query struct {
	ReaderID uuid.UUID
}

// BuildQuery creates a new Query with the provided reader ID.
func BuildQuery(readerID uuid.UUID) Query {
	return Query{
		ReaderID: readerID,
	}
}