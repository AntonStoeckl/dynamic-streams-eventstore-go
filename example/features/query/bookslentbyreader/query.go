package bookslentbyreader

import (
	"fmt"

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

// SnapshotType returns the unique snapshot type identifier that includes query parameters.
func (q Query) SnapshotType() string {
	return fmt.Sprintf("%s:%s", queryType, q.ReaderID.String())
}
