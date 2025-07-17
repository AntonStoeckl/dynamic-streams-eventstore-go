package userland

import (
	jsoniter "github.com/json-iterator/go"
)

const BookCopyAddedToCirculationEventType = EventTypeString("BookCopyAddedToCirculation")

type BookCopyAddedToCirculation struct {
	eventType string
	Payload   BookCopyAddedToCirculationPayload
}

type BookCopyAddedToCirculationPayload struct {
	BookID          string
	ISBN            string
	Title           string
	Authors         string
	Edition         string
	Publisher       string
	PublicationYear uint
}

func BookCopyAddedToCirculationFromPayload(payload BookCopyAddedToCirculationPayload) BookCopyAddedToCirculation {
	return BookCopyAddedToCirculation{
		eventType: BookCopyAddedToCirculationEventType,
		Payload: BookCopyAddedToCirculationPayload{
			BookID:          payload.BookID,
			ISBN:            payload.ISBN,
			Title:           payload.Title,
			Authors:         payload.Authors,
			Edition:         payload.Edition,
			Publisher:       payload.Publisher,
			PublicationYear: payload.PublicationYear,
		},
	}
}

func BookCopyAddedToCirculationFromJSON(eventJSON []byte) (BookCopyAddedToCirculation, error) {
	payload := new(BookCopyAddedToCirculationPayload)
	err := jsoniter.ConfigFastest.Unmarshal(eventJSON, &payload)
	if err != nil {
		return BookCopyAddedToCirculation{}, err
	}

	return BookCopyAddedToCirculation{
		eventType: BookCopyAddedToCirculationEventType,
		Payload: BookCopyAddedToCirculationPayload{
			BookID:          payload.BookID,
			ISBN:            payload.ISBN,
			Title:           payload.Title,
			Authors:         payload.Authors,
			Edition:         payload.Edition,
			Publisher:       payload.Publisher,
			PublicationYear: payload.PublicationYear,
		},
	}, nil
}

func (bc BookCopyAddedToCirculation) EventType() string {
	return bc.eventType
}

func (bc BookCopyAddedToCirculation) PayloadToJSON() ([]byte, error) {
	return jsoniter.ConfigFastest.Marshal(bc.Payload)
}
