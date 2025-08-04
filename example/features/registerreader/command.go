package registerreader

import (
	"time"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// Command represents the intent to register a new reader.
// It encapsulates all the necessary information required to execute the register reader use case.
type Command struct {
	ReaderID   uuid.UUID
	Name       string
	OccurredAt core.OccurredAtTS
}

// BuildCommand creates a new Command with the provided parameters.
func BuildCommand(
	readerID uuid.UUID,
	name string,
	occurredAt time.Time,
) Command {

	return Command{
		ReaderID:   readerID,
		Name:       name,
		OccurredAt: core.ToOccurredAt(occurredAt),
	}
}