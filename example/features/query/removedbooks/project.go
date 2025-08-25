package removedbooks

import (
	"slices"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// Project implements the query logic to determine all books that have been removed from circulation.
// This is a pure function with no side effects - it takes the current domain events and optionally
// a base projection to build upon, returning the projected state showing all removed books.
//
// Query Logic:
//
//	GIVEN: All book removal events in the system (or incremental events since the base projection)
//	WHEN: RemovedBooks query is executed
//	THEN: RemovedBooks struct is returned
//	INCLUDES: Books that have been removed from circulation
//	EXCLUDES: Books that are still in circulation or have never been added
func Project(history core.DomainEvents, _ Query, maxSequence uint, base ...RemovedBooks) RemovedBooks {
	// Track removed book state
	books := make(map[string]*BookInfo) // Start fresh (full projection)

	if len(base) > 0 {
		books = convertBooksToMap(base[0].Books) // Start from an existing projection (incremental update)
	}

	for _, event := range history {
		if e, ok := event.(core.BookCopyRemovedFromCirculation); ok {
			books[e.BookID] = &BookInfo{
				BookID:    e.BookID,
				RemovedAt: e.OccurredAt,
			}
		}
	}

	// Convert map to slice and sort by RemovedAt (oldest first)
	bookList := make([]BookInfo, 0, len(books))
	for _, bookPtr := range books {
		bookList = append(bookList, *bookPtr)
	}
	slices.SortFunc(bookList, func(a, b BookInfo) int {
		return a.RemovedAt.Compare(b.RemovedAt)
	})

	return RemovedBooks{
		Books:          bookList,
		Count:          len(bookList),
		SequenceNumber: maxSequence,
	}
}

// BuildEventFilter creates the filter for querying all book removal events
// which are relevant for this query/use-case.
func BuildEventFilter() eventstore.Filter {
	return eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			core.BookCopyRemovedFromCirculationEventType,
		).
		Finalize()
}

// convertBooksToMap converts a slice of BookInfo to a map keyed by BookID for efficient lookup.
// This is used when starting from a base projection for incremental updates.
func convertBooksToMap(books []BookInfo) map[string]*BookInfo {
	bookInfos := make(map[string]*BookInfo, len(books))
	for i := range books {
		book := books[i] // Create a copy to avoid taking the address of the loop variable
		bookInfos[book.BookID] = &book
	}
	return bookInfos
}
