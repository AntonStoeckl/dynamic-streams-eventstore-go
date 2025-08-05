package booksincirculation

import (
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
	lentBooks := make(map[string]bool) // Track which books are currently lent

	for _, event := range history {
		switch e := event.(type) {
		case core.BookCopyAddedToCirculation:
			// Add book to circulation
			books[e.BookID] = BookInfo{
				BookID:          e.BookID,
				Title:           e.Title,
				Authors:         e.Authors,
				ISBN:            e.ISBN,
				AddedAt:         e.OccurredAt,
				IsCurrentlyLent: false, // Initially not lent
			}

		case core.BookCopyRemovedFromCirculation:
			// Remove book from circulation
			delete(books, e.BookID)
			delete(lentBooks, e.BookID)

		case core.BookCopyLentToReader:
			// Mark book as lent if it's in circulation
			if bookInfo, exists := books[e.BookID]; exists {
				bookInfo.IsCurrentlyLent = true
				books[e.BookID] = bookInfo
				lentBooks[e.BookID] = true
			}

		case core.BookCopyReturnedByReader:
			// Mark book as not lent if it's in circulation
			if bookInfo, exists := books[e.BookID]; exists {
				bookInfo.IsCurrentlyLent = false
				books[e.BookID] = bookInfo
				delete(lentBooks, e.BookID)
			}
		}
	}

	// Convert map to slice for result
	bookList := make([]BookInfo, 0, len(books))
	for _, bookInfo := range books {
		bookList = append(bookList, bookInfo)
	}

	return BooksInCirculation{
		Books: bookList,
		Count: len(bookList),
	}
}
