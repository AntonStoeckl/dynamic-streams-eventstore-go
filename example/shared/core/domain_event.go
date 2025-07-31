package core

import (
	"time"
)

// DomainEvents is a slice of DomainEvent instances
type DomainEvents = []DomainEvent

// DomainEvent represents a business event that has occurred in the domain
type DomainEvent interface {
	// EventType returns the string identifier for this event type
	EventType() string
	// HasOccurredAt returns when this event occurred
	HasOccurredAt() time.Time
}
