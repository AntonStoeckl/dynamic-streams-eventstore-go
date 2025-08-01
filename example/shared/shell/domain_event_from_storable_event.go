package shell

import (
	"errors"
	"strings"

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
	domainEvents := make(core.DomainEvents, 0)

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
		if strings.Contains(storableEvent.EventType, core.SomethingHasHappenedEventTypePrefix) {
			return unmarshalSomethingHasHappened(storableEvent.PayloadJSON)
		}
	}

	return nil, errors.Join(ErrMappingToDomainEventFailed, ErrMappingToDomainEventUnknownEventType)
}

func unmarshalBookCopyAddedToCirculation(payloadJSON []byte) (core.DomainEvent, error) {
	payload := new(core.BookCopyAddedToCirculation)

	err := jsoniter.ConfigFastest.Unmarshal(payloadJSON, &payload)
	if err != nil {
		return core.BookCopyAddedToCirculation{}, errors.Join(ErrMappingToDomainEventFailed, err)
	}

	return core.BookCopyAddedToCirculation{
		BookID:          payload.BookID,
		ISBN:            payload.ISBN,
		Title:           payload.Title,
		Authors:         payload.Authors,
		Edition:         payload.Edition,
		Publisher:       payload.Publisher,
		PublicationYear: payload.PublicationYear,
		OccurredAt:      payload.OccurredAt,
	}, nil
}

func unmarshalBookCopyRemovedFromCirculation(payloadJSON []byte) (core.DomainEvent, error) {
	payload := new(core.BookCopyRemovedFromCirculation)

	err := jsoniter.ConfigFastest.Unmarshal(payloadJSON, &payload)
	if err != nil {
		return core.BookCopyRemovedFromCirculation{}, errors.Join(ErrMappingToDomainEventFailed, err)
	}

	return core.BookCopyRemovedFromCirculation{
		BookID:     payload.BookID,
		OccurredAt: payload.OccurredAt,
	}, nil
}

func unmarshalBookCopyLentToReader(payloadJSON []byte) (core.DomainEvent, error) {
	payload := new(core.BookCopyLentToReader)

	err := jsoniter.ConfigFastest.Unmarshal(payloadJSON, &payload)
	if err != nil {
		return core.BookCopyLentToReader{}, errors.Join(ErrMappingToDomainEventFailed, err)
	}

	return core.BookCopyLentToReader{
		BookID:     payload.BookID,
		ReaderID:   payload.ReaderID,
		OccurredAt: payload.OccurredAt,
	}, nil
}

func unmarshalBookCopyReturnedByReader(payloadJSON []byte) (core.DomainEvent, error) {
	payload := new(core.BookCopyReturnedByReader)

	err := jsoniter.ConfigFastest.Unmarshal(payloadJSON, &payload)
	if err != nil {
		return core.BookCopyReturnedByReader{}, errors.Join(ErrMappingToDomainEventFailed, err)
	}

	return core.BookCopyReturnedByReader{
		BookID:     payload.BookID,
		ReaderID:   payload.ReaderID,
		OccurredAt: payload.OccurredAt,
	}, nil
}

func unmarshalSomethingHasHappened(payloadJSON []byte) (core.DomainEvent, error) {
	payload := new(core.SomethingHasHappened)

	err := jsoniter.ConfigFastest.Unmarshal(payloadJSON, &payload)
	if err != nil {
		return core.BookCopyReturnedByReader{}, errors.Join(ErrMappingToDomainEventFailed, err)
	}

	return core.BuildSomethingHasHappened(payload.ID, payload.SomeInformation, payload.OccurredAt, payload.DynamicEventType), nil
}
