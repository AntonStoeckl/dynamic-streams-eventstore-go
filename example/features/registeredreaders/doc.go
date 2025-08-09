// Package registeredreaders implements the Registered Readers query use case.
//
// This feature provides a pure query operation that returns information about all readers
// currently registered (non-canceled) in the library system. It follows the Query-Project pattern
// without any command processing or event generation.
//
// The query returns a RegisteredReaders struct containing the list of reader information
// and the total count of readers currently registered with active contracts.
//
// This is a read-only operation that projects the current state from the event history
// without modifying any data or generating new events.
package registeredreaders
