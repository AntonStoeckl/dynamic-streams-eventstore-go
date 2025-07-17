package core

const BookCopyLentToReaderEventType = EventTypeString("BookCopyLentToReader")

type BookCopyLentToReader struct {
	BookID   BookIDString
	ReaderID ReaderIDString
}

func (e BookCopyLentToReader) EventType() EventTypeString {
	return BookCopyLentToReaderEventType
}

func (e BookCopyLentToReader) IsDomainEvent() bool {
	return true
}
