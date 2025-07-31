package core

import (
	"time"
)

// Instead of implementing full value objects, I'm using some alias types and helper methods here ...

type (
	// BookIDString represents a book identifier
	BookIDString = string

	// ReaderIDString represents a reader identifier
	ReaderIDString = string

	// ISBNString represents an ISBN identifier
	ISBNString = string

	// OccurredAtTS represents when an event occurred
	OccurredAtTS = time.Time

	// ProducedNewEventToAppendBool indicates whether a new event has been produced and is ready to be appended.
	ProducedNewEventToAppendBool = bool
)

// ToOccurredAt converts a time to OccurredAtTS with UTC normalization and microsecond precision
func ToOccurredAt(t time.Time) OccurredAtTS {
	return t.UTC().Truncate(time.Microsecond)
}
