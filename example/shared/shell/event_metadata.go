package shell

import (
	"errors"

	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

// ErrMappingToEventMetadataFailed is returned when metadata conversion fails.
var ErrMappingToEventMetadataFailed = errors.New("mapping to event metadata failed")

// MessageID represents a unique message identifier.
type MessageID = string

// CausationID represents the ID of the event that caused this event.
type CausationID = string

// CorrelationID represents the ID correlating related events.
type CorrelationID = string

// EventMetadata contains event tracking information.
type EventMetadata struct {
	MessageID     MessageID
	CausationID   CausationID
	CorrelationID CorrelationID
}

// BuildEventMetadata creates EventMetadata from UUID values.
func BuildEventMetadata(messageID uuid.UUID, causationID uuid.UUID, correlationID uuid.UUID) EventMetadata {
	return EventMetadata{
		MessageID:     messageID.String(),
		CausationID:   causationID.String(),
		CorrelationID: correlationID.String(),
	}
}

// EventMetadataFrom extracts EventMetadata from a StorableEvent.
func EventMetadataFrom(storableEvent eventstore.StorableEvent) (EventMetadata, error) {
	metadata := new(EventMetadata)
	err := jsoniter.ConfigFastest.Unmarshal(storableEvent.MetadataJSON, metadata)
	if err != nil {
		return EventMetadata{}, errors.Join(ErrMappingToEventMetadataFailed, err)
	}

	return *metadata, nil
}
