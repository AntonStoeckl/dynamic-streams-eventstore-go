package userland

import (
	jsoniter "github.com/json-iterator/go"
)

const BookCopyLentToReaderEventType = "BookCopyLentToReader"

type BookCopyLentToReader struct {
	eventType EventTypeString
	Payload   BookCopyLentToReaderPayload
}

type BookCopyLentToReaderPayload struct {
	BookID   BookIDString
	ReaderID ReaderIDString
}

func BookCopyLentToReaderFromPayload(payload BookCopyLentToReaderPayload) BookCopyLentToReader {
	return BookCopyLentToReader{
		eventType: BookCopyLentToReaderEventType,
		Payload: BookCopyLentToReaderPayload{
			BookID:   payload.BookID,
			ReaderID: payload.ReaderID,
		},
	}
}

func BookCopyLentToReaderFromJSON(eventJSON []byte) (BookCopyLentToReader, error) {
	payload := new(BookCopyLentToReaderPayload)
	err := jsoniter.ConfigFastest.Unmarshal(eventJSON, &payload)
	if err != nil {
		return BookCopyLentToReader{}, err
	}

	return BookCopyLentToReader{
		eventType: BookCopyLentToReaderEventType,
		Payload: BookCopyLentToReaderPayload{
			BookID:   payload.BookID,
			ReaderID: payload.ReaderID,
		},
	}, nil
}

func (bc BookCopyLentToReader) EventType() EventTypeString {
	return bc.eventType
}

func (bc BookCopyLentToReader) PayloadToJSON() ([]byte, error) {
	return jsoniter.ConfigFastest.Marshal(bc.Payload)
}
