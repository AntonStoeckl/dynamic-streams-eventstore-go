package registeredreaders

import (
	"time"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// ReaderInfo represents information about a registered reader.
type ReaderInfo struct {
	ReaderID     core.ReaderIDString
	Name         string
	RegisteredAt time.Time
}

// RegisteredReaders represents the query result containing all registered (non-canceled) readers.
type RegisteredReaders struct {
	Readers        []ReaderInfo `json:"readers"`
	Count          int          `json:"count"`
	SequenceNumber uint         `json:"sequenceNumber"`
}
