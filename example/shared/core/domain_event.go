package core

import (
	"time"
)

// DomainEvents is a slice of DomainEvent instances.
type DomainEvents = []DomainEvent

// DomainEvent represents a business event that has occurred in the domain.
type DomainEvent interface {
	// HasEventType returns the string identifier for this event type.
	IsEventType() string

	// HasOccurredAt returns when this event occurred.
	HasOccurredAt() time.Time

	// IsErrorEvent returns true if this event represents an error or failure condition.
	IsErrorEvent() bool
}
