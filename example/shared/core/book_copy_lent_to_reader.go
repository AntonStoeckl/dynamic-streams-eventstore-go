package core

import (
	"time"

	"github.com/google/uuid"
)

// BookCopyLentToReaderEventType is the event type identifier.
const BookCopyLentToReaderEventType = "BookCopyLentToReader"

// BookCopyLentToReader represents when a book copy is lent to a reader.
type BookCopyLentToReader struct {
	BookID     BookIDString
	ReaderID   ReaderIDString
	OccurredAt OccurredAtTS
}

// BuildBookCopyLentToReader creates a new BookCopyLentToReader event.
func BuildBookCopyLentToReader(bookID uuid.UUID, readerID uuid.UUID, occurredAt time.Time) DomainEvent {
	event := BookCopyLentToReader{
		BookID:     bookID.String(),
		ReaderID:   readerID.String(),
		OccurredAt: ToOccurredAt(occurredAt),
	}

	return event
}

// EventType returns the event type identifier.
func (e BookCopyLentToReader) EventType() string {
	return BookCopyLentToReaderEventType
}

// HasOccurredAt returns when this event occurred.
func (e BookCopyLentToReader) HasOccurredAt() time.Time {
	return e.OccurredAt
}
