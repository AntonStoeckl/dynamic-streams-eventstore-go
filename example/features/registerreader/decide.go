package registerreader

import (
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// state represents the current state projected from the event history.
type state struct {
	readerIsRegistered bool
}

// Decide implements the business logic to determine whether a reader should be registered.
// This is a pure function with no side effects - it takes the current domain events and a command
// and returns the events that should be appended based on the business rules.
//
// Business Rules:
//   GIVEN: A reader with ReaderID
//   WHEN: RegisterReader command is received
//   THEN: ReaderRegistered event is generated
//   ERROR: None (always succeeds)
//   IDEMPOTENCY: If reader already registered, no event generated (no-op)
func Decide(history core.DomainEvents, command Command) core.DomainEvents {
	s := project(history, command.ReaderID.String())

	if s.readerIsRegistered {
		return core.DomainEvents{} // idempotency - the reader is already registered, so no new event
	}

	return core.DomainEvents{
		core.BuildReaderRegistered(
			command.ReaderID,
			command.Name,
			command.OccurredAt,
		),
	}
}

// project builds the current state by replaying all events from the history.
func project(history core.DomainEvents, readerID string) state {
	s := state{
		readerIsRegistered: false, // Default to "not registered"
	}

	for _, event := range history {
		switch e := event.(type) {
		case core.ReaderRegistered:
			if e.ReaderID == readerID {
				s.readerIsRegistered = true
			}

		case core.ReaderContractCanceled:
			if e.ReaderID == readerID {
				s.readerIsRegistered = false
			}
		}
	}

	return s
}