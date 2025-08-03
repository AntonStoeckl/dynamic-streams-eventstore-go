package lendbookcopytoreader

import (
	"time"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// Command represents the intent to lend a book copy to a reader.
// It encapsulates all the necessary information required to execute the lend book copy use case.
type Command struct {
	BookID     uuid.UUID
	ReaderID   uuid.UUID
	OccurredAt core.OccurredAtTS
}

// BuildCommand creates a new Command with the provided parameters.
func BuildCommand(bookID uuid.UUID, readerID uuid.UUID, occurredAt time.Time) Command {
	return Command{
		BookID:     bookID,
		ReaderID:   readerID,
		OccurredAt: core.ToOccurredAt(occurredAt),
	}
}