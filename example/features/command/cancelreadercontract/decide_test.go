package cancelreadercontract_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/cancelreadercontract"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

func Test_Decide_Success_WhenReaderRegisteredAndNoOutstandingLoans(t *testing.T) {
	// arrange
	readerID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenReaderRegistered(t, readerID, now.Add(-1*time.Hour)),
	}

	command := cancelreadercontract.BuildCommand(readerID, now)

	// act
	result := cancelreadercontract.Decide(events, command)

	// assert
	assertSuccessDecision(t, result, readerID)
}

func Test_Decide_Success_AfterAllBooksReturned(t *testing.T) {
	// arrange
	readerID := uuid.New()
	bookID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenReaderRegistered(t, readerID, now.Add(-4*time.Hour)),
		givenBookAddedToCirculation(t, bookID, now.Add(-3*time.Hour)),
		givenBookLentToReader(t, bookID, readerID, now.Add(-2*time.Hour)),
		givenBookReturnedByReader(t, bookID, readerID, now.Add(-1*time.Hour)),
	}

	command := cancelreadercontract.BuildCommand(readerID, now)

	// act
	result := cancelreadercontract.Decide(events, command)

	// assert - all books returned, so can cancel contract
	assertSuccessDecision(t, result, readerID)
}

// Note: Test_Decide_Idempotent_WhenReaderNeverRegistered was removed because
// "reader never registered" is now an error case, covered in Test_Decide_BusinessErrors

func Test_Decide_Idempotent_WhenReaderContractAlreadyCanceled(t *testing.T) {
	// arrange
	readerID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenReaderRegistered(t, readerID, now.Add(-2*time.Hour)),
		givenReaderContractCanceled(t, readerID, now.Add(-1*time.Hour)),
	}

	command := cancelreadercontract.BuildCommand(readerID, now)

	// act
	result := cancelreadercontract.Decide(events, command)

	// assert
	assertIdempotentDecision(t, result)
}

func Test_Decide_BusinessErrors(t *testing.T) {
	readerID := uuid.New()
	book1ID := uuid.New()
	book2ID := uuid.New()
	now := time.Now()

	testCases := []struct {
		name           string
		events         []core.DomainEvent
		expectedReason string
	}{
		{
			name:   "reader was never registered",
			events: []core.DomainEvent{
				// No reader registration events
			},
			expectedReason: "reader was never registered",
		},
		{
			name: "reader has one outstanding book loan",
			events: []core.DomainEvent{
				givenReaderRegistered(t, readerID, now.Add(-3*time.Hour)),
				givenBookAddedToCirculation(t, book1ID, now.Add(-2*time.Hour)),
				givenBookLentToReader(t, book1ID, readerID, now.Add(-1*time.Hour)),
			},
			expectedReason: "reader has outstanding book loans",
		},
		{
			name: "reader has multiple outstanding book loans",
			events: []core.DomainEvent{
				givenReaderRegistered(t, readerID, now.Add(-5*time.Hour)),
				givenBookAddedToCirculation(t, book1ID, now.Add(-4*time.Hour)),
				givenBookAddedToCirculation(t, book2ID, now.Add(-4*time.Hour)),
				givenBookLentToReader(t, book1ID, readerID, now.Add(-3*time.Hour)),
				givenBookLentToReader(t, book2ID, readerID, now.Add(-2*time.Hour)),
			},
			expectedReason: "reader has outstanding book loans",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			command := cancelreadercontract.BuildCommand(readerID, now)

			// act
			result := cancelreadercontract.Decide(tc.events, command)

			// assert
			assertErrorDecision(t, result, tc.expectedReason)
		})
	}
}

func Test_Decide_ComplexState_ReaderRegisteredCanceledRegisteredAgain(t *testing.T) {
	// arrange
	readerID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenReaderRegistered(t, readerID, now.Add(-5*time.Hour)),       // registered
		givenReaderContractCanceled(t, readerID, now.Add(-4*time.Hour)), // canceled
		givenReaderRegistered(t, readerID, now.Add(-3*time.Hour)),       // registered again
	}

	command := cancelreadercontract.BuildCommand(readerID, now)

	// act
	result := cancelreadercontract.Decide(events, command)

	// assert - reader is currently registered (last state), should succeed
	assertSuccessDecision(t, result, readerID)
}

func Test_Decide_ComplexState_BookLentReturnedLentAgainToSameReader(t *testing.T) {
	// arrange
	readerID := uuid.New()
	bookID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenReaderRegistered(t, readerID, now.Add(-6*time.Hour)),
		givenBookAddedToCirculation(t, bookID, now.Add(-5*time.Hour)),
		givenBookLentToReader(t, bookID, readerID, now.Add(-4*time.Hour)),
		givenBookReturnedByReader(t, bookID, readerID, now.Add(-3*time.Hour)),
		givenBookLentToReader(t, bookID, readerID, now.Add(-2*time.Hour)), // lent again
	}

	command := cancelreadercontract.BuildCommand(readerID, now)

	// act
	result := cancelreadercontract.Decide(events, command)

	// assert - book is currently lent (outstanding loan), should fail
	assertErrorDecision(t, result, "reader has outstanding book loans")
}

func Test_Decide_ComplexState_MultipleReaders_OnlyTargetReaderMatters(t *testing.T) {
	// arrange
	targetReaderID := uuid.New()
	otherReaderID := uuid.New()
	bookID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		// Other reader events should not affect our target reader
		givenReaderRegistered(t, otherReaderID, now.Add(-4*time.Hour)),
		givenBookAddedToCirculation(t, bookID, now.Add(-3*time.Hour)),
		givenBookLentToReader(t, bookID, otherReaderID, now.Add(-2*time.Hour)),
		// Target reader is registered with no loans
		givenReaderRegistered(t, targetReaderID, now.Add(-1*time.Hour)),
	}

	command := cancelreadercontract.BuildCommand(targetReaderID, now)

	// act
	result := cancelreadercontract.Decide(events, command)

	// assert - target reader has no outstanding loans, should succeed
	assertSuccessDecision(t, result, targetReaderID)
}

func Test_Decide_ComplexState_ReaderBorrowsMultipleBooksReturnsPartially(t *testing.T) {
	// arrange
	readerID := uuid.New()
	book1ID := uuid.New()
	book2ID := uuid.New()
	book3ID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenReaderRegistered(t, readerID, now.Add(-7*time.Hour)),
		givenBookAddedToCirculation(t, book1ID, now.Add(-6*time.Hour)),
		givenBookAddedToCirculation(t, book2ID, now.Add(-6*time.Hour)),
		givenBookAddedToCirculation(t, book3ID, now.Add(-6*time.Hour)),
		givenBookLentToReader(t, book1ID, readerID, now.Add(-5*time.Hour)),
		givenBookLentToReader(t, book2ID, readerID, now.Add(-4*time.Hour)),
		givenBookLentToReader(t, book3ID, readerID, now.Add(-3*time.Hour)),
		givenBookReturnedByReader(t, book1ID, readerID, now.Add(-2*time.Hour)), // only returned book1
		givenBookReturnedByReader(t, book2ID, readerID, now.Add(-1*time.Hour)), // only returned book2
		// book3 still outstanding
	}

	command := cancelreadercontract.BuildCommand(readerID, now)

	// act
	result := cancelreadercontract.Decide(events, command)

	// assert - book3 still outstanding, should fail
	assertErrorDecision(t, result, "reader has outstanding book loans")
}

// Test helper functions with t.Helper() for better error reporting

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

func assertSuccessDecision(t *testing.T, result core.DecisionResult, readerID uuid.UUID) {
	t.Helper()
	assert.Equal(t, "success", result.Outcome, "Expected success decision")
	assert.NotNil(t, result.Event, "Expected event to be generated")
	assert.NoError(t, result.HasError(), "Expected no error for success decision")

	// Verify the generated event
	canceledEvent, ok := result.Event.(core.ReaderContractCanceled)
	assert.True(t, ok, "Expected ReaderContractCanceled event")
	assert.Equal(t, readerID.String(), canceledEvent.ReaderID, "Event should have correct ReaderID")
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
	failureEvent, ok := result.Event.(core.CancelingReaderContractFailed)
	assert.True(t, ok, "Expected CancelingReaderContractFailed event")
	assert.Contains(t, failureEvent.FailureInfo, expectedReason, "Failure event should contain expected reason")
}
