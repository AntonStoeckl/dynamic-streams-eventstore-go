package readercontractcanceled

import (
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// state represents the current state projected from the event history.
type state struct {
	readerIsRegistered bool
}

// Decide implements the business logic to determine whether a reader's contract should be canceled.
// This is a pure function with no side effects - it takes the current domain events and a command
// and returns the events that should be appended based on the business rules.
//
// Business Rules:
//
//	GIVEN: A reader with ReaderID
//	WHEN: ReaderContractCanceled command is received
//	THEN: ReaderContractCanceled event is generated
//	ERROR: None (always succeeds)
//	IDEMPOTENCY: If reader not registered or already canceled, no event generated (no-op)
func Decide(history core.DomainEvents, command Command) core.DomainEvents {
	s := project(history, command.ReaderID.String())

	if !s.readerIsRegistered {
		return core.DomainEvents{} // idempotency - the reader is not registered or already canceled, so no new event
	}

	return core.DomainEvents{
		core.BuildReaderContractCanceled(
			command.ReaderID,
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
