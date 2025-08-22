package bookslentbyreader

import (
	"github.com/google/uuid"
)

const (
	queryType = "BooksLentByReader"
)

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

// QueryType returns the query type.
func (q Query) QueryType() string {
	return queryType
}
