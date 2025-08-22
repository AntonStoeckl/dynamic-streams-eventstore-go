package bookslentout

import (
	"slices"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// Project implements the query logic to determine all books currently lent out to readers.
// This is a pure function with no side effects - it takes the current domain events and a query
// and returns the projected state showing all books currently lent out with reader information.
//
// Query Logic:
//
//	GIVEN: All lending/returning events in the system
//	WHEN: BooksLentOut query is executed
//	THEN: BooksLentOut struct is returned with current lending state
//	INCLUDES: All books currently lent to readers (BookID, ReaderID, LentAt)
//	EXCLUDES: Books that have been returned
func Project(history core.DomainEvents, _ Query, maxSequence uint, base ...BooksLentOut) BooksLentOut {
	// Track lending state
	var lendingInfos map[string]*LendingInfo

	if len(base) > 0 {
		lendingInfos = convertLendingsToMap(base[0].Lendings) // Start from an existing projection (incremental update)
	} else {
		lendingInfos = make(map[string]*LendingInfo) // Start fresh (full projection)
	}

	for _, event := range history {
		switch e := event.(type) {
		case core.BookCopyLentToReader:
			// Mark the book as lent
			lendingInfos[e.BookID] = &LendingInfo{
				BookID:   e.BookID,
				ReaderID: e.ReaderID,
				LentAt:   e.OccurredAt,
			}

		case core.BookCopyReturnedByReader:
			// Remove the book from lent books
			delete(lendingInfos, e.BookID)
		}
	}

	// Convert to slice (all books in the map are currently lent)
	lendings := make([]LendingInfo, 0, len(lendingInfos))
	for _, info := range lendingInfos {
		lendings = append(lendings, *info)
	}

	// Sort by LentAt (oldest first)
	slices.SortFunc(lendings, func(a, b LendingInfo) int {
		return a.LentAt.Compare(b.LentAt)
	})

	return BooksLentOut{
		Lendings:       lendings,
		Count:          len(lendings),
		SequenceNumber: maxSequence,
	}
}

// BuildEventFilter creates the filter for querying lending and returning events.
func BuildEventFilter() eventstore.Filter {
	return eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			core.BookCopyLentToReaderEventType,
			core.BookCopyReturnedByReaderEventType,
		).
		Finalize()
}

// convertLendingsToMap converts a slice of LendingInfo to a map keyed by BookID for efficient lookup.
// This is used when starting from a base projection for incremental updates.
func convertLendingsToMap(lendings []LendingInfo) map[string]*LendingInfo {
	lendingInfos := make(map[string]*LendingInfo, len(lendings))
	for i := range lendings {
		lending := lendings[i] // Create a copy to avoid taking address of loop variable
		lendingInfos[lending.BookID] = &lending
	}

	return lendingInfos
}
