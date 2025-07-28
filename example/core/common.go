package core

import (
	"time"
)

// Instead of implementing full value objects, I'm using some alias types and helper methods here ...

type BookIDString = string
type ReaderIDString = string
type ISBNString = string
type OccurredAt = time.Time

func ToOccurredAt(t time.Time) OccurredAt {
	return t.UTC().Truncate(time.Microsecond)
}
