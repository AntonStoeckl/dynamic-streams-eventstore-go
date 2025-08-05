package core

import (
	"time"

	"github.com/google/uuid"
)

// BookCopyReturnedByReaderEventType is the event type identifier.
const BookCopyReturnedByReaderEventType = "BookCopyReturnedByReader"

// BookCopyReturnedByReader represents when a book copy is returned by a reader.
type BookCopyReturnedByReader struct {
	BookID     BookIDString
	ReaderID   ReaderIDString
	OccurredAt OccurredAtTS
}

// BuildBookCopyReturnedFromReader creates a new BookCopyReturnedByReader event.
func BuildBookCopyReturnedFromReader(bookID uuid.UUID, readerID uuid.UUID, occurredAt time.Time) BookCopyReturnedByReader {
	event := BookCopyReturnedByReader{
		BookID:     bookID.String(),
		ReaderID:   readerID.String(),
		OccurredAt: ToOccurredAt(occurredAt),
	}

	return event
}

// EventType returns the event type identifier.
func (e BookCopyReturnedByReader) EventType() string {
	return BookCopyReturnedByReaderEventType
}

// HasOccurredAt returns when this event occurred.
func (e BookCopyReturnedByReader) HasOccurredAt() time.Time {
	return e.OccurredAt
}

// IsErrorEvent returns false since this event represents a successful operation.
func (e BookCopyReturnedByReader) IsErrorEvent() bool {
	return false
}
