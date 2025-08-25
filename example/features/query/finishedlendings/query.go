package finishedlendings

import "fmt"

const (
	queryType = "FinishedLendings"
)

// OrderBy defines the sort order for finished lendings results.
type OrderBy uint8

// OrderBy values for sorting finished lendings.
const (
	OrderByOldestReturn OrderBy = iota // Oldest first (ascending by ReturnedAt)
	OrderByNewestReturn                // Newest first (descending by ReturnedAt)
)

// Query represents the input for querying lending cycles that have been completed.
// Supports limiting results and controlling sort order.
type Query struct {
	MaxResults uint32  // Maximum number of results to return (0 = no limit)
	OrderBy    OrderBy // Sort order for results
}

// BuildQuery creates a new Query for retrieving lending cycles that have been completed.
func BuildQuery(maxResults uint32, orderBy ...OrderBy) Query {
	order := OrderByOldestReturn // default

	if len(orderBy) > 0 {
		order = orderBy[0]
	}

	return Query{
		MaxResults: maxResults,
		OrderBy:    order,
	}
}

// QueryType returns the query type.
func (q Query) QueryType() string {
	return queryType
}

// SnapshotType returns the unique snapshot type identifier that includes query parameters.
func SnapshotType(queryType string, q Query) string {
	return fmt.Sprintf("%s:%d:%d", queryType, q.MaxResults, q.OrderBy)
}
