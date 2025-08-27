package lendbookcopytoreader_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/lendbookcopytoreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

func Test_Decide_Success_WhenAllPreconditionsMet(t *testing.T) {
	// arrange
	bookID := uuid.New()
	readerID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenBookAddedToCirculation(t, bookID, now.Add(-3*time.Hour)),
		givenReaderRegistered(t, readerID, now.Add(-2*time.Hour)),
	}

	command := lendbookcopytoreader.BuildCommand(bookID, readerID, now)

	// act
	result := lendbookcopytoreader.Decide(events, command)

	// assert
	assertSuccessDecision(t, result, bookID, readerID)
}

func Test_Decide_Success_WhenReaderHasNineBooks_BorrowingTenth(t *testing.T) {
	// arrange
	bookID := uuid.New()
	readerID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenBookAddedToCirculation(t, bookID, now.Add(-10*time.Hour)),
		givenReaderRegistered(t, readerID, now.Add(-9*time.Hour)),
	}

	// Add 9 other books lent to reader
	for i := 0; i < 9; i++ {
		otherBookID := uuid.New()
		events = append(events,
			givenBookAddedToCirculation(t, otherBookID, now.Add(-8*time.Hour+time.Duration(i)*time.Minute)),
			givenBookLentToReader(t, otherBookID, readerID, now.Add(-7*time.Hour+time.Duration(i)*time.Minute)),
		)
	}

	command := lendbookcopytoreader.BuildCommand(bookID, readerID, now)

	// act
	result := lendbookcopytoreader.Decide(events, command)

	// assert
	assertSuccessDecision(t, result, bookID, readerID)
}

func Test_Decide_Success_AfterBookReturnedCanBorrowAgain(t *testing.T) {
	// arrange
	bookID := uuid.New()
	readerID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenBookAddedToCirculation(t, bookID, now.Add(-5*time.Hour)),
		givenReaderRegistered(t, readerID, now.Add(-4*time.Hour)),
		givenBookLentToReader(t, bookID, readerID, now.Add(-3*time.Hour)),
		givenBookReturnedByReader(t, bookID, readerID, now.Add(-1*time.Hour)),
	}

	command := lendbookcopytoreader.BuildCommand(bookID, readerID, now)

	// act
	result := lendbookcopytoreader.Decide(events, command)

	// assert - this should be a success, NOT idempotent
	assertSuccessDecision(t, result, bookID, readerID)
}

func Test_Decide_Idempotent_WhenBookCurrentlyLentToSameReader(t *testing.T) {
	// arrange
	bookID := uuid.New()
	readerID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenBookAddedToCirculation(t, bookID, now.Add(-3*time.Hour)),
		givenReaderRegistered(t, readerID, now.Add(-2*time.Hour)),
		givenBookLentToReader(t, bookID, readerID, now.Add(-1*time.Hour)),
	}

	command := lendbookcopytoreader.BuildCommand(bookID, readerID, now)

	// act
	result := lendbookcopytoreader.Decide(events, command)

	// assert
	assertIdempotentDecision(t, result)
}

//nolint:funlen
func Test_Decide_BusinessErrors(t *testing.T) {
	bookID := uuid.New()
	readerID := uuid.New()
	otherReaderID := uuid.New()
	now := time.Now()

	testCases := []struct {
		name           string
		events         []core.DomainEvent
		expectedReason string
	}{
		{
			name: "book not in circulation - never added",
			events: []core.DomainEvent{
				givenReaderRegistered(t, readerID, now),
			},
			expectedReason: "book is not in circulation",
		},
		{
			name: "book removed from circulation",
			events: []core.DomainEvent{
				givenBookAddedToCirculation(t, bookID, now.Add(-2*time.Hour)),
				givenBookRemovedFromCirculation(t, bookID, now.Add(-1*time.Hour)),
				givenReaderRegistered(t, readerID, now),
			},
			expectedReason: "book is not in circulation",
		},
		{
			name: "book already lent to another reader",
			events: []core.DomainEvent{
				givenBookAddedToCirculation(t, bookID, now.Add(-3*time.Hour)),
				givenReaderRegistered(t, readerID, now.Add(-2*time.Hour)),
				givenReaderRegistered(t, otherReaderID, now.Add(-2*time.Hour)),
				givenBookLentToReader(t, bookID, otherReaderID, now.Add(-1*time.Hour)),
			},
			expectedReason: "book is already lent",
		},
		{
			name: "reader has maximum books (10)",
			events: func() []core.DomainEvent {
				events := []core.DomainEvent{
					givenBookAddedToCirculation(t, bookID, now.Add(-12*time.Hour)),
					givenReaderRegistered(t, readerID, now.Add(-11*time.Hour)),
				}
				// Add 10 books already lent to a reader
				for i := 0; i < 10; i++ {
					otherBookID := uuid.New()
					events = append(events,
						givenBookAddedToCirculation(t, otherBookID, now.Add(-10*time.Hour+time.Duration(i)*time.Minute)),
						givenBookLentToReader(t, otherBookID, readerID, now.Add(-9*time.Hour+time.Duration(i)*time.Minute)),
					)
				}
				return events
			}(),
			expectedReason: "reader has too many books",
		},
		{
			name: "reader not registered - never registered",
			events: []core.DomainEvent{
				givenBookAddedToCirculation(t, bookID, now),
			},
			expectedReason: "reader is not currently registered",
		},
		{
			name: "reader contract cancelled",
			events: []core.DomainEvent{
				givenBookAddedToCirculation(t, bookID, now.Add(-3*time.Hour)),
				givenReaderRegistered(t, readerID, now.Add(-2*time.Hour)),
				givenReaderContractCanceled(t, readerID, now.Add(-1*time.Hour)),
			},
			expectedReason: "reader is not currently registered",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			command := lendbookcopytoreader.BuildCommand(bookID, readerID, now)

			// act
			result := lendbookcopytoreader.Decide(tc.events, command)

			// assert
			assertErrorDecision(t, result, tc.expectedReason)
		})
	}
}

func Test_Decide_ComplexState_BookAddedRemovedAddedAgain(t *testing.T) {
	// arrange
	bookID := uuid.New()
	readerID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenBookAddedToCirculation(t, bookID, now.Add(-5*time.Hour)),     // added
		givenBookRemovedFromCirculation(t, bookID, now.Add(-4*time.Hour)), // removed
		givenBookAddedToCirculation(t, bookID, now.Add(-3*time.Hour)),     // added again
		givenReaderRegistered(t, readerID, now.Add(-2*time.Hour)),
	}

	command := lendbookcopytoreader.BuildCommand(bookID, readerID, now)

	// act
	result := lendbookcopytoreader.Decide(events, command)

	// assert - book should be considered in circulation (last state)
	assertSuccessDecision(t, result, bookID, readerID)
}

func Test_Decide_ComplexState_AccurateBookCountWithMultipleLendings(t *testing.T) {
	// arrange
	bookID := uuid.New()
	readerID := uuid.New()
	now := time.Now()

	// Create a scenario: reader borrows 3 books, returns 1, so has 2 currently
	book1ID := uuid.New()
	book2ID := uuid.New()
	book3ID := uuid.New()

	events := []core.DomainEvent{
		givenBookAddedToCirculation(t, bookID, now.Add(-10*time.Hour)),
		givenReaderRegistered(t, readerID, now.Add(-9*time.Hour)),

		// Lend 3 books
		givenBookAddedToCirculation(t, book1ID, now.Add(-8*time.Hour)),
		givenBookLentToReader(t, book1ID, readerID, now.Add(-7*time.Hour)),

		givenBookAddedToCirculation(t, book2ID, now.Add(-6*time.Hour)),
		givenBookLentToReader(t, book2ID, readerID, now.Add(-5*time.Hour)),

		givenBookAddedToCirculation(t, book3ID, now.Add(-4*time.Hour)),
		givenBookLentToReader(t, book3ID, readerID, now.Add(-3*time.Hour)),

		// Return 1 book
		givenBookReturnedByReader(t, book2ID, readerID, now.Add(-2*time.Hour)),
	}

	command := lendbookcopytoreader.BuildCommand(bookID, readerID, now)

	// act
	result := lendbookcopytoreader.Decide(events, command)

	// assert - reader has 2 books, should be able to lend another (< limit 10)
	assertSuccessDecision(t, result, bookID, readerID)
}

// Test helper functions with t.Helper() for better error reporting

func givenBookAddedToCirculation(t *testing.T, bookID uuid.UUID, at time.Time) core.DomainEvent {
	t.Helper()
	return core.BuildBookCopyAddedToCirculation(
		bookID,
		"978-1-098-10013-1",
		"Test Book Title",
		"Test Author",
		"First Edition",
		"Test Publisher",
		2021,
		core.ToOccurredAt(at),
	)
}

func givenBookRemovedFromCirculation(t *testing.T, bookID uuid.UUID, at time.Time) core.DomainEvent {
	t.Helper()
	return core.BuildBookCopyRemovedFromCirculation(
		bookID,
		at,
	)
}

func givenReaderRegistered(t *testing.T, readerID uuid.UUID, at time.Time) core.DomainEvent {
	t.Helper()
	return core.BuildReaderRegistered(
		readerID,
		"Test Reader Name",
		core.ToOccurredAt(at),
	)
}

func givenReaderContractCanceled(t *testing.T, readerID uuid.UUID, at time.Time) core.DomainEvent {
	t.Helper()
	return core.BuildReaderContractCanceled(
		readerID,
		at,
	)
}

func givenBookLentToReader(t *testing.T, bookID, readerID uuid.UUID, at time.Time) core.DomainEvent {
	t.Helper()
	return core.BuildBookCopyLentToReader(
		bookID,
		readerID,
		core.ToOccurredAt(at),
	)
}

func givenBookReturnedByReader(t *testing.T, bookID, readerID uuid.UUID, at time.Time) core.DomainEvent {
	t.Helper()
	return core.BuildBookCopyReturnedFromReader(
		bookID,
		readerID,
		at,
	)
}

func assertSuccessDecision(t *testing.T, result core.DecisionResult, bookID, readerID uuid.UUID) {
	t.Helper()
	assert.Equal(t, "success", result.Outcome, "Expected success decision")
	assert.NotNil(t, result.Event, "Expected event to be generated")
	assert.NoError(t, result.HasError(), "Expected no error for success decision")

	// Verify the generated event
	lentEvent, ok := result.Event.(core.BookCopyLentToReader)
	assert.True(t, ok, "Expected BookCopyLentToReader event")
	assert.Equal(t, bookID.String(), lentEvent.BookID, "Event should have correct BookID")
	assert.Equal(t, readerID.String(), lentEvent.ReaderID, "Event should have correct ReaderID")
}

func assertIdempotentDecision(t *testing.T, result core.DecisionResult) {
	t.Helper()
	assert.Equal(t, "idempotent", result.Outcome, "Expected idempotent decision")
	assert.Nil(t, result.Event, "Expected no event for idempotent decision")
	assert.NoError(t, result.HasError(), "Expected no error for idempotent decision")
}

func assertErrorDecision(t *testing.T, result core.DecisionResult, expectedReason string) {
	t.Helper()
	assert.Equal(t, "error", result.Outcome, "Expected error decision")
	assert.NotNil(t, result.Event, "Expected error event to be generated")
	assert.Error(t, result.HasError(), "Expected error for error decision")
	assert.ErrorContains(t, result.HasError(), expectedReason, "Error message should contain expected reason")

	// Verify the generated error event
	failureEvent, ok := result.Event.(core.LendingBookToReaderFailed)
	assert.True(t, ok, "Expected LendingBookToReaderFailed event")
	assert.Contains(t, failureEvent.FailureInfo, expectedReason, "Failure event should contain expected reason")
}
