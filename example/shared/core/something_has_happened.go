package core

import (
	"time"
)

// SomethingHasHappenedEventTypePrefix is the prefix for dynamic event types
const SomethingHasHappenedEventTypePrefix = "SomethingHasHappened"

// SomethingHasHappened represents a generic event with dynamic event type
type SomethingHasHappened struct {
	ID               string
	SomeInformation  string
	OccurredAt       OccurredAtTS
	DynamicEventType string
}

// BuildSomethingHasHappened creates a new SomethingHasHappened event with dynamic type
func BuildSomethingHasHappened(
	id string,
	someInformation string,
	occurredAt time.Time,
	dynamicEventType string,
) SomethingHasHappened {

	return SomethingHasHappened{
		ID:               id,
		SomeInformation:  someInformation,
		OccurredAt:       ToOccurredAt(occurredAt),
		DynamicEventType: dynamicEventType,
	}
}

// EventType returns the dynamic event type identifier
func (e SomethingHasHappened) EventType() string {
	return e.DynamicEventType
}

// HasOccurredAt returns when this event occurred
func (e SomethingHasHappened) HasOccurredAt() time.Time {
	return e.OccurredAt
}
