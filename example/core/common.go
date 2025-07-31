package core

import (
	"time"
)

// Instead of implementing full value objects, I'm using some alias types and helper methods here ...

// BookIDString represents a book identifier
type BookIDString = string

// ReaderIDString represents a reader identifier
type ReaderIDString = string

// ISBNString represents an ISBN identifier
type ISBNString = string

// OccurredAt represents when an event occurred
type OccurredAt = time.Time

// ToOccurredAt converts a time to OccurredAt with UTC normalization and microsecond precision
func ToOccurredAt(t time.Time) OccurredAt {
	return t.UTC().Truncate(time.Microsecond)
}
