package bookslentout

import (
	"slices"
	"time"

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
	// Track all books with lending state (unified approach like BooksInCirculation)
	lendingInfos := make(map[string]*LendingInfo)

	for _, event := range history {
		switch e := event.(type) {
		case core.BookCopyAddedToCirculation:
			// Add the book (only if not already added)
			if _, exists := lendingInfos[e.BookID]; !exists {
				lendingInfos[e.BookID] = &LendingInfo{
					BookID:          e.BookID,
					Title:           e.Title,
					Authors:         e.Authors,
					ISBN:            e.ISBN,
					Edition:         e.Edition,
					Publisher:       e.Publisher,
					PublicationYear: e.PublicationYear,
					ReaderID:        "", // Initially not lent
					LentAt:          time.Time{},
				}
			}

		case core.BookCopyRemovedFromCirculation:
			// Remove book from circulation
			delete(lendingInfos, e.BookID)

		case core.BookCopyLentToReader:
			// Mark the book as lent and add reader info
			if info := lendingInfos[e.BookID]; info != nil {
				info.ReaderID = e.ReaderID
				info.LentAt = e.OccurredAt
			}

		case core.BookCopyReturnedByReader:
			// Mark the book as not lent and clear reader info
			if info := lendingInfos[e.BookID]; info != nil {
				info.ReaderID = ""
				info.LentAt = time.Time{}
			}
		}
	}

	// Filter to only currently lent books
	lendings := make([]LendingInfo, 0)
	for _, info := range lendingInfos {
		if info.ReaderID != "" { // Book is lent if it has a ReaderID
			lendings = append(lendings, *info)
		}
	}

	// Sort by LentAt (oldest first)
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
			core.BookCopyRemovedFromCirculationEventType,
			core.BookCopyLentToReaderEventType,
			core.BookCopyReturnedByReaderEventType,
		).
		Finalize()
}
