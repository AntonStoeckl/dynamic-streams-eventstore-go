package core

import (
	"time"

	"github.com/google/uuid"
)

// ReaderContractCanceledEventType is the event type identifier.
const ReaderContractCanceledEventType = "ReaderContractCanceled"

// ReaderContractCanceled represents when a reader's contract is canceled in the library system.
type ReaderContractCanceled struct {
	ReaderID   ReaderIDString
	OccurredAt OccurredAtTS
}

// BuildReaderContractCanceled creates a new ReaderContractCanceled event.
func BuildReaderContractCanceled(
	readerID uuid.UUID,
	occurredAt time.Time,
) ReaderContractCanceled {

	event := ReaderContractCanceled{
		ReaderID:   readerID.String(),
		OccurredAt: ToOccurredAt(occurredAt),
	}

	return event
}

// EventType returns the event type identifier.
func (e ReaderContractCanceled) EventType() string {
	return ReaderContractCanceledEventType
}

// HasOccurredAt returns when this event occurred.
func (e ReaderContractCanceled) HasOccurredAt() time.Time {
	return e.OccurredAt
}

// IsErrorEvent returns false since this event represents a successful operation.
func (e ReaderContractCanceled) IsErrorEvent() bool {
	return false
}
