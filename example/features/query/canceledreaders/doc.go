// Package canceledreaders implements the Canceled Readers query use case.
//
// This feature provides a pure query operation that returns information about all readers
// that have canceled their contracts in the library system. It follows the Query-Project pattern
// without any command processing or event generation.
//
// The query returns a CanceledReaders struct containing the list of reader information
// and the total count of canceled readers.
//
// This is a read-only operation that projects the current state from the event history
// without modifying any data or generating new events.
package canceledreaders
