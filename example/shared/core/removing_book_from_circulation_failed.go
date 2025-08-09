package core

import (
	"time"

	"github.com/google/uuid"
)

// RemovingBookFromCirculationFailedEventType is the event type identifier.
const RemovingBookFromCirculationFailedEventType = "RemovingBookFromCirculationFailed"

// RemovingBookFromCirculationFailed represents when removing a book copy from circulation fails due to business rule violations.
type RemovingBookFromCirculationFailed struct {
	BookID      BookIDString
	FailureInfo string
	OccurredAt  OccurredAtTS
}

// BuildRemovingBookFromCirculationFailed creates a new RemovingBookFromCirculationFailed event.
func BuildRemovingBookFromCirculationFailed(
	bookID uuid.UUID,
	failureInfo string,
	occurredAt time.Time,
) RemovingBookFromCirculationFailed {

	event := RemovingBookFromCirculationFailed{
		BookID:      bookID.String(),
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

// IsErrorEvent returns true since this event represents a failure condition.
func (e RemovingBookFromCirculationFailed) IsErrorEvent() bool {
	return true
}
