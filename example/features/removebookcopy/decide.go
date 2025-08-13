package removebookcopy

import (
	"github.com/google/uuid"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

const (
	failureReasonBookNeverAddedToCirculation = "book was never added to circulation"
	failureReasonBookIsCurrentlyLent         = "book is currently lent"
)

// state represents the current state projected from the event history.
type state struct {
	bookIsNotInCirculation         bool
	bookWasNeverAddedToCirculation bool
	bookIsCurrentlyLent            bool
}

// Decide implements the business logic to determine whether a book copy should be removed from circulation.
// This is a pure function with no side effects - it takes the current domain events and a command
// and returns the events that should be appended based on the business rules.
//
// Business Rules:
//
//	GIVEN: A book copy with BookID
//	WHEN: RemoveBookCopyFromCirculation command is received
//	THEN: BookCopyRemovedFromCirculation event is generated
//	ERROR: "book was never added to circulation" if a book was never added to circulation
//	ERROR: "book is currently lent" if a book is currently lent to a reader
//	IDEMPOTENCY: If book already removed from circulation, no event generated (no-op)
func Decide(history core.DomainEvents, command Command) core.DecisionResult {
	s := project(history, command.BookID.String())

	if s.bookIsNotInCirculation {
		return core.IdempotentDecision() // idempotency - the book was already removed, so no new event
	}

	if s.bookWasNeverAddedToCirculation {
		return core.ErrorDecision(
			core.BuildRemovingBookFromCirculationFailed(
				command.BookID,
				failureReasonBookNeverAddedToCirculation,
				command.OccurredAt,
			),
		)
	}

	if s.bookIsCurrentlyLent {
		return core.ErrorDecision(
			core.BuildRemovingBookFromCirculationFailed(
				command.BookID,
				failureReasonBookIsCurrentlyLent,
				command.OccurredAt,
			),
		)
	}

	return core.SuccessDecision(
		core.BuildBookCopyRemovedFromCirculation(
			command.BookID,
			command.OccurredAt,
		),
	)
}

// project builds the current state by replaying all events from the history.
func project(history core.DomainEvents, bookID string) state {
	s := state{
		bookIsNotInCirculation:         true, // Default to "not in circulation"
		bookWasNeverAddedToCirculation: true, // Default to "never added to circulation"
	}

	for _, event := range history {
		switch e := event.(type) {
		case core.BookCopyAddedToCirculation:
			if e.BookID == bookID {
				s.bookIsNotInCirculation = false
				s.bookWasNeverAddedToCirculation = false
			}

		case core.BookCopyRemovedFromCirculation:
			if e.BookID == bookID {
				s.bookIsNotInCirculation = true
			}

		case core.BookCopyLentToReader:
			if e.BookID == bookID {
				s.bookIsCurrentlyLent = true
			}

		case core.BookCopyReturnedByReader:
			if e.BookID == bookID {
				s.bookIsCurrentlyLent = false
			}
		}
	}

	return s
}

// BuildEventFilter creates the filter for querying all events
// related to the specified book which are relevant for this feature/use-case.
func BuildEventFilter(bookID uuid.UUID) eventstore.Filter {
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
		).
		Finalize()
}
