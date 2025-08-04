package readercontractcanceled

import (
	"time"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// Command represents the intent to cancel a reader's contract.
// It encapsulates all the necessary information required to execute the reader contract cancellation use case.
type Command struct {
	ReaderID   uuid.UUID
	OccurredAt core.OccurredAtTS
}

// BuildCommand creates a new Command with the provided parameters.
func BuildCommand(
	readerID uuid.UUID,
	occurredAt time.Time,
) Command {

	return Command{
		ReaderID:   readerID,
		OccurredAt: core.ToOccurredAt(occurredAt),
	}
}
