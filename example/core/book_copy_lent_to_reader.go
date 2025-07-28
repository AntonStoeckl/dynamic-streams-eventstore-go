package core

import (
	"time"

	"github.com/google/uuid"
)

const BookCopyLentToReaderEventType = "BookCopyLentToReader"

type BookCopyLentToReader struct {
	BookID     BookIDString
	ReaderID   ReaderIDString
	OccurredAt OccurredAt
}

func BuildBookCopyLentToReader(bookID uuid.UUID, readerID uuid.UUID, occurredAt time.Time) DomainEvent {
	event := BookCopyLentToReader{
		BookID:     bookID.String(),
		ReaderID:   readerID.String(),
		OccurredAt: ToOccurredAt(occurredAt),
	}

	return event
}

func (e BookCopyLentToReader) EventType() string {
	return BookCopyLentToReaderEventType
}

func (e BookCopyLentToReader) HasOccurredAt() time.Time {
	return e.OccurredAt
}
