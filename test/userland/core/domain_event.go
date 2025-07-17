package core

type EventTypeString = string
type DomainEvents = []DomainEvent

type BookIDString = string
type ReaderIDString = string

type DomainEvent interface {
	EventType() EventTypeString
	IsDomainEvent() bool
}
