package cancelreadercontract

import (
	"errors"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

const (
	failureReasonOutstandingLoans      = "reader has outstanding book loans"
	failureReasonReaderNeverRegistered = "reader was never registered"
)

// state represents the current state projected from the event history.
type state struct {
	readerIsRegistered       bool
	readerWasNeverRegistered bool
	outstandingBookLoans     map[core.BookIDString]bool
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
//	ERROR: "reader has outstanding book loans" if the reader currently has books borrowed
//	IDEMPOTENCY: If reader not registered or already canceled, no event generated (no-op)
func Decide(history core.DomainEvents, command Command) core.DecisionResult {
	s := project(history, command.ReaderID.String())

	if s.readerWasNeverRegistered {
		event := core.BuildCancelingReaderContractFailed(
			command.ReaderID,
			failureReasonReaderNeverRegistered,
			command.OccurredAt,
		)

		return core.ErrorDecision(event, errors.New(event.EventType+": "+failureReasonReaderNeverRegistered))
	}

	// idempotency - the reader is not registered or already canceled, so no new event
	if !s.readerIsRegistered {
		return core.IdempotentDecision()
	}

	if len(s.outstandingBookLoans) > 0 {
		event := core.BuildCancelingReaderContractFailed(
			command.ReaderID,
			failureReasonOutstandingLoans,
			command.OccurredAt,
		)

		return core.ErrorDecision(event, errors.New(event.EventType+": "+failureReasonOutstandingLoans))
	}

	return core.SuccessDecision(
		core.BuildReaderContractCanceled(
			command.ReaderID,
			command.OccurredAt,
		),
	)
}

// project builds the current state by replaying all events from the history.
func project(history core.DomainEvents, readerID string) state {
	s := state{
		readerIsRegistered:       false, // Default to "not registered"
		readerWasNeverRegistered: true,  // Default to "never registered"
		outstandingBookLoans:     make(map[core.BookIDString]bool),
	}

	for _, event := range history {
		switch e := event.(type) {
		case core.ReaderRegistered:
			if e.ReaderID == readerID {
				s.readerIsRegistered = true
				s.readerWasNeverRegistered = false
			}

		case core.ReaderContractCanceled:
			if e.ReaderID == readerID {
				s.readerIsRegistered = false
				s.readerWasNeverRegistered = false
			}

		case core.BookCopyLentToReader:
			if e.ReaderID == readerID {
				s.outstandingBookLoans[e.BookID] = true
			}

		case core.BookCopyReturnedByReader:
			if e.ReaderID == readerID {
				delete(s.outstandingBookLoans, e.BookID)
			}
		}
	}

	return s
}

// BuildEventFilter creates the filter for querying all events
// related to the specified reader which are relevant for this feature/use-case.
func BuildEventFilter(readerID uuid.UUID) eventstore.Filter {
	return eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			core.ReaderRegisteredEventType,
			core.ReaderContractCanceledEventType,
			core.BookCopyLentToReaderEventType,
			core.BookCopyReturnedByReaderEventType,
		).
		AndAnyPredicateOf(
			eventstore.P("ReaderID", readerID.String()),
		).
		Finalize()
}
