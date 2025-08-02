package removebookcopy

import (
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// Decide implements the business logic to determine whether a book copy should be removed from circulation.
// This is a pure function with no side effects - it takes the current domain events and a command
// and returns the events that should be appended based on the business rules.
//
// Business rule: A book copy can only be removed if it currently exists in circulation.
// The existence is determined by analyzing the event history - the last relevant event determines the current state.
//
// In a real application there should be a unit test for this business logic, unless coarse-grained
// feature tests are preferred. Complex business logic typically benefits from dedicated pure unit tests.
func Decide(history core.DomainEvents, command Command) core.DomainEvents {
	bookIsInCirculation := false
	bookWasInCirculation := false

	for _, event := range history {
		switch event.EventType() {
		case core.BookCopyAddedToCirculationEventType:
			bookIsInCirculation = true
			bookWasInCirculation = true

		case core.BookCopyRemovedFromCirculationEventType:
			bookIsInCirculation = false
		}
	}

	if !bookWasInCirculation {
		// in a real application this would be a real error event
		return core.DomainEvents{
			core.BuildSomethingHasHappened(
				command.BookID.String(),
				"book is not in circulation",
				command.OccurredAt,
				"RemovingBookFromCirculationFailed",
			),
		}
	}

	if !bookIsInCirculation {
		// idempotency - the book was already removed, so no new event
		return core.DomainEvents{}
	}

	return core.DomainEvents{core.BuildBookCopyRemovedFromCirculation(command.BookID, command.OccurredAt)}
}
