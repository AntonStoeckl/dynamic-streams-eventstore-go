package lendbookcopytoreader

import (
	"time"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

const (
	commandType = "LendBookCopy"
)

// Command represents the intent to lend a book copy to a reader.
// It encapsulates all the necessary information required to execute the lend book copy use case.
type Command struct {
	BookID     uuid.UUID
	ReaderID   uuid.UUID
	OccurredAt core.OccurredAtTS
}

// CommandType returns the type identifier for this command, used for observability and routing.
func (c Command) CommandType() string {
	return commandType
}

// BuildCommand creates a new Command with the provided parameters.
func BuildCommand(bookID uuid.UUID, readerID uuid.UUID, occurredAt time.Time) Command {
	return Command{
		BookID:     bookID,
		ReaderID:   readerID,
		OccurredAt: core.ToOccurredAt(occurredAt),
	}
}
