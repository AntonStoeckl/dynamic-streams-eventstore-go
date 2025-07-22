package core

import (
	"time"

	"github.com/google/uuid"
)

const BookCopyReturnedByReaderEventType = "BookCopyReturnedByReader"

type BookCopyReturnedByReader struct {
	BookID     BookIDString
	ReaderID   ReaderIDString
	OccurredAt OccurredAt
}

func BuildBookCopyReturnedFromReader(bookID uuid.UUID, readerID uuid.UUID, occurredAt time.Time) DomainEvent {
	event := BookCopyReturnedByReader{
		BookID:     bookID.String(),
		ReaderID:   readerID.String(),
		OccurredAt: ToOccurredAt(occurredAt),
	}

	return event
}

func (e BookCopyReturnedByReader) EventType() string {
	return BookCopyReturnedByReaderEventType
}

func (e BookCopyReturnedByReader) HasOccurredAt() time.Time {
	return e.OccurredAt
}
