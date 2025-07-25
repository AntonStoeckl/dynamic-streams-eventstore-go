package shell

import (
	"errors"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/test/userland/core"
)

var ErrEventEnvelopeFromStorableEventFailed = errors.New("event envelope from storable event failed")

type EventEnvelopes = []EventEnvelope

type EventEnvelope struct {
	DomainEvent   core.DomainEvent
	EventMetadata EventMetadata
}

func BuildEventEnvelope(domainEvent core.DomainEvent, eventMetadata EventMetadata) EventEnvelope {
	return EventEnvelope{
		DomainEvent:   domainEvent,
		EventMetadata: eventMetadata,
	}
}

func EventEnvelopeFrom(storableEvent eventstore.StorableEvent) (EventEnvelope, error) {
	metadata, err := EventMetadataFrom(storableEvent)
	if err != nil {
		return EventEnvelope{}, errors.Join(ErrEventEnvelopeFromStorableEventFailed, err)
	}

	domainEvent, err := DomainEventFrom(storableEvent)
	if err != nil {
		return EventEnvelope{}, errors.Join(ErrEventEnvelopeFromStorableEventFailed, err)
	}

	return BuildEventEnvelope(domainEvent, metadata), nil
}

func EventEnvelopesFrom(storableEvents eventstore.StorableEvents) (EventEnvelopes, error) {
	envelopes := make(EventEnvelopes, 0)

	for _, storableEvent := range storableEvents {
		envelope, err := EventEnvelopeFrom(storableEvent)
		if err != nil {
			return nil, err
		}

		envelopes = append(envelopes, envelope)
	}

	return envelopes, nil
}
