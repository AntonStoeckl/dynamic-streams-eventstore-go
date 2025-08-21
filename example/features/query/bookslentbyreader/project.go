package bookslentbyreader

import (
	"slices"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// Project implements the query logic to determine books currently lent to a reader.
// This is a pure function with no side effects - it takes the current domain events and a query
// and returns the projected state showing books currently lent to the specified reader.
//
// Query Logic:
//
//	GIVEN: A reader with ReaderID
//	WHEN: BooksLentByReader query is executed
//	THEN: BooksCurrentlyLent struct is returned with current lending state
//	INCLUDES: lending information (BookID, lent date)
//	EXCLUDES: Books that have been returned
func Project(history core.DomainEvents, query Query, maxSequence uint, base ...BooksCurrentlyLent) BooksCurrentlyLent {
	queriedReaderID := query.ReaderID.String()

	lendingInfos := make(map[string]*LendingInfo)

	if len(base) > 0 {
		lendingInfos = convertLendingsToMap(base[0].Books) // Start from an existing projection (incremental update)
	}

	for _, event := range history {
		switch e := event.(type) {
		case core.BookCopyLentToReader:
			if e.ReaderID == queriedReaderID {
				// Add the book to lent books
				lendingInfos[e.BookID] = &LendingInfo{
					BookID: e.BookID,
					LentAt: e.OccurredAt,
				}
			}

		case core.BookCopyReturnedByReader:
			if e.ReaderID == queriedReaderID {
				// Remove the book from lent books
				delete(lendingInfos, e.BookID)
			}
		}
	}

	// Convert map to slice and sort by LentAt (oldest first)
	books := make([]LendingInfo, 0, len(lendingInfos))
	for _, bookPtr := range lendingInfos {
		books = append(books, *bookPtr)
	}
	slices.SortFunc(books, func(a, b LendingInfo) int {
		return a.LentAt.Compare(b.LentAt)
	})

	return BooksCurrentlyLent{
		ReaderID:       queriedReaderID,
		Books:          books,
		Count:          len(lendingInfos),
		SequenceNumber: maxSequence,
	}
}

// BuildEventFilter creates the filter for querying events related to the specified reader.
func BuildEventFilter(readerID uuid.UUID) eventstore.Filter {
	return eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			core.BookCopyLentToReaderEventType,
			core.BookCopyReturnedByReaderEventType,
		).
		AndAnyPredicateOf(
			eventstore.P("ReaderID", readerID.String()),
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
