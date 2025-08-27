package returnbookcopyfromreader_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/returnbookcopyfromreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

func Test_Decide_Success_WhenBookLentToReader(t *testing.T) {
	// arrange
	bookID := uuid.New()
	readerID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenBookAddedToCirculation(t, bookID, now.Add(-3*time.Hour)),
		givenReaderRegistered(t, readerID, now.Add(-2*time.Hour)),
		givenBookLentToReader(t, bookID, readerID, now.Add(-1*time.Hour)),
	}

	command := returnbookcopyfromreader.BuildCommand(bookID, readerID, now)

	// act
	result := returnbookcopyfromreader.Decide(events, command)

	// assert
	assertSuccessDecision(t, result, bookID, readerID)
}

func Test_Decide_Idempotent_WhenBookAlreadyReturned(t *testing.T) {
	// arrange
	bookID := uuid.New()
	readerID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenBookAddedToCirculation(t, bookID, now.Add(-4*time.Hour)),
		givenReaderRegistered(t, readerID, now.Add(-3*time.Hour)),
		givenBookLentToReader(t, bookID, readerID, now.Add(-2*time.Hour)),
		givenBookReturnedByReader(t, bookID, readerID, now.Add(-1*time.Hour)),
	}

	command := returnbookcopyfromreader.BuildCommand(bookID, readerID, now)

	// act
	result := returnbookcopyfromreader.Decide(events, command)

	// assert
	assertIdempotentDecision(t, result)
}

//nolint:funlen
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
			name: "reader was never registered but book is lent",
			events: []core.DomainEvent{
				givenBookAddedToCirculation(t, bookID, now.Add(-3*time.Hour)),
				// No reader registration, but somehow book was lent (orphaned data scenario)
				givenBookLentToReader(t, bookID, readerID, now.Add(-1*time.Hour)),
			},
			expectedReason: "reader was never registered",
		},
		{
			name: "book was never added to circulation but is lent",
			events: []core.DomainEvent{
				givenReaderRegistered(t, readerID, now.Add(-2*time.Hour)),
				// No book added to circulation, but somehow book was lent (orphaned data)
				givenBookLentToReader(t, bookID, readerID, now.Add(-1*time.Hour)),
			},
			expectedReason: "book was never added to circulation",
		},
		{
			name: "reader was never registered and book never lent",
			events: []core.DomainEvent{
				givenBookAddedToCirculation(t, bookID, now.Add(-2*time.Hour)),
				// No reader registration, no lending
			},
			expectedReason: "reader was never registered",
		},
		{
			name: "book was never added to circulation (no lending)",
			events: []core.DomainEvent{
				givenReaderRegistered(t, readerID, now.Add(-2*time.Hour)),
				// No book added to circulation, no lending
			},
			expectedReason: "book was never added to circulation",
		},
		{
			name: "book was never lent to this reader",
			events: []core.DomainEvent{
				givenBookAddedToCirculation(t, bookID, now.Add(-3*time.Hour)),
				givenReaderRegistered(t, readerID, now.Add(-2*time.Hour)),
				// Book added and reader registered, but book never lent to this reader
			},
			expectedReason: "book is not lent to this reader",
		},
		{
			name: "book was lent to another reader not this one",
			events: []core.DomainEvent{
				givenBookAddedToCirculation(t, bookID, now.Add(-4*time.Hour)),
				givenReaderRegistered(t, readerID, now.Add(-3*time.Hour)),
				givenBookLentToReader(t, bookID, uuid.New(), now.Add(-2*time.Hour)), // lent to different reader
			},
			expectedReason: "book is not lent to this reader",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			command := returnbookcopyfromreader.BuildCommand(bookID, readerID, now)

			// act
			result := returnbookcopyfromreader.Decide(tc.events, command)

			// assert
			assertErrorDecision(t, result, tc.expectedReason)
		})
	}
}

func Test_Decide_ComplexState_BookLentReturnedLentAgain(t *testing.T) {
	// arrange
	bookID := uuid.New()
	readerID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenBookAddedToCirculation(t, bookID, now.Add(-6*time.Hour)),
		givenReaderRegistered(t, readerID, now.Add(-5*time.Hour)),
		givenBookLentToReader(t, bookID, readerID, now.Add(-4*time.Hour)),
		givenBookReturnedByReader(t, bookID, readerID, now.Add(-3*time.Hour)),
		givenBookLentToReader(t, bookID, readerID, now.Add(-2*time.Hour)), // lent again
	}

	command := returnbookcopyfromreader.BuildCommand(bookID, readerID, now)

	// act
	result := returnbookcopyfromreader.Decide(events, command)

	// assert - book is currently lent to reader, should succeed
	assertSuccessDecision(t, result, bookID, readerID)
}

func Test_Decide_ComplexState_ReaderBorrowsMultipleBooks(t *testing.T) {
	// arrange
	targetBookID := uuid.New()
	otherBookID := uuid.New()
	readerID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenBookAddedToCirculation(t, targetBookID, now.Add(-5*time.Hour)),
		givenBookAddedToCirculation(t, otherBookID, now.Add(-5*time.Hour)),
		givenReaderRegistered(t, readerID, now.Add(-4*time.Hour)),
		givenBookLentToReader(t, targetBookID, readerID, now.Add(-3*time.Hour)),
		givenBookLentToReader(t, otherBookID, readerID, now.Add(-2*time.Hour)),
	}

	command := returnbookcopyfromreader.BuildCommand(targetBookID, readerID, now)

	// act
	result := returnbookcopyfromreader.Decide(events, command)

	// assert - target book is lent to reader, should succeed (other book doesn't matter)
	assertSuccessDecision(t, result, targetBookID, readerID)
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

func assertSuccessDecision(t *testing.T, result core.DecisionResult, bookID, readerID uuid.UUID) {
	t.Helper()
	assert.Equal(t, "success", result.Outcome, "Expected success decision")
	assert.NotNil(t, result.Event, "Expected event to be generated")
	assert.NoError(t, result.HasError(), "Expected no error for success decision")

	// Verify the generated event
	returnedEvent, ok := result.Event.(core.BookCopyReturnedByReader)
	assert.True(t, ok, "Expected BookCopyReturnedByReader event")
	assert.Equal(t, bookID.String(), returnedEvent.BookID, "Event should have correct BookID")
	assert.Equal(t, readerID.String(), returnedEvent.ReaderID, "Event should have correct ReaderID")
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
	failureEvent, ok := result.Event.(core.ReturningBookFromReaderFailed)
	assert.True(t, ok, "Expected ReturningBookFromReaderFailed event")
	assert.Contains(t, failureEvent.FailureInfo, expectedReason, "Failure event should contain expected reason")
}
