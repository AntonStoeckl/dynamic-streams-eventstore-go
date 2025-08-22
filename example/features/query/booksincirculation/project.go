package booksincirculation

import (
	"slices"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// Project implements the query logic to determine all books currently in circulation.
// This is a pure function with no side effects - it takes the current domain events and optionally
// a base projection to build upon, returning the projected state showing all books currently in circulation.
//
// Query Logic:
//
//	GIVEN: All events in the system (or incremental events since base projection)
//	WHEN: BooksInCirculation query is executed
//	THEN: BooksInCirculation struct is returned with current circulation state
//	INCLUDES: Books currently in circulation (added but not removed)
//	EXCLUDES: Books that have been removed from circulation
//	DETAILS: Includes lending status for each book
func Project(history core.DomainEvents, _ Query, maxSequence uint, base ...BooksInCirculation) BooksInCirculation {
	// Track book circulation state and book information
	var bookInfos map[string]*BookInfo

	if len(base) > 0 {
		bookInfos = convertBooksToMap(base[0].Books) // Start from an existing projection (incremental update)
	} else {
		bookInfos = make(map[string]*BookInfo) // Start fresh (full projection)
	}

	for _, event := range history {
		switch e := event.(type) {
		case core.BookCopyAddedToCirculation:
			// Add the book (only if not already added)
			if _, exists := bookInfos[e.BookID]; !exists {
				bookInfos[e.BookID] = &BookInfo{
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
			}

		case core.BookCopyRemovedFromCirculation:
			// Remove book from circulation
			delete(bookInfos, e.BookID)

		case core.BookCopyLentToReader:
			// Mark the book as lent if it's in circulation
			if bookInfo := bookInfos[e.BookID]; bookInfo != nil {
				bookInfo.IsCurrentlyLent = true
			}

		case core.BookCopyReturnedByReader:
			// Mark the book as not lent if it's in circulation
			if bookInfo := bookInfos[e.BookID]; bookInfo != nil {
				bookInfo.IsCurrentlyLent = false
			}
		}
	}

	// Convert map to slice and sort by AddedAt (oldest first)
	bookList := make([]BookInfo, 0, len(bookInfos))
	for _, bookPtr := range bookInfos {
		bookList = append(bookList, *bookPtr)
	}
	slices.SortFunc(bookList, func(a, b BookInfo) int {
		return a.AddedAt.Compare(b.AddedAt)
	})

	return BooksInCirculation{
		Books:          bookList,
		Count:          len(bookList),
		SequenceNumber: maxSequence,
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

// convertBooksToMap converts a slice of BookInfo to a map keyed by BookID for efficient lookup.
// This is used when starting from a base projection for incremental updates.
func convertBooksToMap(books []BookInfo) map[string]*BookInfo {
	bookInfos := make(map[string]*BookInfo, len(books))
	for i := range books {
		book := books[i] // Create a copy to avoid taking address of loop variable
		bookInfos[book.BookID] = &book
	}
	return bookInfos
}
