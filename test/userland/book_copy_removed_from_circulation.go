package userland

import (
	jsoniter "github.com/json-iterator/go"
)

const BookCopyRemovedFromCirculationEventType = "BookCopyRemovedFromCirculation"

type BookCopyRemovedFromCirculation struct {
	eventType string
	Payload   BookCopyRemovedFromCirculationPayload
}

type BookCopyRemovedFromCirculationPayload struct {
	BookID string
}

func BookCopyRemovedFromCirculationFromPayload(payload BookCopyRemovedFromCirculationPayload) BookCopyRemovedFromCirculation {
	return BookCopyRemovedFromCirculation{
		eventType: BookCopyRemovedFromCirculationEventType,
		Payload: BookCopyRemovedFromCirculationPayload{
			BookID: payload.BookID,
		},
	}
}

func BookCopyRemovedFromCirculationFromJSON(eventJSON []byte) (BookCopyRemovedFromCirculation, error) {
	payload := new(BookCopyRemovedFromCirculationPayload)
	err := jsoniter.ConfigFastest.Unmarshal(eventJSON, &payload)
	if err != nil {
		return BookCopyRemovedFromCirculation{}, err
	}

	return BookCopyRemovedFromCirculation{
		eventType: BookCopyRemovedFromCirculationEventType,
		Payload: BookCopyRemovedFromCirculationPayload{
			BookID: payload.BookID,
		},
	}, nil
}

func (bc BookCopyRemovedFromCirculation) EventType() string {
	return bc.eventType
}

func (bc BookCopyRemovedFromCirculation) PayloadToJSON() ([]byte, error) {
	return jsoniter.ConfigFastest.Marshal(bc.Payload)
}
