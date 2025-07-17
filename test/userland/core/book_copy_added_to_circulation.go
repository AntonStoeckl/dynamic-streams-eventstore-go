package core

const BookCopyAddedToCirculationEventType = EventTypeString("BookCopyAddedToCirculation")

type BookCopyAddedToCirculation struct {
	BookID          BookIDString
	ISBN            string
	Title           string
	Authors         string
	Edition         string
	Publisher       string
	PublicationYear uint
}

func (e BookCopyAddedToCirculation) EventType() EventTypeString {
	return BookCopyAddedToCirculationEventType
}

func (e BookCopyAddedToCirculation) IsDomainEvent() bool {
	return true
}
