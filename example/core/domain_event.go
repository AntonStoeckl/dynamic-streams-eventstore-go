package core

import (
	"time"
)

type DomainEvents = []DomainEvent

type DomainEvent interface {
	EventType() string
	HasOccurredAt() time.Time
}
