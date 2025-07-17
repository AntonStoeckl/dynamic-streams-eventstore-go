package core

const BookCopyRemovedFromCirculationEventType = EventTypeString("BookCopyRemovedFromCirculation")

type BookCopyRemovedFromCirculation struct {
	BookID BookIDString
}

func (e BookCopyRemovedFromCirculation) EventType() EventTypeString {
	return BookCopyRemovedFromCirculationEventType
}

func (e BookCopyRemovedFromCirculation) IsDomainEvent() bool {
	return true
}
