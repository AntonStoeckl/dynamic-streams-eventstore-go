package shell

import (
	"encoding/json"
	"errors"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// ErrMappingToStorableEventFailedForDomainEvent is returned when domain event serialization fails
var ErrMappingToStorableEventFailedForDomainEvent = errors.New("mapping to storable event failed for domain event")

// ErrMappingToStorableEventFailedForMetadata is returned when metadata serialization fails
var ErrMappingToStorableEventFailedForMetadata = errors.New("mapping to storable event failed for metadata")

// StorableEventFrom converts a DomainEvent and EventMetadata to a StorableEvent
func StorableEventFrom(event core.DomainEvent, metadata EventMetadata) (eventstore.StorableEvent, error) {
	payloadJSON, err := json.Marshal(event)
	if err != nil {
		return eventstore.StorableEvent{}, errors.Join(ErrMappingToStorableEventFailedForDomainEvent, err)
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return eventstore.StorableEvent{}, errors.Join(ErrMappingToStorableEventFailedForMetadata, err)
	}

	storableEvent, err := eventstore.BuildStorableEvent(
		event.EventType(),
		event.HasOccurredAt(),
		payloadJSON,
		metadataJSON,
	)

	if err != nil {
		return eventstore.StorableEvent{}, errors.Join(ErrMappingToStorableEventFailedForDomainEvent, err)
	}

	return storableEvent, nil
}

// StorableEventWithEmptyMetadataFrom converts a DomainEvent to a StorableEvent with empty metadata
func StorableEventWithEmptyMetadataFrom(event core.DomainEvent) (eventstore.StorableEvent, error) {
	payloadJSON, err := json.Marshal(event)
	if err != nil {
		return eventstore.StorableEvent{}, errors.Join(ErrMappingToStorableEventFailedForDomainEvent, err)
	}

	storableEvent, err := eventstore.BuildStorableEventWithEmptyMetadata(
		event.EventType(),
		event.HasOccurredAt(),
		payloadJSON,
	)

	if err != nil {
		return eventstore.StorableEvent{}, errors.Join(ErrMappingToStorableEventFailedForDomainEvent, err)
	}

	return storableEvent, nil
}
