// Package shared provides EventStore interfaces and common types for testing.
//
// This package defines the QueriesAndAppendsEvents interface that abstracts
// EventStore operations for test helpers, enabling test utilities to work with
// any EventStore implementation (PostgreSQL, future backends) without tight coupling.
//
// Key components:
//   - QueriesAndAppendsEvents: Interface for essential EventStore operations
//   - DomainEvent: Common interface for domain events
//   - Event metadata utilities for test setup
//
// This enables EventStore-agnostic test helpers in estesthelpers package.
package shared
