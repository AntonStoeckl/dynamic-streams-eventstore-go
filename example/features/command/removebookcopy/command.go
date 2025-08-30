package removebookcopy

import (
	"time"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

const (
	commandType = "RemoveBookCopy"
)

// Command represents the intent to remove a book copy from circulation.
// It encapsulates all the necessary information required to execute the remove book copy use case.
type Command struct {
	BookID     uuid.UUID
	OccurredAt core.OccurredAtTS
}

// CommandType returns the type of this command for observability and routing purposes.
func (c Command) CommandType() string {
	return commandType
}

// BuildCommand creates a new Command with the provided parameters.
func BuildCommand(bookID uuid.UUID, occurredAt time.Time) Command {
	return Command{
		BookID:     bookID,
		OccurredAt: core.ToOccurredAt(occurredAt),
	}
}
