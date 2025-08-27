package addbookcopy_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/addbookcopy"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

func Test_Decide_Success_WhenBookNotInCirculation(t *testing.T) {
	// arrange
	bookID := uuid.New()
	now := time.Now()

	var events []core.DomainEvent // No prior events - book never added

	command := addbookcopy.BuildCommand(
		bookID,
		"978-1-098-10013-1",
		"Learning Domain-Driven Design",
		"Vlad Khononov",
		"First Edition",
		"O'Reilly Media, Inc.",
		2021,
		now,
	)

	// act
	result := addbookcopy.Decide(events, command)

	// assert
	assertSuccessDecision(t, result, bookID)
}

func Test_Decide_Success_AfterBookRemovedFromCirculation(t *testing.T) {
	// arrange
	bookID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenBookAddedToCirculation(t, bookID, now.Add(-2*time.Hour)),
		givenBookRemovedFromCirculation(t, bookID, now.Add(-1*time.Hour)),
	}

	command := addbookcopy.BuildCommand(
		bookID,
		"978-1-098-10013-1",
		"Learning Domain-Driven Design",
		"Vlad Khononov",
		"First Edition",
		"O'Reilly Media, Inc.",
		2021,
		now,
	)

	// act
	result := addbookcopy.Decide(events, command)

	// assert - book was removed, so can be added again
	assertSuccessDecision(t, result, bookID)
}

func Test_Decide_Idempotent_WhenBookAlreadyInCirculation(t *testing.T) {
	// arrange
	bookID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenBookAddedToCirculation(t, bookID, now.Add(-1*time.Hour)),
	}

	command := addbookcopy.BuildCommand(
		bookID,
		"978-1-098-10013-1",
		"Learning Domain-Driven Design",
		"Vlad Khononov",
		"First Edition",
		"O'Reilly Media, Inc.",
		2021,
		now,
	)

	// act
	result := addbookcopy.Decide(events, command)

	// assert
	assertIdempotentDecision(t, result)
}

func Test_Decide_ComplexState_BookAddedRemovedMultipleTimes(t *testing.T) {
	// arrange
	bookID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenBookAddedToCirculation(t, bookID, now.Add(-5*time.Hour)),     // added
		givenBookRemovedFromCirculation(t, bookID, now.Add(-4*time.Hour)), // removed
		givenBookAddedToCirculation(t, bookID, now.Add(-3*time.Hour)),     // added again
	}

	command := addbookcopy.BuildCommand(
		bookID,
		"978-1-098-10013-1",
		"Learning Domain-Driven Design",
		"Vlad Khononov",
		"First Edition",
		"O'Reilly Media, Inc.",
		2021,
		now,
	)

	// act
	result := addbookcopy.Decide(events, command)

	// assert - book is currently in circulation (last state), so should be idempotent
	assertIdempotentDecision(t, result)
}

func Test_Decide_ComplexState_MultipleBooks_OnlyTargetBookMatters(t *testing.T) {
	// arrange
	targetBookID := uuid.New()
	otherBookID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		// Other book events should not affect our target book
		givenBookAddedToCirculation(t, otherBookID, now.Add(-3*time.Hour)),
		givenBookRemovedFromCirculation(t, otherBookID, now.Add(-2*time.Hour)),
		// Target book is not in circulation
	}

	command := addbookcopy.BuildCommand(
		targetBookID,
		"978-1-098-10013-1",
		"Learning Domain-Driven Design",
		"Vlad Khononov",
		"First Edition",
		"O'Reilly Media, Inc.",
		2021,
		now,
	)

	// act
	result := addbookcopy.Decide(events, command)

	// assert - target book not in circulation, should succeed
	assertSuccessDecision(t, result, targetBookID)
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

func assertSuccessDecision(t *testing.T, result core.DecisionResult, bookID uuid.UUID) {
	t.Helper()
	assert.Equal(t, "success", result.Outcome, "Expected success decision")
	assert.NotNil(t, result.Event, "Expected event to be generated")
	assert.NoError(t, result.HasError(), "Expected no error for success decision")

	// Verify the generated event
	addedEvent, ok := result.Event.(core.BookCopyAddedToCirculation)
	assert.True(t, ok, "Expected BookCopyAddedToCirculation event")
	assert.Equal(t, bookID.String(), addedEvent.BookID, "Event should have correct BookID")
}

func assertIdempotentDecision(t *testing.T, result core.DecisionResult) {
	t.Helper()
	assert.Equal(t, "idempotent", result.Outcome, "Expected idempotent decision")
	assert.Nil(t, result.Event, "Expected no event for idempotent decision")
	assert.NoError(t, result.HasError(), "Expected no error for idempotent decision")
}
