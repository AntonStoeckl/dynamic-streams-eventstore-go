package core

import (
	"time"

	"github.com/google/uuid"
)

const BookCopyRemovedFromCirculationEventType = "BookCopyRemovedFromCirculation"

type BookCopyRemovedFromCirculation struct {
	BookID     BookIDString
	OccurredAt OccurredAt
}

func BuildBookCopyRemovedFromCirculation(bookID uuid.UUID, occurredAt time.Time) DomainEvent {
	event := BookCopyRemovedFromCirculation{
		BookID:     bookID.String(),
		OccurredAt: ToOccurredAt(occurredAt),
	}

	return event
}

func (e BookCopyRemovedFromCirculation) EventType() string {
	return BookCopyRemovedFromCirculationEventType
}

func (e BookCopyRemovedFromCirculation) HasOccurredAt() time.Time {
	return e.OccurredAt
}
