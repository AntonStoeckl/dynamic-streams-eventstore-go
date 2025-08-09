package bookslentbyreader

import (
	"maps"
	"slices"

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
//	INCLUDES: Book information (ID, title, authors, isbn, lent date)
//	EXCLUDES: Books that have been returned
func ProjectBooksCurrentlyLent(history core.DomainEvents, query Query) BooksCurrentlyLent {
	queriedReaderID := query.ReaderID.String()

	// Track book lending state and book information
	lentBooks := make(map[string]BookInfo)
	bookInfos := make(map[string]BookInfo)

	for _, event := range history {
		switch e := event.(type) {
		case core.BookCopyAddedToCirculation:
			// Track book details for later use
			bookInfos[e.BookID] = BookInfo{
				BookID:          e.BookID,
				Title:           e.Title,
				Authors:         e.Authors,
				ISBN:            e.ISBN,
				Edition:         e.Edition,
				Publisher:       e.Publisher,
				PublicationYear: e.PublicationYear,
			}

		case core.BookCopyLentToReader:
			if e.ReaderID == queriedReaderID {
				// Add the book to bookInfos if we have details - gracefully ignore if we don't have details
				if details, exists := bookInfos[e.BookID]; exists {
					lentBooks[e.BookID] = BookInfo{
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
				// Remove book from lent bookInfos
				delete(lentBooks, e.BookID)
			}
		}
	}

	// Convert map to slice and sort by LentAt (oldest first)
	books := slices.Collect(maps.Values(lentBooks))
	slices.SortFunc(books, func(a, b BookInfo) int {
		return a.LentAt.Compare(b.LentAt)
	})

	return BooksCurrentlyLent{
		ReaderID: queriedReaderID,
		Books:    books,
		Count:    len(lentBooks),
	}
}
