package removedbooks

const (
	queryType = "RemovedBooks"
)

// Query represents the input for querying all books that have been removed from circulation.
// This query uses an empty struct since it doesn't require any input parameters - it returns all removed books.
type Query struct{}

// BuildQuery creates a new Query for retrieving all books that have been removed from circulation.
func BuildQuery() Query {
	return Query{}
}

// QueryType returns the query type.
func (q Query) QueryType() string {
	return queryType
}
