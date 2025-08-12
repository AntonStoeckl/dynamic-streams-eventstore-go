package core

import (
	"time"

	"github.com/google/uuid"
)

// BookCopyLentToReaderEventType is the event type identifier.
const BookCopyLentToReaderEventType = "BookCopyLentToReader"

// BookCopyLentToReader represents when a book copy is lent to a reader.
type BookCopyLentToReader struct {
	EventType  EventTypeString
	BookID     BookIDString
	ReaderID   ReaderIDString
	OccurredAt OccurredAtTS
}

// BuildBookCopyLentToReader creates a new BookCopyLentToReader event.
func BuildBookCopyLentToReader(bookID uuid.UUID, readerID uuid.UUID, occurredAt time.Time) BookCopyLentToReader {
	event := BookCopyLentToReader{
		EventType:  BookCopyLentToReaderEventType,
		BookID:     bookID.String(),
		ReaderID:   readerID.String(),
		OccurredAt: ToOccurredAt(occurredAt),
	}

	return event
}

// IsEventType returns the event type identifier.
func (e BookCopyLentToReader) IsEventType() string {
	return BookCopyLentToReaderEventType
}

// HasOccurredAt returns when this event occurred.
func (e BookCopyLentToReader) HasOccurredAt() time.Time {
	return e.OccurredAt
}

// IsErrorEvent returns false since this event represents a successful operation.
func (e BookCopyLentToReader) IsErrorEvent() bool {
	return false
}
