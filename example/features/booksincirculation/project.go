package booksincirculation

import (
	"maps"
	"slices"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// ProjectBooksInCirculation implements the query logic to determine all books currently in circulation.
// This is a pure function with no side effects - it takes the current domain events and a query
// and returns the projected state showing all books currently in circulation.
//
// Query Logic:
//
//	GIVEN: All events in the system
//	WHEN: BooksInCirculation query is executed
//	THEN: BooksInCirculation struct is returned with current circulation state
//	INCLUDES: Books currently in circulation (added but not removed)
//	EXCLUDES: Books that have been removed from circulation
//	DETAILS: Includes lending status for each book
func ProjectBooksInCirculation(history core.DomainEvents) BooksInCirculation {
	// Track book circulation state and book information
	books := make(map[string]BookInfo)

	for _, event := range history {
		switch e := event.(type) {
		case core.BookCopyAddedToCirculation:
			// Add the book to circulation
			books[e.BookID] = BookInfo{
				BookID:          e.BookID,
				Title:           e.Title,
				Authors:         e.Authors,
				ISBN:            e.ISBN,
				Edition:         e.Edition,
				Publisher:       e.Publisher,
				PublicationYear: e.PublicationYear,
				AddedAt:         e.OccurredAt,
				IsCurrentlyLent: false, // Initially not lent
			}

		case core.BookCopyRemovedFromCirculation:
			// Remove book from circulation
			delete(books, e.BookID)

		case core.BookCopyLentToReader:
			// Mark the book as lent if it's in circulation
			if bookInfo, exists := books[e.BookID]; exists {
				bookInfo.IsCurrentlyLent = true
				books[e.BookID] = bookInfo
			}

		case core.BookCopyReturnedByReader:
			// Mark the book as not lent if it's in circulation
			if bookInfo, exists := books[e.BookID]; exists {
				bookInfo.IsCurrentlyLent = false
				books[e.BookID] = bookInfo
			}
		}
	}

	// Convert map to slice and sort by AddedAt (oldest first)
	bookList := slices.Collect(maps.Values(books))
	slices.SortFunc(bookList, func(a, b BookInfo) int {
		return a.AddedAt.Compare(b.AddedAt)
	})

	return BooksInCirculation{
		Books: bookList,
		Count: len(bookList),
	}
}

// BuildEventFilter creates the filter for querying all book circulation events
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
