package bookslentout

import (
	"maps"
	"slices"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// ProjectBooksLentOut implements the query logic to determine all books currently lent out to readers.
// This is a pure function with no side effects - it takes the current domain events and a query
// and returns the projected state showing all books currently lent out with reader information.
//
// Query Logic:
//
//	GIVEN: All events in the system
//	WHEN: BooksLentOut query is executed
//	THEN: BooksLentOut struct is returned with current lending state
//	INCLUDES: All books currently lent to readers with lending details
//	EXCLUDES: Books that have been returned
//	DETAILS: Includes reader ID, book details, and lending timestamp
func ProjectBooksLentOut(history core.DomainEvents) BooksLentOut {
	// Track book lending state and book details
	lentBooks := make(map[string]LendingInfo) // BookID -> LendingInfo
	lendingInfos := make(map[string]LendingInfo)

	for _, event := range history {
		switch e := event.(type) {
		case core.BookCopyAddedToCirculation:
			// Track book details for later use
			lendingInfos[e.BookID] = LendingInfo{
				BookID:          e.BookID,
				Title:           e.Title,
				Authors:         e.Authors,
				ISBN:            e.ISBN,
				Edition:         e.Edition,
				Publisher:       e.Publisher,
				PublicationYear: e.PublicationYear,
			}

		case core.BookCopyLentToReader:
			// Add the book to lent books
			if details, exists := lendingInfos[e.BookID]; exists {
				lentBooks[e.BookID] = LendingInfo{
					BookID:          e.BookID,
					ReaderID:        e.ReaderID,
					Title:           details.Title,
					Authors:         details.Authors,
					ISBN:            details.ISBN,
					Edition:         details.Edition,
					Publisher:       details.Publisher,
					PublicationYear: details.PublicationYear,
					LentAt:          e.OccurredAt,
				}
			}

		case core.BookCopyReturnedByReader:
			// Remove book from lent books
			delete(lentBooks, e.BookID)
		}
	}

	// Convert map to slice and sort by LentAt (oldest first)
	lendings := slices.Collect(maps.Values(lentBooks))
	slices.SortFunc(lendings, func(a, b LendingInfo) int {
		return a.LentAt.Compare(b.LentAt)
	})

	return BooksLentOut{
		Lendings: lendings,
		Count:    len(lendings),
	}
}

// BuildEventFilter creates the filter for querying all book circulation and lending events
// which are relevant for this query/use-case.
func BuildEventFilter() eventstore.Filter {
	return eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			core.BookCopyAddedToCirculationEventType,
			core.BookCopyLentToReaderEventType,
			core.BookCopyReturnedByReaderEventType,
		).
		Finalize()
}
