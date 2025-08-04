package core

import (
	"time"
)

// RemovingBookFromCirculationFailedEventType is the event type identifier.
const RemovingBookFromCirculationFailedEventType = "RemovingBookFromCirculationFailed"

// RemovingBookFromCirculationFailed represents when removing a book copy from circulation fails due to business rule violations.
type RemovingBookFromCirculationFailed struct {
	EntityID     string
	FailureInfo  string
	OccurredAt   OccurredAtTS
}

// BuildRemovingBookFromCirculationFailed creates a new RemovingBookFromCirculationFailed event.
func BuildRemovingBookFromCirculationFailed(
	entityID string,
	failureInfo string,
	occurredAt time.Time,
) RemovingBookFromCirculationFailed {

	event := RemovingBookFromCirculationFailed{
		EntityID:    entityID,
		FailureInfo: failureInfo,
		OccurredAt:  ToOccurredAt(occurredAt),
	}

	return event
}

// EventType returns the event type identifier.
func (e RemovingBookFromCirculationFailed) EventType() string {
	return RemovingBookFromCirculationFailedEventType
}

// HasOccurredAt returns when this event occurred.
func (e RemovingBookFromCirculationFailed) HasOccurredAt() time.Time {
	return e.OccurredAt
}