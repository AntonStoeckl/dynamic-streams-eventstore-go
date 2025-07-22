package eventstore

import (
	"time"
)

type StorableEvents []StorableEvent

type StorableEvent struct {
	EventType   string
	OccurredAt  time.Time
	PayloadJSON []byte
}
