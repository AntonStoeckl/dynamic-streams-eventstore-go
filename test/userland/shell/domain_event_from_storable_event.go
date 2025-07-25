package shell

import (
	"errors"
	"strings"

	jsoniter "github.com/json-iterator/go"

	"github.com/AntonStoeckl/dynamic-streams-eventstore/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore/test/userland/core"
)

var ErrMappingToDomainEventFailed = errors.New("mapping to domain event failed")
var ErrMappingToDomainEventUnknownEventType = errors.New("unknown event type")

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

func DomainEventFrom(storableEvent eventstore.StorableEvent) (core.DomainEvent, error) {
	switch storableEvent.EventType {
	case core.BookCopyAddedToCirculationEventType:
		payload := new(core.BookCopyAddedToCirculation)

		err := jsoniter.ConfigFastest.Unmarshal(storableEvent.PayloadJSON, &payload)
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

	case core.BookCopyRemovedFromCirculationEventType:
		payload := new(core.BookCopyRemovedFromCirculation)

		err := jsoniter.ConfigFastest.Unmarshal(storableEvent.PayloadJSON, &payload)
		if err != nil {
			return core.BookCopyRemovedFromCirculation{}, errors.Join(ErrMappingToDomainEventFailed, err)
		}

		return core.BookCopyRemovedFromCirculation{
			BookID:     payload.BookID,
			OccurredAt: payload.OccurredAt,
		}, nil

	case core.BookCopyLentToReaderEventType:
		payload := new(core.BookCopyLentToReader)

		err := jsoniter.ConfigFastest.Unmarshal(storableEvent.PayloadJSON, &payload)
		if err != nil {
			return core.BookCopyLentToReader{}, errors.Join(ErrMappingToDomainEventFailed, err)
		}

		return core.BookCopyLentToReader{
			BookID:     payload.BookID,
			ReaderID:   payload.ReaderID,
			OccurredAt: payload.OccurredAt,
		}, nil

	case core.BookCopyReturnedByReaderEventType:
		payload := new(core.BookCopyReturnedByReader)

		err := jsoniter.ConfigFastest.Unmarshal(storableEvent.PayloadJSON, &payload)
		if err != nil {
			return core.BookCopyReturnedByReader{}, errors.Join(ErrMappingToDomainEventFailed, err)
		}

		return core.BookCopyReturnedByReader{
			BookID:     payload.BookID,
			ReaderID:   payload.ReaderID,
			OccurredAt: payload.OccurredAt,
		}, nil

	default:
		if strings.Contains(storableEvent.EventType, core.SomethingHasHappenedEventTypePrefix) {
			payload := new(core.SomethingHasHappened)

			err := jsoniter.ConfigFastest.Unmarshal(storableEvent.PayloadJSON, &payload)
			if err != nil {
				return core.BookCopyReturnedByReader{}, errors.Join(ErrMappingToDomainEventFailed, err)
			}

			return core.BuildSomethingHasHappened(payload.ID, payload.SomeInformation, payload.OccurredAt, payload.DynamicEventType), nil

		}
	}

	return nil, errors.Join(ErrMappingToDomainEventFailed, ErrMappingToDomainEventUnknownEventType)
}
