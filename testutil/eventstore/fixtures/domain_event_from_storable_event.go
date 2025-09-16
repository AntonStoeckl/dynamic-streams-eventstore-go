package fixtures

import (
	"errors"

	jsoniter "github.com/json-iterator/go"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/eventstore/shared"
)

var (
	// ErrMappingToDomainEventFailed is returned when domain event conversion fails.
	ErrMappingToDomainEventFailed = errors.New("mapping to domain event failed")

	// ErrMappingToDomainEventUnknownEventType is returned for unrecognized event types.
	ErrMappingToDomainEventUnknownEventType = errors.New("unknown event type")
)

// DomainEventsFrom converts multiple StorableEvents to DomainEvents.
func DomainEventsFrom(storableEvents eventstore.StorableEvents) (shared.DomainEvents, error) {
	domainEvents := make(shared.DomainEvents, 0, len(storableEvents))

	for _, storableEvent := range storableEvents {
		domainEvent, err := DomainEventFrom(storableEvent)
		if err != nil {
			return nil, err
		}

		domainEvents = append(domainEvents, domainEvent)
	}

	return domainEvents, nil
}

// DomainEventFrom converts a StorableEvent to its corresponding DomainEvent.
func DomainEventFrom(storableEvent eventstore.StorableEvent) (shared.DomainEvent, error) {
	switch storableEvent.EventType {
	case BookCopyAddedToCirculationEventType:
		return unmarshalBookCopyAddedToCirculation(storableEvent.PayloadJSON)

	case BookCopyRemovedFromCirculationEventType:
		return unmarshalBookCopyRemovedFromCirculation(storableEvent.PayloadJSON)

	case BookCopyLentToReaderEventType:
		return unmarshalBookCopyLentToReader(storableEvent.PayloadJSON)

	case BookCopyReturnedByReaderEventType:
		return unmarshalBookCopyReturnedByReader(storableEvent.PayloadJSON)

	default:
		return nil, errors.Join(ErrMappingToDomainEventFailed, ErrMappingToDomainEventUnknownEventType)
	}
}

func unmarshalBookCopyAddedToCirculation(payloadJSON []byte) (shared.DomainEvent, error) {
	var event BookCopyAddedToCirculation

	err := jsoniter.ConfigFastest.Unmarshal(payloadJSON, &event)
	if err != nil {
		return BookCopyAddedToCirculation{}, errors.Join(ErrMappingToDomainEventFailed, err)
	}

	return event, nil
}

func unmarshalBookCopyRemovedFromCirculation(payloadJSON []byte) (shared.DomainEvent, error) {
	var event BookCopyRemovedFromCirculation

	err := jsoniter.ConfigFastest.Unmarshal(payloadJSON, &event)
	if err != nil {
		return BookCopyRemovedFromCirculation{}, errors.Join(ErrMappingToDomainEventFailed, err)
	}

	return event, nil
}

func unmarshalBookCopyLentToReader(payloadJSON []byte) (shared.DomainEvent, error) {
	var event BookCopyLentToReader

	err := jsoniter.ConfigFastest.Unmarshal(payloadJSON, &event)
	if err != nil {
		return BookCopyLentToReader{}, errors.Join(ErrMappingToDomainEventFailed, err)
	}

	return event, nil
}

func unmarshalBookCopyReturnedByReader(payloadJSON []byte) (shared.DomainEvent, error) {
	var event BookCopyReturnedByReader

	err := jsoniter.ConfigFastest.Unmarshal(payloadJSON, &event)
	if err != nil {
		return BookCopyReturnedByReader{}, errors.Join(ErrMappingToDomainEventFailed, err)
	}

	return event, nil
}
