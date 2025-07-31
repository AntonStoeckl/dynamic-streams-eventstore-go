package eventstore

import (
	"encoding/json"
	"errors"
	"time"
)

var ErrInvalidPayloadJSON = errors.New("payload json is not valid")
var ErrInvalidMetadataJSON = errors.New("metadata json is not valid")

// StorableEvents is an alias type for a slice of StorableEvent
type StorableEvents = []StorableEvent

// StorableEvent is a DTO (data transfer object) used by the EventStore to append events and query them back.
//
// It is built on scalars to be completely agnostic of the implementation of Domain Events in the client code.
//
// While its properties are exported, it should only be constructed with the supplied factory methods:
//   - BuildStorableEvent
//   - BuildStorableEventWithEmptyMetadata
type StorableEvent struct {
	EventType    string
	OccurredAt   time.Time
	PayloadJSON  []byte
	MetadataJSON []byte
}

// BuildStorableEvent is a factory method for StorableEvent.
//
// It populates the StorableEvent with the given scalar input.
// Returns an error if payloadJSON or metadataJSON are not valid JSON.
func BuildStorableEvent(eventType string, occurredAt time.Time, payloadJSON []byte, metadataJSON []byte) (StorableEvent, error) {
	if !json.Valid(payloadJSON) {
		return StorableEvent{}, ErrInvalidPayloadJSON
	}

	if !json.Valid(metadataJSON) {
		return StorableEvent{}, ErrInvalidMetadataJSON
	}

	return StorableEvent{
		EventType:    eventType,
		OccurredAt:   occurredAt,
		PayloadJSON:  payloadJSON,
		MetadataJSON: metadataJSON,
	}, nil
}

// BuildStorableEventWithEmptyMetadata is a factory method for StorableEvent.
//
// It populates the StorableEvent with the given scalar input and creates valid empty JSON for MetadataJSON.
// Returns an error if payloadJSON is not valid JSON.
func BuildStorableEventWithEmptyMetadata(eventType string, occurredAt time.Time, payloadJSON []byte) (StorableEvent, error) {
	return BuildStorableEvent(eventType, occurredAt, payloadJSON, []byte("{}"))
}
