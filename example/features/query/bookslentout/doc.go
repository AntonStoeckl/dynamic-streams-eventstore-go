// Package bookslentout implements the Books Lent Out To Readers query use case.
//
// This feature provides a pure query operation that returns information about all books
// currently lent out to readers in the library system. It follows the Query-Project pattern
// without any command processing or event generation.
//
// The query returns a BooksLentOut struct containing the list of lending information
// with details about which reader has which book, including lending timestamps.
//
// This is a read-only operation that projects the current state from the event history
// without modifying any data or generating new events.
package bookslentout
