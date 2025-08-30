package addbookcopy

import (
	"time"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// Command represents the intent to add a book copy to circulation.
// It encapsulates all the necessary information required to execute the add book copy use case.
type Command struct {
	BookID          uuid.UUID
	ISBN            string
	Title           string
	Authors         string
	Edition         string
	Publisher       string
	PublicationYear uint
	OccurredAt      core.OccurredAtTS
}

// CommandType returns the type identifier for this command.
func (c Command) CommandType() string {
	return "AddBookCopy"
}

// BuildCommand creates a new Command with the provided parameters.
func BuildCommand(
	bookID uuid.UUID,
	isbn string,
	title string,
	authors string,
	edition string,
	publisher string,
	publicationYear uint,
	occurredAt time.Time,
) Command {

	return Command{
		BookID:          bookID,
		ISBN:            isbn,
		Title:           title,
		Authors:         authors,
		Edition:         edition,
		Publisher:       publisher,
		PublicationYear: publicationYear,
		OccurredAt:      core.ToOccurredAt(occurredAt),
	}
}
