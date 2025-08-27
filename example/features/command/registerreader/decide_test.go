package registerreader_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/command/registerreader"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

func Test_Decide_Success_WhenReaderNotRegistered(t *testing.T) {
	// arrange
	readerID := uuid.New()
	now := time.Now()

	var events []core.DomainEvent // No prior events - reader never registered

	command := registerreader.BuildCommand(
		readerID,
		"John Doe",
		now,
	)

	// act
	result := registerreader.Decide(events, command)

	// assert
	assertSuccessDecision(t, result, readerID)
}

func Test_Decide_Success_AfterReaderContractCanceled(t *testing.T) {
	// arrange
	readerID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenReaderRegistered(t, readerID, now.Add(-2*time.Hour)),
		givenReaderContractCanceled(t, readerID, now.Add(-1*time.Hour)),
	}

	command := registerreader.BuildCommand(
		readerID,
		"John Doe",
		now,
	)

	// act
	result := registerreader.Decide(events, command)

	// assert - reader contract was canceled, so can be registered again
	assertSuccessDecision(t, result, readerID)
}

func Test_Decide_Idempotent_WhenReaderAlreadyRegistered(t *testing.T) {
	// arrange
	readerID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenReaderRegistered(t, readerID, now.Add(-1*time.Hour)),
	}

	command := registerreader.BuildCommand(
		readerID,
		"John Doe",
		now,
	)

	// act
	result := registerreader.Decide(events, command)

	// assert
	assertIdempotentDecision(t, result)
}

func Test_Decide_ComplexState_ReaderRegisteredCanceledMultipleTimes(t *testing.T) {
	// arrange
	readerID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		givenReaderRegistered(t, readerID, now.Add(-5*time.Hour)),       // registered
		givenReaderContractCanceled(t, readerID, now.Add(-4*time.Hour)), // canceled
		givenReaderRegistered(t, readerID, now.Add(-3*time.Hour)),       // registered again
	}

	command := registerreader.BuildCommand(
		readerID,
		"John Doe",
		now,
	)

	// act
	result := registerreader.Decide(events, command)

	// assert - reader is currently registered (last state), so should be idempotent
	assertIdempotentDecision(t, result)
}

func Test_Decide_ComplexState_MultipleReaders_OnlyTargetReaderMatters(t *testing.T) {
	// arrange
	targetReaderID := uuid.New()
	otherReaderID := uuid.New()
	now := time.Now()

	events := []core.DomainEvent{
		// Other reader events should not affect our target reader
		givenReaderRegistered(t, otherReaderID, now.Add(-3*time.Hour)),
		givenReaderContractCanceled(t, otherReaderID, now.Add(-2*time.Hour)),
		// Target reader is not registered
	}

	command := registerreader.BuildCommand(
		targetReaderID,
		"John Doe",
		now,
	)

	// act
	result := registerreader.Decide(events, command)

	// assert - target reader not registered, should succeed
	assertSuccessDecision(t, result, targetReaderID)
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

func assertSuccessDecision(t *testing.T, result core.DecisionResult, readerID uuid.UUID) {
	t.Helper()
	assert.Equal(t, "success", result.Outcome, "Expected success decision")
	assert.NotNil(t, result.Event, "Expected event to be generated")
	assert.NoError(t, result.HasError(), "Expected no error for success decision")

	// Verify the generated event
	registeredEvent, ok := result.Event.(core.ReaderRegistered)
	assert.True(t, ok, "Expected ReaderRegistered event")
	assert.Equal(t, readerID.String(), registeredEvent.ReaderID, "Event should have correct ReaderID")
}

func assertIdempotentDecision(t *testing.T, result core.DecisionResult) {
	t.Helper()
	assert.Equal(t, "idempotent", result.Outcome, "Expected idempotent decision")
	assert.Nil(t, result.Event, "Expected no event for idempotent decision")
	assert.NoError(t, result.HasError(), "Expected no error for idempotent decision")
}
