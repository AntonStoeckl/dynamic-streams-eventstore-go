package removebookcopy_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/removebookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

func Test_Decide_Success_WhenBookInCirculationAndNotLent(t *testing.T) {
	// arrange
	bookID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenBookAddedToCirculation(t, bookID, now.Add(-2*time.Hour)),
	}

	command := removebookcopy.BuildCommand(bookID, now)

	// act
	result := removebookcopy.Decide(events, command)

	// assert
	assertSuccessDecision(t, result, bookID)
}

func Test_Decide_Idempotent_WhenBookAlreadyRemovedFromCirculation(t *testing.T) {
	// arrange
	bookID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenBookAddedToCirculation(t, bookID, now.Add(-3*time.Hour)),
		givenBookRemovedFromCirculation(t, bookID, now.Add(-1*time.Hour)),
	}

	command := removebookcopy.BuildCommand(bookID, now)

	// act
	result := removebookcopy.Decide(events, command)

	// assert
	assertIdempotentDecision(t, result)
}

func Test_Decide_BusinessErrors(t *testing.T) {
	bookID := uuid.New()
	readerID := uuid.New()
	now := time.Now()

	testCases := []struct {
		name           string
		events         []core.DomainEvent
		expectedReason string
	}{
		{
			name:   "book was never added to circulation",
			events: []core.DomainEvent{
				// No book added to circulation
			},
			expectedReason: "book was never added to circulation",
		},
		{
			name: "book is currently lent to a reader",
			events: []core.DomainEvent{
				givenBookAddedToCirculation(t, bookID, now.Add(-3*time.Hour)),
				givenReaderRegistered(t, readerID, now.Add(-2*time.Hour)),
				givenBookLentToReader(t, bookID, readerID, now.Add(-1*time.Hour)),
			},
			expectedReason: "book is currently lent",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			command := removebookcopy.BuildCommand(bookID, now)

			// act
			result := removebookcopy.Decide(tc.events, command)

			// assert
			assertErrorDecision(t, result, tc.expectedReason)
		})
	}
}

func Test_Decide_Success_AfterBookReturnedFromReader(t *testing.T) {
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

	command := removebookcopy.BuildCommand(bookID, now)

	// act
	result := removebookcopy.Decide(events, command)

	// assert - book was returned, so can be removed
	assertSuccessDecision(t, result, bookID)
}

func Test_Decide_ComplexState_BookAddedRemovedAddedAgain(t *testing.T) {
	// arrange
	bookID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenBookAddedToCirculation(t, bookID, now.Add(-5*time.Hour)),     // added
		givenBookRemovedFromCirculation(t, bookID, now.Add(-4*time.Hour)), // removed
		givenBookAddedToCirculation(t, bookID, now.Add(-3*time.Hour)),     // added again
	}

	command := removebookcopy.BuildCommand(bookID, now)

	// act
	result := removebookcopy.Decide(events, command)

	// assert - book is currently in circulation (last state), should succeed
	assertSuccessDecision(t, result, bookID)
}

func Test_Decide_ComplexState_MultipleBooks_OnlyTargetBookMatters(t *testing.T) {
	// arrange
	targetBookID := uuid.New()
	otherBookID := uuid.New()
	readerID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		// Other book events should not affect our target book
		givenBookAddedToCirculation(t, otherBookID, now.Add(-4*time.Hour)),
		givenReaderRegistered(t, readerID, now.Add(-3*time.Hour)),
		givenBookLentToReader(t, otherBookID, readerID, now.Add(-2*time.Hour)),
		// Target book is in circulation and not lent
		givenBookAddedToCirculation(t, targetBookID, now.Add(-1*time.Hour)),
	}

	command := removebookcopy.BuildCommand(targetBookID, now)

	// act
	result := removebookcopy.Decide(events, command)

	// assert - target book is in circulation and not lent, should succeed
	assertSuccessDecision(t, result, targetBookID)
}

func Test_Decide_ComplexState_BookLentAndReturnedMultipleTimes(t *testing.T) {
	// arrange
	bookID := uuid.New()
	reader1ID := uuid.New()
	reader2ID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenBookAddedToCirculation(t, bookID, now.Add(-8*time.Hour)),
		givenReaderRegistered(t, reader1ID, now.Add(-7*time.Hour)),
		givenReaderRegistered(t, reader2ID, now.Add(-7*time.Hour)),
		givenBookLentToReader(t, bookID, reader1ID, now.Add(-6*time.Hour)),
		givenBookReturnedByReader(t, bookID, reader1ID, now.Add(-5*time.Hour)),
		givenBookLentToReader(t, bookID, reader2ID, now.Add(-4*time.Hour)),
		givenBookReturnedByReader(t, bookID, reader2ID, now.Add(-2*time.Hour)),
	}

	command := removebookcopy.BuildCommand(bookID, now)

	// act
	result := removebookcopy.Decide(events, command)

	// assert - book is not currently lent (last return), should succeed
	assertSuccessDecision(t, result, bookID)
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

func assertSuccessDecision(t *testing.T, result core.DecisionResult, bookID uuid.UUID) {
	t.Helper()
	assert.Equal(t, "success", result.Outcome, "Expected success decision")
	assert.NotNil(t, result.Event, "Expected event to be generated")
	assert.NoError(t, result.HasError(), "Expected no error for success decision")

	// Verify the generated event
	removedEvent, ok := result.Event.(core.BookCopyRemovedFromCirculation)
	assert.True(t, ok, "Expected BookCopyRemovedFromCirculation event")
	assert.Equal(t, bookID.String(), removedEvent.BookID, "Event should have correct BookID")
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
	failureEvent, ok := result.Event.(core.RemovingBookFromCirculationFailed)
	assert.True(t, ok, "Expected RemovingBookFromCirculationFailed event")
	assert.Contains(t, failureEvent.FailureInfo, expectedReason, "Failure event should contain expected reason")
}
