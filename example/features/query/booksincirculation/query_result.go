package booksincirculation

import (
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// BookInfo represents information about a book in circulation.
type BookInfo struct {
	BookID          core.BookIDString
	Title           string
	Authors         string
	ISBN            string
	Edition         string
	Publisher       string
	PublicationYear uint
	AddedAt         time.Time
	IsCurrentlyLent bool
}

// BooksInCirculation represents the query result containing all books in circulation.
type BooksInCirculation struct {
	Books []BookInfo
	Count int
}
