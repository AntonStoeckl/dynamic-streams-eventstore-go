package bookslentout

import (
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// ProjectBooksLentOut implements the query logic to determine all books currently lent out to readers.
// This is a pure function with no side effects - it takes the current domain events and a query
// and returns the projected state showing all books currently lent out with reader information.
//
// Query Logic:
//   GIVEN: All events in the system
//   WHEN: BooksLentOut query is executed
//   THEN: BooksLentOut struct is returned with current lending state
//   INCLUDES: All books currently lent to readers with lending details
//   EXCLUDES: Books that have been returned or removed from circulation
//   DETAILS: Includes reader ID, book details, and lending timestamp
func ProjectBooksLentOut(history core.DomainEvents) BooksLentOut {
	// Track book lending state and book details
	lentBooks := make(map[string]LendingInfo) // BookID -> LendingInfo
	bookDetails := make(map[string]struct {
		title   string
		authors string
		isbn    string
	})

	for _, event := range history {
		switch e := event.(type) {
		case core.BookCopyAddedToCirculation:
			// Track book details for later use
			bookDetails[e.BookID] = struct {
				title   string
				authors string
				isbn    string
			}{
				title:   e.Title,
				authors: e.Authors,
				isbn:    e.ISBN,
			}

		case core.BookCopyRemovedFromCirculation:
			// If a book is removed from circulation, it's no longer lentable
			delete(lentBooks, e.BookID)
			delete(bookDetails, e.BookID)

		case core.BookCopyLentToReader:
			// Add book to lent books
			if details, exists := bookDetails[e.BookID]; exists {
				lentBooks[e.BookID] = LendingInfo{
					BookID:   e.BookID,
					ReaderID: e.ReaderID,
					Title:    details.title,
					Authors:  details.authors,
					ISBN:     details.isbn,
					LentAt:   e.OccurredAt,
				}
			} else {
				// Add with minimal information if details not available
				lentBooks[e.BookID] = LendingInfo{
					BookID:   e.BookID,
					ReaderID: e.ReaderID,
					Title:    "Unknown Title",
					Authors:  "Unknown Authors",
					ISBN:     "Unknown ISBN",
					LentAt:   e.OccurredAt,
				}
			}

		case core.BookCopyReturnedByReader:
			// Remove book from lent books
			delete(lentBooks, e.BookID)
		}
	}

	// Convert map to slice for result
	lendings := make([]LendingInfo, 0, len(lentBooks))
	for _, lendingInfo := range lentBooks {
		lendings = append(lendings, lendingInfo)
	}

	return BooksLentOut{
		Lendings: lendings,
		Count:    len(lendings),
	}
}