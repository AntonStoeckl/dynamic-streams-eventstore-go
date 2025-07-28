package core

import (
	"time"

	"github.com/google/uuid"
)

const BookCopyAddedToCirculationEventType = "BookCopyAddedToCirculation"

type BookCopyAddedToCirculation struct {
	BookID          BookIDString
	ISBN            ISBNString
	Title           string
	Authors         string
	Edition         string
	Publisher       string
	PublicationYear uint
	OccurredAt      OccurredAt
}

func BuildBookCopyAddedToCirculation(
	bookID uuid.UUID,
	isbn string,
	title string,
	authors string,
	edition string,
	publisher string,
	publicationYear uint,
	occurredAt time.Time,
) DomainEvent {

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

func (e BookCopyAddedToCirculation) EventType() string {
	return BookCopyAddedToCirculationEventType
}

func (e BookCopyAddedToCirculation) HasOccurredAt() time.Time {
	return e.OccurredAt
}
