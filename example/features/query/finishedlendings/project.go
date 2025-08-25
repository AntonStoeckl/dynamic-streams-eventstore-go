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
func Project(history core.DomainEvents, _ Query, maxSequence uint, base ...FinishedLendings) FinishedLendings {
	// Track finished lending state
	lendings := make(map[string]*LendingInfo) // Key format: BookID-ReaderID

	if len(base) > 0 {
		lendings = convertLendingsToMap(base[0].Lendings) // Start from an existing projection (incremental update)
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
		}
	}

	// Convert map to slice and sort by ReturnedAt (oldest first)
	lendingList := make([]LendingInfo, 0, len(lendings))
	for _, lendingPtr := range lendings {
		lendingList = append(lendingList, *lendingPtr)
	}
	slices.SortFunc(lendingList, func(a, b LendingInfo) int {
		return a.ReturnedAt.Compare(b.ReturnedAt)
	})

	return FinishedLendings{
		Lendings:       lendingList,
		Count:          len(lendingList),
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
