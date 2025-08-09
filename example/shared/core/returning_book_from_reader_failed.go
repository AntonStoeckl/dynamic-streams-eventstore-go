package core

import (
	"time"

	"github.com/google/uuid"
)

// ReturningBookFromReaderFailedEventType is the event type identifier.
const ReturningBookFromReaderFailedEventType = "ReturningBookFromReaderFailed"

// ReturningBookFromReaderFailed represents when returning a book copy from a reader fails due to business rule violations.
type ReturningBookFromReaderFailed struct {
	BookID      BookIDString
	ReaderID    ReaderIDString
	FailureInfo string
	OccurredAt  OccurredAtTS
}

// BuildReturningBookFromReaderFailed creates a new ReturningBookFromReaderFailed event.
func BuildReturningBookFromReaderFailed(
	bookID uuid.UUID,
	readerID uuid.UUID,
	failureInfo string,
	occurredAt time.Time,
) ReturningBookFromReaderFailed {

	event := ReturningBookFromReaderFailed{
		BookID:      bookID.String(),
		ReaderID:    readerID.String(),
		FailureInfo: failureInfo,
		OccurredAt:  ToOccurredAt(occurredAt),
	}

	return event
}

// EventType returns the event type identifier.
func (e ReturningBookFromReaderFailed) EventType() string {
	return ReturningBookFromReaderFailedEventType
}

// HasOccurredAt returns when this event occurred.
func (e ReturningBookFromReaderFailed) HasOccurredAt() time.Time {
	return e.OccurredAt
}

// IsErrorEvent returns true since this event represents a failure condition.
func (e ReturningBookFromReaderFailed) IsErrorEvent() bool {
	return true
}
