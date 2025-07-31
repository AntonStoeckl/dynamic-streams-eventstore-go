package shell

import (
	"errors"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// ErrEventEnvelopeFromStorableEventFailed is returned when event envelope conversion fails
var ErrEventEnvelopeFromStorableEventFailed = errors.New("event envelope from storable event failed")

// EventEnvelopes is a slice of EventEnvelope instances
type EventEnvelopes = []EventEnvelope

// EventEnvelope combines a domain event with its metadata
type EventEnvelope struct {
	DomainEvent   core.DomainEvent
	EventMetadata EventMetadata
}

// BuildEventEnvelope creates a new EventEnvelope from domain event and metadata
func BuildEventEnvelope(domainEvent core.DomainEvent, eventMetadata EventMetadata) EventEnvelope {
	return EventEnvelope{
		DomainEvent:   domainEvent,
		EventMetadata: eventMetadata,
	}
}

// EventEnvelopeFrom converts a StorableEvent to an EventEnvelope
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

// EventEnvelopesFrom converts multiple StorableEvents to EventEnvelopes
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
