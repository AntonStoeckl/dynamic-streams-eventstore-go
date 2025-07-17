package core

const SomethingHasHappenedEventTypePrefix = EventTypeString("SomethingHasHappened")

type SomethingHasHappened struct {
	ID               string
	SomeInformation  string
	DynamicEventType EventTypeString
}

func BuildSomethingHasHappened(id string, someInformation string, dynamicEventType string) SomethingHasHappened {
	return SomethingHasHappened{
		ID:               id,
		SomeInformation:  someInformation,
		DynamicEventType: dynamicEventType,
	}
}

func (e SomethingHasHappened) EventType() EventTypeString {
	return e.DynamicEventType
}

func (e SomethingHasHappened) IsDomainEvent() bool {
	return true
}
