package shell

import (
	"errors"

	jsoniter "github.com/json-iterator/go"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

var (
	// ErrMappingToDomainEventFailed is returned when domain event conversion fails.
	ErrMappingToDomainEventFailed = errors.New("mapping to domain event failed")

	// ErrMappingToDomainEventUnknownEventType is returned for unrecognized event types.
	ErrMappingToDomainEventUnknownEventType = errors.New("unknown event type")
)

// DomainEventsFrom converts multiple StorableEvents to DomainEvents.
func DomainEventsFrom(storableEvents eventstore.StorableEvents) (core.DomainEvents, error) {
	domainEvents := make(core.DomainEvents, 0, len(storableEvents))

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
func DomainEventFrom(storableEvent eventstore.StorableEvent) (core.DomainEvent, error) {
	switch storableEvent.EventType {
	case core.BookCopyAddedToCirculationEventType:
		return unmarshalBookCopyAddedToCirculation(storableEvent.PayloadJSON)

	case core.BookCopyRemovedFromCirculationEventType:
		return unmarshalBookCopyRemovedFromCirculation(storableEvent.PayloadJSON)

	case core.BookCopyLentToReaderEventType:
		return unmarshalBookCopyLentToReader(storableEvent.PayloadJSON)

	case core.BookCopyReturnedByReaderEventType:
		return unmarshalBookCopyReturnedByReader(storableEvent.PayloadJSON)

	default:
		return nil, errors.Join(ErrMappingToDomainEventFailed, ErrMappingToDomainEventUnknownEventType)
	}
}

func unmarshalBookCopyAddedToCirculation(payloadJSON []byte) (core.DomainEvent, error) {
	var event core.BookCopyAddedToCirculation

	err := jsoniter.ConfigFastest.Unmarshal(payloadJSON, &event)
	if err != nil {
		return core.BookCopyAddedToCirculation{}, errors.Join(ErrMappingToDomainEventFailed, err)
	}

	return event, nil
}

func unmarshalBookCopyRemovedFromCirculation(payloadJSON []byte) (core.DomainEvent, error) {
	var event core.BookCopyRemovedFromCirculation

	err := jsoniter.ConfigFastest.Unmarshal(payloadJSON, &event)
	if err != nil {
		return core.BookCopyRemovedFromCirculation{}, errors.Join(ErrMappingToDomainEventFailed, err)
	}

	return event, nil
}

func unmarshalBookCopyLentToReader(payloadJSON []byte) (core.DomainEvent, error) {
	var event core.BookCopyLentToReader

	err := jsoniter.ConfigFastest.Unmarshal(payloadJSON, &event)
	if err != nil {
		return core.BookCopyLentToReader{}, errors.Join(ErrMappingToDomainEventFailed, err)
	}

	return event, nil
}

func unmarshalBookCopyReturnedByReader(payloadJSON []byte) (core.DomainEvent, error) {
	var event core.BookCopyReturnedByReader

	err := jsoniter.ConfigFastest.Unmarshal(payloadJSON, &event)
	if err != nil {
		return core.BookCopyReturnedByReader{}, errors.Join(ErrMappingToDomainEventFailed, err)
	}

	return event, nil
}
