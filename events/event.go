package events

type EventTypeString = string
type Events = []Event

type BookIDString = string
type ReaderIDString = string

type Event interface {
	EventType() string
	PayloadToJSON() ([]byte, error)
}
