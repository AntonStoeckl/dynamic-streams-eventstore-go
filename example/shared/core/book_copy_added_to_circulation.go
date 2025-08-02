package core

import (
	"time"

	"github.com/google/uuid"
)

// BookCopyAddedToCirculationEventType is the event type identifier.
const BookCopyAddedToCirculationEventType = "BookCopyAddedToCirculation"

// BookCopyAddedToCirculation represents when a book copy is added to library circulation.
type BookCopyAddedToCirculation struct {
	BookID          BookIDString
	ISBN            ISBNString
	Title           string
	Authors         string
	Edition         string
	Publisher       string
	PublicationYear uint
	OccurredAt      OccurredAtTS
}

// BuildBookCopyAddedToCirculation creates a new BookCopyAddedToCirculation event.
func BuildBookCopyAddedToCirculation(
	bookID uuid.UUID,
	isbn string,
	title string,
	authors string,
	edition string,
	publisher string,
	publicationYear uint,
	occurredAt time.Time,
) BookCopyAddedToCirculation {

	event := BookCopyAddedToCirculation{
		BookID:          bookID.String(),
		ISBN:            isbn,
		Title:           title,
		Authors:         authors,
		Edition:         edition,
		Publisher:       publisher,
		PublicationYear: publicationYear,
		OccurredAt:      ToOccurredAt(occurredAt),
	}

	return event
}

// EventType returns the event type identifier.
func (e BookCopyAddedToCirculation) EventType() string {
	return BookCopyAddedToCirculationEventType
}

// HasOccurredAt returns when this event occurred.
func (e BookCopyAddedToCirculation) HasOccurredAt() time.Time {
	return e.OccurredAt
}
