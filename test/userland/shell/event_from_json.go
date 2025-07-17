package shell

import (
	"errors"
	"strings"

	"dynamic-streams-eventstore/test/userland/core"
)

func EventFromJSON(eventType core.EventTypeString, payload []byte) (core.Event, error) {
	switch eventType {
	case core.BookCopyAddedToCirculationEventType:
		event, unmarshallingErr := core.BookCopyAddedToCirculationFromJSON(payload)
		if unmarshallingErr != nil {
			return nil, errors.Join(errors.New("unmarshalling event from json failed"), unmarshallingErr)
		}

		return event, nil

	case core.BookCopyRemovedFromCirculationEventType:
		event, unmarshallingErr := core.BookCopyRemovedFromCirculationFromJSON(payload)
		if unmarshallingErr != nil {
			return nil, errors.Join(errors.New("unmarshalling event from json failed"), unmarshallingErr)
		}

		return event, nil

	case core.BookCopyLentToReaderEventType:
		event, unmarshallingErr := core.BookCopyLentToReaderFromJSON(payload)
		if unmarshallingErr != nil {
			return nil, errors.Join(errors.New("unmarshalling event from json failed"), unmarshallingErr)
		}

		return event, nil

	case core.BookCopyReturnedByReaderEventType:
		event, unmarshallingErr := core.BookCopyReturnedByReaderFromJSON(payload)
		if unmarshallingErr != nil {
			return nil, errors.Join(errors.New("unmarshalling event from json failed"), unmarshallingErr)
		}

		return event, nil

	default:
		if strings.Contains(eventType, core.SomethingHasHappenedEventTypePrefix) {
			event, unmarshallingErr := core.SomethingHasHappenedFromJSON(payload)
			if unmarshallingErr != nil {
				return nil, errors.Join(errors.New("unmarshalling event from json failed"), unmarshallingErr)
			}

			return event, nil
		}
	}

	return nil, errors.New("unknown event type")
}
