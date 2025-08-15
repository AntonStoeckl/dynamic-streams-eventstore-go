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

	case core.LendingBookToReaderFailedEventType:
		return unmarshalLendingBookToReaderFailed(storableEvent.PayloadJSON)

	case core.ReturningBookFromReaderFailedEventType:
		return unmarshalReturningBookFromReaderFailed(storableEvent.PayloadJSON)

	case core.RemovingBookFromCirculationFailedEventType:
		return unmarshalRemovingBookFromCirculationFailed(storableEvent.PayloadJSON)

	case core.ReaderRegisteredEventType:
		return unmarshalReaderRegistered(storableEvent.PayloadJSON)

	case core.ReaderContractCanceledEventType:
		return unmarshalReaderContractCanceled(storableEvent.PayloadJSON)

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

func unmarshalLendingBookToReaderFailed(payloadJSON []byte) (core.DomainEvent, error) {
	var event core.LendingBookToReaderFailed

	err := jsoniter.ConfigFastest.Unmarshal(payloadJSON, &event)
	if err != nil {
		return core.LendingBookToReaderFailed{}, errors.Join(ErrMappingToDomainEventFailed, err)
	}

	return event, nil
}

func unmarshalReturningBookFromReaderFailed(payloadJSON []byte) (core.DomainEvent, error) {
	var event core.ReturningBookFromReaderFailed

	err := jsoniter.ConfigFastest.Unmarshal(payloadJSON, &event)
	if err != nil {
		return core.ReturningBookFromReaderFailed{}, errors.Join(ErrMappingToDomainEventFailed, err)
	}

	return event, nil
}

func unmarshalRemovingBookFromCirculationFailed(payloadJSON []byte) (core.DomainEvent, error) {
	var event core.RemovingBookFromCirculationFailed

	err := jsoniter.ConfigFastest.Unmarshal(payloadJSON, &event)
	if err != nil {
		return core.RemovingBookFromCirculationFailed{}, errors.Join(ErrMappingToDomainEventFailed, err)
	}

	return event, nil
}

func unmarshalReaderRegistered(payloadJSON []byte) (core.DomainEvent, error) {
	var event core.ReaderRegistered

	err := jsoniter.ConfigFastest.Unmarshal(payloadJSON, &event)
	if err != nil {
		return core.ReaderRegistered{}, errors.Join(ErrMappingToDomainEventFailed, err)
	}

	return event, nil
}

func unmarshalReaderContractCanceled(payloadJSON []byte) (core.DomainEvent, error) {
	var event core.ReaderContractCanceled

	err := jsoniter.ConfigFastest.Unmarshal(payloadJSON, &event)
	if err != nil {
		return core.ReaderContractCanceled{}, errors.Join(ErrMappingToDomainEventFailed, err)
	}

	return event, nil
}
