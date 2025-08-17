package lendbookcopytoreader

import (
	"errors"

	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

const (
	maxBookLoansAllowedPerReader      = 10
	failureReasonBookNotInCirculation = "book is not in circulation"
	failureReasonBookAlreadyLent      = "book is already lent"
	failureReasonReaderTooManyBooks   = "reader has too many books"
	failureReasonReaderNotRegistered  = "reader is not currently registered"
)

// state represents the current state projected from the event history.
type state struct {
	bookIsNotInCirculation    bool
	bookIsLentToThisReader    bool
	bookIsLentToAnotherReader bool
	readerCurrentBookCount    int
	readerIsNotRegistered     bool
}

// Decide implements the business logic to determine whether a book copy should be lent to a reader.
// This is a pure function with no side effects - it takes the current domain events and a command
// and returns the events that should be appended based on the business rules.
//
// Business Rules:
//
//		GIVEN: A book copy with BookID and reader with ReaderID
//		WHEN: LendBookCopyToReader command is received
//		THEN: BookCopyLentToReader event is generated
//		ERROR: "book is not in circulation" if a book not added or was removed
//		ERROR: "book is already lent" if book currently lent to any reader
//		ERROR: "reader has too many books" if reader already has 10 books lent
//	 ERROR: "reader is not currently registered" if reader is not registered
//		IDEMPOTENCY: If book already lent to this reader, no event generated (no-op)
func Decide(history core.DomainEvents, command Command) core.DecisionResult {
	s := project(history, command.BookID.String(), command.ReaderID.String())

	if s.bookIsLentToThisReader {
		return core.IdempotentDecision() // idempotency - the book is already lent to this reader, so no new event
	}

	// Strict validation: reader must be currently registered (not cancelled)
	if s.readerIsNotRegistered {
		event := core.BuildLendingBookToReaderFailed(command.BookID, command.ReaderID, failureReasonReaderNotRegistered, command.OccurredAt)
		return core.ErrorDecision(event, errors.New(event.EventType+": "+failureReasonReaderNotRegistered))
	}

	if s.bookIsNotInCirculation {
		event := core.BuildLendingBookToReaderFailed(command.BookID, command.ReaderID, failureReasonBookNotInCirculation, command.OccurredAt)
		return core.ErrorDecision(event, errors.New(event.EventType+": "+failureReasonBookNotInCirculation))
	}

	if s.bookIsLentToAnotherReader {
		event := core.BuildLendingBookToReaderFailed(command.BookID, command.ReaderID, failureReasonBookAlreadyLent, command.OccurredAt)
		return core.ErrorDecision(event, errors.New(event.EventType+": "+failureReasonBookAlreadyLent))
	}

	if s.readerCurrentBookCount >= maxBookLoansAllowedPerReader {
		event := core.BuildLendingBookToReaderFailed(command.BookID, command.ReaderID, failureReasonReaderTooManyBooks, command.OccurredAt)
		return core.ErrorDecision(event, errors.New(event.EventType+": "+failureReasonReaderTooManyBooks))
	}

	return core.SuccessDecision(
		core.BuildBookCopyLentToReader(
			command.BookID,
			command.ReaderID,
			command.OccurredAt,
		),
	)
}

// project builds the current state by replaying all events from the history.
func project(history core.DomainEvents, bookID string, readerID string) state { //nolint:gocognit // Complex business logic with multiple domain rule checks
	s := state{
		bookIsNotInCirculation:    true,  // Default to "not in circulation"
		bookIsLentToThisReader:    false, // Default to "not lent to this reader"
		bookIsLentToAnotherReader: false, // Default to "not lent to another reader"
		readerCurrentBookCount:    0,     // Default to "no books"
		readerIsNotRegistered:     true,  // Default to "not registered"
	}

	for _, event := range history {
		switch e := event.(type) {
		case core.BookCopyAddedToCirculation:
			if e.BookID == bookID {
				s.bookIsNotInCirculation = false
			}

		case core.BookCopyRemovedFromCirculation:
			if e.BookID == bookID {
				s.bookIsNotInCirculation = true
			}

		case core.BookCopyLentToReader:
			if e.BookID == bookID {
				if e.ReaderID == readerID {
					s.bookIsLentToThisReader = true
				} else {
					s.bookIsLentToAnotherReader = true
				}
			}

			if e.ReaderID == readerID {
				s.readerCurrentBookCount++
			}

		case core.BookCopyReturnedByReader:
			if e.BookID == bookID {
				if e.ReaderID == readerID {
					s.bookIsLentToThisReader = false
				} else {
					s.bookIsLentToAnotherReader = false
				}
			}

			if e.ReaderID == readerID {
				s.readerCurrentBookCount--
			}

		case core.ReaderRegistered:
			if e.ReaderID == readerID {
				s.readerIsNotRegistered = false
			}

		case core.ReaderContractCanceled:
			if e.ReaderID == readerID {
				s.readerIsNotRegistered = true
			}
		}
	}

	return s
}

// BuildEventFilter creates the filter for querying all events
// related to the specified book and reader which are relevant for this feature/use-case.
func BuildEventFilter(bookID uuid.UUID, readerID uuid.UUID) eventstore.Filter {
	return eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			core.BookCopyAddedToCirculationEventType,
			core.BookCopyRemovedFromCirculationEventType,
			core.BookCopyLentToReaderEventType,
			core.BookCopyReturnedByReaderEventType,
			core.ReaderRegisteredEventType,
			core.ReaderContractCanceledEventType,
		).
		AndAnyPredicateOf(
			eventstore.P("BookID", bookID.String()),
			eventstore.P("ReaderID", readerID.String()),
		).
		Finalize()
}
