package finishedlendings

const (
	queryType = "FinishedLendings"
)

// Query represents the input for querying all lending cycles that have been completed.
// This query uses an empty struct since it doesn't require any input parameters - it returns all finished lendings.
type Query struct{}

// BuildQuery creates a new Query for retrieving all lending cycles that have been completed.
func BuildQuery() Query {
	return Query{}
}

// QueryType returns the query type.
func (q Query) QueryType() string {
	return queryType
}
