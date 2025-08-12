package core

import (
	"time"

	"github.com/google/uuid"
)

// LendingBookToReaderFailedEventType is the event type identifier.
const LendingBookToReaderFailedEventType = "LendingBookToReaderFailed"

// LendingBookToReaderFailed represents when lending a book copy to a reader fails due to business rule violations.
type LendingBookToReaderFailed struct {
	EventType   EventTypeString
	BookID      BookIDString
	ReaderID    ReaderIDString
	FailureInfo string
	OccurredAt  OccurredAtTS
}

// BuildLendingBookToReaderFailed creates a new LendingBookToReaderFailed event.
func BuildLendingBookToReaderFailed(
	bookID uuid.UUID,
	readerID uuid.UUID,
	failureInfo string,
	occurredAt time.Time,
) LendingBookToReaderFailed {

	event := LendingBookToReaderFailed{
		EventType:   LendingBookToReaderFailedEventType,
		BookID:      bookID.String(),
		ReaderID:    readerID.String(),
		FailureInfo: failureInfo,
		OccurredAt:  ToOccurredAt(occurredAt),
	}

	return event
}

// IsEventType returns the event type identifier.
func (e LendingBookToReaderFailed) IsEventType() string {
	return LendingBookToReaderFailedEventType
}

// HasOccurredAt returns when this event occurred.
func (e LendingBookToReaderFailed) HasOccurredAt() time.Time {
	return e.OccurredAt
}

// IsErrorEvent returns true since this event represents a failure condition.
func (e LendingBookToReaderFailed) IsErrorEvent() bool {
	return true
}
