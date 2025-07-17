package events

import (
	"errors"
	"strings"
)

type EventTypeString = string
type Events = []Event

type BookIDString = string
type ReaderIDString = string

type Event interface {
	EventType() string
	PayloadToJSON() ([]byte, error)
}

func EventFromJSON(eventType EventTypeString, payload []byte) (Event, error) {
	switch eventType {
	case BookCopyAddedToCirculationEventType:
		event, unmarshallingErr := BookCopyAddedToCirculationFromJSON(payload)
		if unmarshallingErr != nil {
			return nil, errors.Join(errors.New("unmarshalling event from json failed"), unmarshallingErr)
		}

		return event, nil

	case BookCopyRemovedFromCirculationEventType:
		event, unmarshallingErr := BookCopyRemovedFromCirculationFromJSON(payload)
		if unmarshallingErr != nil {
			return nil, errors.Join(errors.New("unmarshalling event from json failed"), unmarshallingErr)
		}

		return event, nil

	case BookCopyLentToReaderEventType:
		event, unmarshallingErr := BookCopyLentToReaderFromJSON(payload)
		if unmarshallingErr != nil {
			return nil, errors.Join(errors.New("unmarshalling event from json failed"), unmarshallingErr)
		}

		return event, nil

	case BookCopyReturnedByReaderEventType:
		event, unmarshallingErr := BookCopyReturnedByReaderFromJSON(payload)
		if unmarshallingErr != nil {
			return nil, errors.Join(errors.New("unmarshalling event from json failed"), unmarshallingErr)
		}

		return event, nil

	default:
		if strings.Contains(eventType, SomethingHasHappenedEventTypePrefix) {
			event, unmarshallingErr := SomethingHasHappenedFromJSON(payload)
			if unmarshallingErr != nil {
				return nil, errors.Join(errors.New("unmarshalling event from json failed"), unmarshallingErr)
			}

			return event, nil
		}
	}

	return nil, errors.New("unknown event type")
}
