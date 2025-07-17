package core

const BookCopyReturnedByReaderEventType = EventTypeString("BookCopyReturnedByReader")

type BookCopyReturnedByReader struct {
	BookID   BookIDString
	ReaderID ReaderIDString
}

func (e BookCopyReturnedByReader) EventType() EventTypeString {
	return BookCopyReturnedByReaderEventType
}

func (e BookCopyReturnedByReader) IsDomainEvent() bool {
	return true
}
