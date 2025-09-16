// Package fixtures contains minimal test events for EventStore testing.
//
// This package provides a small set of domain events from a library management
// domain that are used for testing EventStore functionality. These events
// follow proper domain-driven design patterns with meaningful business events
// like BookCopyAddedToCirculation and BookCopyLentToReader.
//
// The events implement the DomainEvent interface and include serialization
// utilities (StorableEventFrom, DomainEventFrom) needed for EventStore testing.
//
// This is testing infrastructure - not production domain code. For a complete
// example implementation, see the separate public-library-example-go repository.
package fixtures
