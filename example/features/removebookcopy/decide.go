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
func Decide(domainEvents core.DomainEvents, command Command) core.DomainEvents {
	bookExists := false

	for _, domainEvent := range domainEvents {
		switch domainEvent.EventType() {
		case core.BookCopyAddedToCirculationEventType:
			bookExists = true

		case core.BookCopyRemovedFromCirculationEventType:
			bookExists = false
		}
	}

	if bookExists {
		removalEvent := core.BuildBookCopyRemovedFromCirculation(command.BookID, command.OccurredAt)
		return core.DomainEvents{removalEvent}
	}

	return core.DomainEvents{}
}
