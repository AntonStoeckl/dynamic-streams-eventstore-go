package bookscurrentlylentbyreader

import (
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// ProjectBooksCurrentlyLent implements the query logic to determine books currently lent to a reader.
// This is a pure function with no side effects - it takes the current domain events and a query
// and returns the projected state showing books currently lent to the specified reader.
//
// Query Logic:
//   GIVEN: A reader with ReaderID
//   WHEN: BooksCurrentlyLentByReader query is executed
//   THEN: BooksCurrentlyLent struct is returned with current lending state
//   INCLUDES: Book information (ID, title, authors, lent date)
//   EXCLUDES: Books that have been returned or removed from circulation
func ProjectBooksCurrentlyLent(history core.DomainEvents, query Query) BooksCurrentlyLent {
	readerID := query.ReaderID.String()
	
	// Track book lending state and book information
	lentBooks := make(map[string]BookInfo)
	bookDetails := make(map[string]struct {
		title   string
		authors string
	})

	for _, event := range history {
		switch e := event.(type) {
		case core.BookCopyAddedToCirculation:
			// Track book details for later use
			bookDetails[e.BookID] = struct {
				title   string
				authors string
			}{
				title:   e.Title,
				authors: e.Authors,
			}

		case core.BookCopyLentToReader:
			if e.ReaderID == readerID {
				// Add book to lent books if we have details
				if details, exists := bookDetails[e.BookID]; exists {
					lentBooks[e.BookID] = BookInfo{
						BookID:  e.BookID,
						Title:   details.title,
						Authors: details.authors,
						LentAt:  e.OccurredAt,
					}
				} else {
					// Add with minimal information if details not available
					lentBooks[e.BookID] = BookInfo{
						BookID:  e.BookID,
						Title:   "Unknown Title",
						Authors: "Unknown Authors",
						LentAt:  e.OccurredAt,
					}
				}
			}

		case core.BookCopyReturnedByReader:
			if e.ReaderID == readerID {
				// Remove book from lent books
				delete(lentBooks, e.BookID)
			}

		case core.BookCopyRemovedFromCirculation:
			// If a book is removed from circulation, it's no longer lent
			delete(lentBooks, e.BookID)
		}
	}

	// Convert map to slice for result
	books := make([]BookInfo, 0, len(lentBooks))
	for _, bookInfo := range lentBooks {
		books = append(books, bookInfo)
	}

	return BooksCurrentlyLent{
		ReaderID: readerID,
		Books:    books,
		Count:    len(books),
	}
}