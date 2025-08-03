package addbookcopy

import (
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// state represents the current state projected from the event history.
type state struct {
	bookIsInCirculation bool
}

// Decide implements the business logic to determine whether a book copy should be added to circulation.
// This is a pure function with no side effects - it takes the current domain events and a command
// and returns the events that should be appended based on the business rules.
//
// Business Rules:
//
//	GIVEN: A book copy with BookID
//	WHEN: AddBookCopyToCirculation command is received
//	THEN: BookCopyAddedToCirculation event is generated
//	ERROR: None (always succeeds)
//	IDEMPOTENCY: If book already in circulation, no event generated (no-op)
func Decide(history core.DomainEvents, command Command) core.DomainEvents {
	s := project(history, command.BookID.String())

	if s.bookIsInCirculation {
		return core.DomainEvents{} // idempotency - the book is already in circulation, so no new event
	}

	return core.DomainEvents{
		core.BuildBookCopyAddedToCirculation(
			command.BookID,
			command.ISBN,
			command.Title,
			command.Authors,
			command.Edition,
			command.Publisher,
			command.PublicationYear,
			command.OccurredAt,
		),
	}
}

// project builds the current state by replaying all events from the history.
func project(history core.DomainEvents, bookID string) state {
	s := state{
		bookIsInCirculation: false, // Default to "not in circulation"
	}

	for _, event := range history {
		switch e := event.(type) {
		case core.BookCopyAddedToCirculation:
			if e.BookID == bookID {
				s.bookIsInCirculation = true
			}

		case core.BookCopyRemovedFromCirculation:
			if e.BookID == bookID {
				s.bookIsInCirculation = false
			}
		}
	}

	return s
}
