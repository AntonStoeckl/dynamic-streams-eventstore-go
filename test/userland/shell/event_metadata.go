package shell

import (
	"errors"

	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

var ErrMappingToEventMetadataFailed = errors.New("mapping to event metadata failed")

type MessageID = string
type CausationID = string
type CorrelationID = string

type EventMetadata struct {
	MessageID     MessageID
	CausationID   CausationID
	CorrelationID CorrelationID
}

func BuildEventMetadata(messageID uuid.UUID, causationID uuid.UUID, correlationID uuid.UUID) EventMetadata {
	return EventMetadata{
		MessageID:     messageID.String(),
		CausationID:   causationID.String(),
		CorrelationID: correlationID.String(),
	}
}

func EventMetadataFrom(storableEvent eventstore.StorableEvent) (EventMetadata, error) {
	metadata := new(EventMetadata)
	err := jsoniter.ConfigFastest.Unmarshal(storableEvent.MetadataJSON, metadata)
	if err != nil {
		return EventMetadata{}, errors.Join(ErrMappingToEventMetadataFailed, err)
	}

	return *metadata, nil
}
