package finishedlendings

import (
	"slices"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// Project implements the query logic to determine all lending cycles that have been completed.
// This is a pure function with no side effects - it takes the current domain events and optionally
// a base projection to build upon, returning the projected state showing all finished lendings.
//
// Query Logic:
//
//	GIVEN: All book return events in the system (or incremental events since the base projection)
//	WHEN: FinishedLendings query is executed
//	THEN: FinishedLendings struct is returned
//	INCLUDES: Books that have been returned (indicating a completed lending cycle)
//	EXCLUDES: Books that are still lent out or have never been lent
//
// Respects query parameters:
//   - OrderBy: Controls sort order (oldest first vs. newest first)
//   - MaxResults: Limits number of results returned
func Project(history core.DomainEvents, query Query, maxSequence uint, base ...FinishedLendings) FinishedLendings {
	// Track finished lending state
	lendings := make(map[string]*LendingInfo) // Key format: BookID-ReaderID

	totalCount := 0

	if len(base) > 0 {
		lendings = convertLendingsToMap(base[0].Lendings) // Start from an existing projection (incremental update)
		totalCount = base[0].TotalCount
	}

	for _, event := range history {
		if e, ok := event.(core.BookCopyReturnedByReader); ok {
			// Create a unique key for this book-reader combination
			key := e.BookID + "-" + e.ReaderID
			lendings[key] = &LendingInfo{
				BookID:     e.BookID,
				ReaderID:   e.ReaderID,
				ReturnedAt: e.OccurredAt,
			}

			totalCount++
		}
	}

	// Convert map to slice
	lendingList := make([]LendingInfo, 0, len(lendings))
	for _, lendingPtr := range lendings {
		lendingList = append(lendingList, *lendingPtr)
	}

	// Sort based on query OrderBy parameter
	switch query.OrderBy {
	case OrderByNewestReturn:
		slices.SortFunc(lendingList, func(a, b LendingInfo) int {
			return b.ReturnedAt.Compare(a.ReturnedAt) // Descending (newest first)
		})

	default: // OrderByOldestReturn or any other value
		slices.SortFunc(lendingList, func(a, b LendingInfo) int {
			return a.ReturnedAt.Compare(b.ReturnedAt) // Ascending (oldest first)
		})
	}

	// Apply MaxResults limit if specified
	if query.MaxResults > 0 && len(lendingList) > int(query.MaxResults) {
		lendingList = lendingList[:query.MaxResults]
	}

	return FinishedLendings{
		Lendings:       lendingList,
		Count:          len(lendingList),
		TotalCount:     totalCount,
		SequenceNumber: maxSequence,
	}
}

// BuildEventFilter creates the filter for querying all book return events
// which are relevant for this query/use-case.
func BuildEventFilter() eventstore.Filter {
	return eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			core.BookCopyReturnedByReaderEventType,
		).
		Finalize()
}

// convertLendingsToMap converts a slice of LendingInfo to a map keyed by BookID-ReaderID for efficient lookup.
// This is used when starting from a base projection for incremental updates.
func convertLendingsToMap(lendings []LendingInfo) map[string]*LendingInfo {
	lendingInfos := make(map[string]*LendingInfo, len(lendings))
	for i := range lendings {
		lending := lendings[i] // Create a copy to avoid taking the address of the loop variable
		key := lending.BookID + "-" + lending.ReaderID
		lendingInfos[key] = &lending
	}
	return lendingInfos
}
