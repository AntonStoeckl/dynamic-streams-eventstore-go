package returnbookcopyfromreader

import (
	"time"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

const (
	commandType = "ReturnBookCopyFromReader"
)

// Command represents the intent to return a book copy from a reader.
// It encapsulates all the necessary information required to execute the return book copy use case.
type Command struct {
	BookID     uuid.UUID
	ReaderID   uuid.UUID
	OccurredAt core.OccurredAtTS
}

// CommandType returns the type of this command for observability and routing purposes.
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
