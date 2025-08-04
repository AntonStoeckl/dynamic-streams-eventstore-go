package core

import (
	"time"
)

// LendingBookToReaderFailedEventType is the event type identifier.
const LendingBookToReaderFailedEventType = "LendingBookToReaderFailed"

// LendingBookToReaderFailed represents when lending a book copy to a reader fails due to business rule violations.
type LendingBookToReaderFailed struct {
	EntityID     string
	FailureInfo  string
	OccurredAt   OccurredAtTS
}

// BuildLendingBookToReaderFailed creates a new LendingBookToReaderFailed event.
func BuildLendingBookToReaderFailed(
	entityID string,
	failureInfo string,
	occurredAt time.Time,
) LendingBookToReaderFailed {

	event := LendingBookToReaderFailed{
		EntityID:    entityID,
		FailureInfo: failureInfo,
		OccurredAt:  ToOccurredAt(occurredAt),
	}

	return event
}

// EventType returns the event type identifier.
func (e LendingBookToReaderFailed) EventType() string {
	return LendingBookToReaderFailedEventType
}

// HasOccurredAt returns when this event occurred.
func (e LendingBookToReaderFailed) HasOccurredAt() time.Time {
	return e.OccurredAt
}