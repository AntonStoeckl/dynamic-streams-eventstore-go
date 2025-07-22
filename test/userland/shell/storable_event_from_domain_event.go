package shell

import (
	"encoding/json"
	"errors"

	"dynamic-streams-eventstore/eventstore"
	"dynamic-streams-eventstore/test/userland/core"
)

var ErrMappingToStorableEventFailedForDomainEvent = errors.New("mapping to storable event failed for domain event")
var ErrMappingToStorableEventFailedForMetadata = errors.New("mapping to storable event failed for metadata")

func StorableEventFrom(event core.DomainEvent, metadata EventMetadata) (eventstore.StorableEvent, error) {
	payloadJSON, err := json.Marshal(event)
	if err != nil {
		return eventstore.StorableEvent{}, errors.Join(ErrMappingToStorableEventFailedForDomainEvent, err)
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return eventstore.StorableEvent{}, errors.Join(ErrMappingToStorableEventFailedForMetadata, err)
	}

	storableEvent := eventstore.BuildStorableEvent(
		event.EventType(),
		event.HasOccurredAt(),
		payloadJSON,
		metadataJSON,
	)

	return storableEvent, nil
}

func StorableEventWithEmptyMetadataFrom(event core.DomainEvent) (eventstore.StorableEvent, error) {
	payloadJSON, err := json.Marshal(event)
	if err != nil {
		return eventstore.StorableEvent{}, errors.Join(ErrMappingToStorableEventFailedForDomainEvent, err)
	}

	storableEvent := eventstore.BuildStorableEventWithEmptyMetadata(
		event.EventType(),
		event.HasOccurredAt(),
		payloadJSON,
	)

	return storableEvent, nil
}
