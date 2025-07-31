package removebookcopy

import (
	"time"

	"github.com/google/uuid"
)

// Command represents the intent to remove a book copy from circulation.
// It encapsulates all the necessary information required to execute the remove book copy use case.
type Command struct {
	BookID     uuid.UUID
	OccurredAt time.Time
}

// BuildCommand creates a new Command with the provided parameters.
func BuildCommand(bookID uuid.UUID, occurredAt time.Time) Command {
	return Command{
		BookID:     bookID,
		OccurredAt: occurredAt,
	}
}
