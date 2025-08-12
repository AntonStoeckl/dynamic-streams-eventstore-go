package core

import (
	"time"

	"github.com/google/uuid"
)

// ReaderRegisteredEventType is the event type identifier.
const ReaderRegisteredEventType = "ReaderRegistered"

// ReaderRegistered represents when a new reader is registered in the library system.
type ReaderRegistered struct {
	EventType  EventTypeString
	ReaderID   ReaderIDString
	Name       string
	OccurredAt OccurredAtTS
}

// BuildReaderRegistered creates a new ReaderRegistered event.
func BuildReaderRegistered(
	readerID uuid.UUID,
	name string,
	occurredAt time.Time,
) ReaderRegistered {

	event := ReaderRegistered{
		EventType:  ReaderRegisteredEventType,
		ReaderID:   readerID.String(),
		Name:       name,
		OccurredAt: ToOccurredAt(occurredAt),
	}

	return event
}

// IsEventType returns the event type identifier.
func (e ReaderRegistered) IsEventType() string {
	return ReaderRegisteredEventType
}

// HasOccurredAt returns when this event occurred.
func (e ReaderRegistered) HasOccurredAt() time.Time {
	return e.OccurredAt
}

// IsErrorEvent returns false since this event represents a successful operation.
func (e ReaderRegistered) IsErrorEvent() bool {
	return false
}
