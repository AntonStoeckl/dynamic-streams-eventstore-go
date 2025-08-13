package core

import (
	"time"

	"github.com/google/uuid"
)

// CancelingReaderContractFailedEventType is the event type identifier.
const CancelingReaderContractFailedEventType = "CancelingReaderContractFailed"

// CancelingReaderContractFailed represents when a reader's contract can't be canceled due to business rule violations.
type CancelingReaderContractFailed struct {
	EventType   EventTypeString
	ReaderID    ReaderIDString
	FailureInfo string
	OccurredAt  OccurredAtTS
}

// BuildCancelingReaderContractFailed creates a new CancelingReaderContractFailed event.
func BuildCancelingReaderContractFailed(
	readerID uuid.UUID,
	failureInfo string,
	occurredAt time.Time,
) CancelingReaderContractFailed {

	event := CancelingReaderContractFailed{
		EventType:   CancelingReaderContractFailedEventType,
		ReaderID:    readerID.String(),
		FailureInfo: failureInfo,
		OccurredAt:  ToOccurredAt(occurredAt),
	}

	return event
}

// IsEventType returns the event type identifier.
func (e CancelingReaderContractFailed) IsEventType() string {
	return CancelingReaderContractFailedEventType
}

// HasOccurredAt returns when this event occurred.
func (e CancelingReaderContractFailed) HasOccurredAt() time.Time {
	return e.OccurredAt
}

// IsErrorEvent returns false since this event represents a successful operation.
func (e CancelingReaderContractFailed) IsErrorEvent() bool {
	return false
}
