// Package bookslentbyreader implements the Books Lent By Reader query use case.
//
// This feature provides a pure query operation that returns information about books
// currently lent to a specific reader. It follows the Query-Project pattern without
// any command processing or event generation.
//
// The query returns a BooksCurrentlyLent struct containing the reader ID, list of
// book information, and the total count of books currently lent to the reader.
//
// This is a read-only operation that projects the current state from the event history
// without modifying any data or generating new events.
package bookslentbyreader
