// Package core contains domain events for the example:
// Book circulation in a public library.
//
// This package implements domain events following proper domain-driven design patterns,
// avoiding CRUD-style antipatterns. Events represent meaningful business occurrences
// like BookCopyAddedToCirculation and BookCopyLentToReader rather than generic
// create/update operations.
//
// All domain events implement the DomainEvent interface with EventType() and
// HasOccurredAt() methods for event sourcing integration.
//
// In Domain-Driven Design or Hexagonal Architecture terminology, this would be
// called the 'domain' layer.
package core
