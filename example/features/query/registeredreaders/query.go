package registeredreaders

const (
	queryType = "RegisteredReaders"
)

// Query represents the intent to query books currently lent by a reader.
type Query struct {
}

// BuildQuery creates a new Query (currently empty).
func BuildQuery() Query {
	return Query{}
}

// QueryType returns the query type.
func (q Query) QueryType() string {
	return queryType
}

// SnapshotType returns the unique snapshot type identifier that includes query parameters.
func (q Query) SnapshotType() string {
	return queryType
}
