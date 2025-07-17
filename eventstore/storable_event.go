package eventstore

type StorableEvent struct {
	eventType   string
	payloadJSON []byte
}

func BuildStorableEvent(eventType string, payloadJSON []byte) StorableEvent {
	return StorableEvent{
		eventType:   eventType,
		payloadJSON: payloadJSON,
	}
}

func (e StorableEvent) EventType() string {
	return e.eventType
}

func (e StorableEvent) PayloadJSON() []byte {
	return e.payloadJSON
}

type StorableEvents []StorableEvent
