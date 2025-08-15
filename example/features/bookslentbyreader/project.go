package bookslentbyreader

import (
	"slices"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// ProjectBooksCurrentlyLent implements the query logic to determine books currently lent to a reader.
// This is a pure function with no side effects - it takes the current domain events and a query
// and returns the projected state showing books currently lent to the specified reader.
//
// Query Logic:
//
//	GIVEN: A reader with ReaderID
//	WHEN: BooksLentByReader query is executed
//	THEN: BooksCurrentlyLent struct is returned with current lending state
//	INCLUDES: lending information (ID, title, authors, isbn, lent date)
//	EXCLUDES: Books that have been returned
func ProjectBooksCurrentlyLent(history core.DomainEvents, query Query) BooksCurrentlyLent {
	queriedReaderID := query.ReaderID.String()

	// Track book lending state and lending information
	lentBooks := make(map[string]*LendingInfo)
	lendingInfos := make(map[string]*LendingInfo)

	for _, event := range history {
		switch e := event.(type) {
		case core.BookCopyAddedToCirculation:
			// Track lending info for later use (only if not already tracked)
			if _, exists := lendingInfos[e.BookID]; !exists {
				lendingInfos[e.BookID] = &LendingInfo{
					BookID:          e.BookID,
					Title:           e.Title,
					Authors:         e.Authors,
					ISBN:            e.ISBN,
					Edition:         e.Edition,
					Publisher:       e.Publisher,
					PublicationYear: e.PublicationYear,
				}
			}

		case core.BookCopyRemovedFromCirculation:
			// Remove book from lending info and lent books
			delete(lendingInfos, e.BookID)
			delete(lentBooks, e.BookID)

		case core.BookCopyLentToReader:
			if e.ReaderID == queriedReaderID {
				// Add the book to lent books (only if tracked)
				if details := lendingInfos[e.BookID]; details != nil {
					lentBooks[e.BookID] = &LendingInfo{
						BookID:          e.BookID,
						Title:           details.Title,
						Authors:         details.Authors,
						ISBN:            details.ISBN,
						Edition:         details.Edition,
						Publisher:       details.Publisher,
						PublicationYear: details.PublicationYear,
						LentAt:          e.OccurredAt,
					}
				}
			}

		case core.BookCopyReturnedByReader:
			if e.ReaderID == queriedReaderID {
				// Remove the book from lent books (only if it's actually lent)
				delete(lentBooks, e.BookID)
			}
		}
	}

	// Convert map to slice and sort by LentAt (oldest first)
	books := make([]LendingInfo, 0, len(lentBooks))
	for _, bookPtr := range lentBooks {
		books = append(books, *bookPtr)
	}
	slices.SortFunc(books, func(a, b LendingInfo) int {
		return a.LentAt.Compare(b.LentAt)
	})

	return BooksCurrentlyLent{
		ReaderID: queriedReaderID,
		Books:    books,
		Count:    len(lentBooks),
	}
}

// BuildEventFilter creates the filter for querying all events
// related to the specified reader and all book circulation events
// which are relevant for this query/use-case.
func BuildEventFilter(readerID uuid.UUID) eventstore.Filter {
	return eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			core.BookCopyAddedToCirculationEventType,
			core.BookCopyRemovedFromCirculationEventType,
			core.BookCopyLentToReaderEventType,
			core.BookCopyReturnedByReaderEventType,
		).
		AndAnyPredicateOf(
			eventstore.P("ReaderID", readerID.String()),
		).
		Finalize()
}
