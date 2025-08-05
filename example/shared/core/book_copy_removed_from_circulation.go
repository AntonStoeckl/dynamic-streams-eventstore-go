package core

import (
	"time"

	"github.com/google/uuid"
)

// BookCopyRemovedFromCirculationEventType is the event type identifier.
const BookCopyRemovedFromCirculationEventType = "BookCopyRemovedFromCirculation"

// BookCopyRemovedFromCirculation represents when a book copy is removed from library circulation.
type BookCopyRemovedFromCirculation struct {
	BookID     BookIDString
	OccurredAt OccurredAtTS
}

// BuildBookCopyRemovedFromCirculation creates a new BookCopyRemovedFromCirculation event.
func BuildBookCopyRemovedFromCirculation(bookID uuid.UUID, occurredAt time.Time) BookCopyRemovedFromCirculation {
	event := BookCopyRemovedFromCirculation{
		BookID:     bookID.String(),
		OccurredAt: ToOccurredAt(occurredAt),
	}

	return event
}

// EventType returns the event type identifier.
func (e BookCopyRemovedFromCirculation) EventType() string {
	return BookCopyRemovedFromCirculationEventType
}

// HasOccurredAt returns when this event occurred.
func (e BookCopyRemovedFromCirculation) HasOccurredAt() time.Time {
	return e.OccurredAt
}

// IsErrorEvent returns false since this event represents a successful operation.
func (e BookCopyRemovedFromCirculation) IsErrorEvent() bool {
	return false
}
