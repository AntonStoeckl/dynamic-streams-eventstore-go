// Package shell provides conversion functions between domain events and storable events
// for the example: Book circulation in a public library
//
// This package implements the "imperative shell" pattern, handling the
// translation between the functional core (domain events) and the external
// storage layer (storable events). It manages event serialization, deserialization,
// and metadata handling for the event sourcing system.
//
// In Domain-Driven Design or Hexagonal Architecture terminology, this would be
// called the 'infrastructure' layer.
package shell
