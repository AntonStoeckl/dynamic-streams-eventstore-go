package core

import (
	"time"
)

const SomethingHasHappenedEventTypePrefix = "SomethingHasHappened"

type SomethingHasHappened struct {
	ID               string
	SomeInformation  string
	OccurredAt       OccurredAt
	DynamicEventType string
}

func BuildSomethingHasHappened(id string, someInformation string, occurredAt time.Time, dynamicEventType string) SomethingHasHappened {
	return SomethingHasHappened{
		ID:               id,
		SomeInformation:  someInformation,
		OccurredAt:       ToOccurredAt(occurredAt),
		DynamicEventType: dynamicEventType,
	}
}

func (e SomethingHasHappened) EventType() string {
	return e.DynamicEventType
}

func (e SomethingHasHappened) HasOccurredAt() time.Time {
	return e.OccurredAt
}
