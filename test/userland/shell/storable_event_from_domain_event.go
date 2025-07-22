package shell

import (
	"encoding/json"
	"errors"

	"dynamic-streams-eventstore/eventstore"
	"dynamic-streams-eventstore/test/userland/core"
)

var ErrMappingStorableEventFromDomainEvent = errors.New("mapping storable event from domain event failed")

func StorableEventFrom(event core.DomainEvent) (eventstore.StorableEvent, error) {
	payloadJSON, err := json.Marshal(event)
	if err != nil {
		return eventstore.StorableEvent{}, errors.Join(ErrMappingStorableEventFromDomainEvent, err)
	}

	esEvent := eventstore.StorableEvent{
		EventType:   event.EventType(),
		OccurredAt:  event.HasOccurredAt(),
		PayloadJSON: payloadJSON,
	}

	return esEvent, nil
}
