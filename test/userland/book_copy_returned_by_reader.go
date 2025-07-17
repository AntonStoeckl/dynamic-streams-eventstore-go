package userland

import (
	jsoniter "github.com/json-iterator/go"
)

const BookCopyReturnedByReaderEventType = EventTypeString("BookCopyReturnedByReader")

type BookCopyReturnedByReader struct {
	eventType EventTypeString
	Payload   BookCopyReturnedByReaderPayload
}

type BookCopyReturnedByReaderPayload struct {
	BookID   BookIDString
	ReaderID ReaderIDString
}

func BookCopyReturnedByReaderFromPayload(payload BookCopyReturnedByReaderPayload) BookCopyReturnedByReader {
	return BookCopyReturnedByReader{
		eventType: BookCopyReturnedByReaderEventType,
		Payload: BookCopyReturnedByReaderPayload{
			BookID:   payload.BookID,
			ReaderID: payload.ReaderID,
		},
	}
}

func BookCopyReturnedByReaderFromJSON(eventJSON []byte) (BookCopyReturnedByReader, error) {
	payload := new(BookCopyReturnedByReaderPayload)
	err := jsoniter.ConfigFastest.Unmarshal(eventJSON, &payload)
	if err != nil {
		return BookCopyReturnedByReader{}, err
	}

	return BookCopyReturnedByReader{
		eventType: BookCopyReturnedByReaderEventType,
		Payload: BookCopyReturnedByReaderPayload{
			BookID:   payload.BookID,
			ReaderID: payload.ReaderID,
		},
	}, nil
}

func (bc BookCopyReturnedByReader) EventType() EventTypeString {
	return bc.eventType
}

func (bc BookCopyReturnedByReader) PayloadToJSON() ([]byte, error) {
	return jsoniter.ConfigFastest.Marshal(bc.Payload)
}
