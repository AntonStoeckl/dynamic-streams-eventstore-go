// Package booksincirculation implements the Books In Circulation query use case.
//
// This feature provides a pure query operation that returns information about all books
// currently in circulation in the library system. It follows the Query-Project pattern
// without any command processing or event generation.
//
// The query returns a BooksInCirculation struct containing the list of book information
// and the total count of books currently in circulation.
//
// This is a read-only operation that projects the current state from the event history
// without modifying any data or generating new events.
package booksincirculation
