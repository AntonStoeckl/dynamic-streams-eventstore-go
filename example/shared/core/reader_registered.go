package core

import (
	"time"

	"github.com/google/uuid"
)

// ReaderRegisteredEventType is the event type identifier.
const ReaderRegisteredEventType = "ReaderRegistered"

// ReaderRegistered represents when a new reader is registered in the library system.
type ReaderRegistered struct {
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
		ReaderID:   readerID.String(),
		Name:       name,
		OccurredAt: ToOccurredAt(occurredAt),
	}

	return event
}

// EventType returns the event type identifier.
func (e ReaderRegistered) EventType() string {
	return ReaderRegisteredEventType
}

// HasOccurredAt returns when this event occurred.
func (e ReaderRegistered) HasOccurredAt() time.Time {
	return e.OccurredAt
}