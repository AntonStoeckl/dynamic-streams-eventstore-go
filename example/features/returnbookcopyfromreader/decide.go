package returnbookcopyfromreader

import (
	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

const (
	failureReasonBookNotInCirculation = "book is not in circulation"
	failureReasonBookNotLentToReader  = "book is not lent to this reader"
)

// state represents the current state projected from the event history.
type state struct {
	bookIsNotInCirculation       bool
	bookWasNeverLentToThisReader bool
	bookIsNotLentToThisReader    bool
}

// Decide implements the business logic to determine whether a book copy should be returned from a reader.
// This is a pure function with no side effects - it takes the current domain events and a command
// and returns the events that should be appended based on the business rules.
//
// Business Rules:
//
//	GIVEN: A book copy with BookID and reader with ReaderID
//	WHEN: ReturnBookCopyFromReader command is received
//	THEN: BookCopyReturnedByReader event is generated
//	ERROR: "book is not in circulation" if a book not added or was removed
//	ERROR: "book is not lent to this reader" if a book not lent to this specific reader
//	IDEMPOTENCY: If book already returned by this reader, no event generated (no-op)
func Decide(history core.DomainEvents, command Command) core.DecisionResult {
	s := project(history, command.BookID.String(), command.ReaderID.String())

	if s.bookIsNotLentToThisReader {
		return core.IdempotentDecision() // idempotency - the book was already returned by this reader, so no new event
	}

	if s.bookIsNotInCirculation {
		return core.ErrorDecision(
			core.BuildReturningBookFromReaderFailed(
				command.BookID,
				command.ReaderID,
				failureReasonBookNotInCirculation,
				command.OccurredAt,
			),
		)
	}

	if s.bookWasNeverLentToThisReader {
		return core.ErrorDecision(
			core.BuildReturningBookFromReaderFailed(
				command.BookID,
				command.ReaderID,
				failureReasonBookNotLentToReader,
				command.OccurredAt,
			),
		)
	}

	return core.SuccessDecision(
		core.BuildBookCopyReturnedFromReader(
			command.BookID,
			command.ReaderID,
			command.OccurredAt,
		),
	)
}

// project builds the current state by replaying all events from the history.
func project(history core.DomainEvents, bookID string, readerID string) state {
	s := state{
		bookIsNotInCirculation:       true, // Default to "not in circulation"
		bookWasNeverLentToThisReader: true, // Default to "never lent to this reader"
		bookIsNotLentToThisReader:    true, // Default to "not lent to this reader"
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
			if e.BookID == bookID && e.ReaderID == readerID {
				s.bookWasNeverLentToThisReader = false
				s.bookIsNotLentToThisReader = false
			}

		case core.BookCopyReturnedByReader:
			if e.BookID == bookID && e.ReaderID == readerID {
				s.bookIsNotLentToThisReader = true
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
		).
		AndAnyPredicateOf(
			eventstore.P("BookID", bookID.String()),
			eventstore.P("ReaderID", readerID.String()),
		).
		Finalize()
}
