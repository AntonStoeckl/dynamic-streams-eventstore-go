package postgresengine_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/eventstore/estesthelpers"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/eventstore/fixtures"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/eventstore/shared"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/pgtesthelpers"
)

func Test_Append_When_NoEvent_MatchesTheQuery_BeforeAppend(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := estesthelpers.GivenUniqueID(t)
	filter := estesthelpers.FilterAllEventTypesForOneBook(bookID)
	maxSequenceNumberBeforeAppend := estesthelpers.QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter)

	// act
	fakeClock = fakeClock.Add(time.Second)
	err := es.Append(
		ctxWithTimeout,
		filter,
		maxSequenceNumberBeforeAppend,
		estesthelpers.ToStorable(t, estesthelpers.FixtureBookCopyAddedToCirculation(bookID, fakeClock)),
	)

	// assert
	assert.NoError(t, err)
}

func Test_Append_When_SomeEvents_MatchTheQuery_BeforeAppend(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := estesthelpers.GivenUniqueID(t)
	fakeClock = fakeClock.Add(time.Second)
	estesthelpers.GivenBookCopyAddedToCirculationWasAppended(t, ctxWithTimeout, es, bookID, fakeClock)
	filter := estesthelpers.FilterAllEventTypesForOneBook(bookID)
	maxSequenceNumberBeforeAppend := estesthelpers.QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter)

	// act
	fakeClock = fakeClock.Add(time.Second)
	err := es.Append(
		ctxWithTimeout,
		filter,
		maxSequenceNumberBeforeAppend,
		estesthelpers.ToStorable(t, estesthelpers.FixtureBookCopyRemovedFromCirculation(bookID, fakeClock)),
	)

	// assert
	assert.NoError(t, err)
}

func Test_Append_When_A_ConcurrencyConflict_ShouldHappen(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := estesthelpers.GivenUniqueID(t)
	readerID := estesthelpers.GivenUniqueID(t)
	fakeClock = fakeClock.Add(time.Second)
	estesthelpers.GivenBookCopyAddedToCirculationWasAppended(t, ctxWithTimeout, es, bookID, fakeClock)
	filter := estesthelpers.FilterAllEventTypesForOneBook(bookID)
	maxSequenceNumberBeforeAppend := estesthelpers.QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter)
	fakeClock = fakeClock.Add(time.Second)
	estesthelpers.GivenBookCopyLentToReaderWasAppended(t, ctxWithTimeout, es, bookID, readerID, fakeClock) // concurrent append

	// act
	fakeClock = fakeClock.Add(time.Second)
	err := es.Append(
		ctxWithTimeout,
		filter,
		maxSequenceNumberBeforeAppend,
		estesthelpers.ToStorable(t, estesthelpers.FixtureBookCopyRemovedFromCirculation(bookID, fakeClock)),
	)

	// assert
	assert.ErrorContains(t, err, eventstore.ErrConcurrencyConflict.Error())
}

func Test_AppendMultiple(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := estesthelpers.GivenUniqueID(t)
	readerID := estesthelpers.GivenUniqueID(t)
	fakeClock = fakeClock.Add(time.Second)
	estesthelpers.GivenBookCopyAddedToCirculationWasAppended(t, ctxWithTimeout, es, bookID, fakeClock)
	filter := estesthelpers.FilterAllEventTypesForOneBook(bookID)
	maxSequenceNumberBeforeAppend := estesthelpers.QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter)

	// act
	fakeClock = fakeClock.Add(time.Second)
	err := es.Append(
		ctxWithTimeout,
		filter,
		maxSequenceNumberBeforeAppend,
		estesthelpers.ToStorable(t, estesthelpers.FixtureBookCopyLentToReader(bookID, readerID, fakeClock)),
		estesthelpers.ToStorable(t, estesthelpers.FixtureBookCopyReturnedByReader(bookID, readerID, fakeClock)),
	)

	// assert
	assert.NoError(t, err)
	actualEvents, _, queryErr := es.Query(ctxWithTimeout, filter)
	assert.NoError(t, queryErr)
	assert.Len(t, actualEvents, 3, "there should be exactly 3 events") // 1 in arrange and 2 in act
}

func Test_AppendMultiple_When_A_ConcurrencyConflict_ShouldHappen(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := estesthelpers.GivenUniqueID(t)
	readerID := estesthelpers.GivenUniqueID(t)
	fakeClock = fakeClock.Add(time.Second)
	estesthelpers.GivenBookCopyAddedToCirculationWasAppended(t, ctxWithTimeout, es, bookID, fakeClock)
	filter := estesthelpers.FilterAllEventTypesForOneBook(bookID)
	maxSequenceNumberBeforeAppend := estesthelpers.QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter)
	fakeClock = fakeClock.Add(time.Second)
	estesthelpers.GivenBookCopyLentToReaderWasAppended(t, ctxWithTimeout, es, bookID, readerID, fakeClock) // concurrent append

	// act
	fakeClock = fakeClock.Add(time.Second)
	err := es.Append(
		ctxWithTimeout,
		filter,
		maxSequenceNumberBeforeAppend,
		estesthelpers.ToStorable(t, estesthelpers.FixtureBookCopyLentToReader(bookID, readerID, fakeClock)),
		estesthelpers.ToStorable(t, estesthelpers.FixtureBookCopyReturnedByReader(bookID, readerID, fakeClock)),
	)

	// assert
	assert.ErrorContains(t, err, eventstore.ErrConcurrencyConflict.Error())
}

// Test_Append_Concurrent is an integration test that requires complex setup and coordination of multiple goroutines.
// High statement count is acceptable for comprehensive concurrency testing.
//
//nolint:funlen
func Test_Append_Concurrent(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := estesthelpers.GivenUniqueID(t)
	readerID := estesthelpers.GivenUniqueID(t)

	successCountSingle := atomic.Int32{}
	successCountMultiple := atomic.Int32{}
	conflictCountSingle := atomic.Int32{}
	conflictCountMultiple := atomic.Int32{}
	eventCount := atomic.Int32{}

	numGoroutines := 10
	operationsPerGoroutine := 100
	var wg sync.WaitGroup

	// act
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)

		go func(_ int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				filter := estesthelpers.FilterAllEventTypesForOneBookOrReader(bookID, readerID)
				maxSeq := estesthelpers.QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter)

				// Randomly choose between appending single and multiple event(s)
				if rand.IntN(2)%2 == 0 { //nolint:gosec
					// Single event
					err := es.Append(
						ctxWithTimeout,
						filter,
						maxSeq,
						estesthelpers.ToStorable(t, estesthelpers.FixtureBookCopyLentToReader(bookID, readerID, fakeClock)),
					)
					switch {
					case err == nil:
						successCountSingle.Add(1)
						eventCount.Add(1)
					case errors.Is(err, eventstore.ErrConcurrencyConflict):
						conflictCountSingle.Add(1)
					default:
						assert.FailNow(t, "unexpected error")
					}
				} else {
					// Multiple events
					event1 := estesthelpers.ToStorable(t, estesthelpers.FixtureBookCopyLentToReader(bookID, readerID, fakeClock))
					event2 := estesthelpers.ToStorable(t, estesthelpers.FixtureBookCopyReturnedByReader(bookID, readerID, fakeClock))
					err := es.Append(
						ctxWithTimeout,
						filter,
						maxSeq,
						event1,
						event2,
					)
					switch {
					case err == nil:
						successCountMultiple.Add(1)
						eventCount.Add(2) // Count both events
					case errors.Is(err, eventstore.ErrConcurrencyConflict):
						conflictCountMultiple.Add(1)
					default:
						t.Errorf("unexpected error: %v", err)
					}
				}
			}
		}(i)
	}

	wg.Wait()

	// assert
	assert.Greater(t, successCountSingle.Load(), int32(0))
	assert.Greater(t, successCountMultiple.Load(), int32(0))
	assert.Greater(t, conflictCountSingle.Load(), int32(0))
	assert.Greater(t, conflictCountMultiple.Load(), int32(0))

	events, _, err := es.Query(ctxWithTimeout, estesthelpers.FilterAllEventTypesForOneBookOrReader(bookID, readerID))
	assert.NoError(t, err)
	assert.Equal(t, int(eventCount.Load()), len(events))
}

func Test_Append_EventWithMetadata(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := estesthelpers.GivenUniqueID(t)
	filter := estesthelpers.FilterAllEventTypesForOneBook(bookID)
	maxSequenceNumberBeforeAppend := estesthelpers.QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter)
	fakeClock = fakeClock.Add(time.Second)
	bookCopyAddedToCirculation := estesthelpers.FixtureBookCopyAddedToCirculation(bookID, fakeClock)

	messageID := estesthelpers.GivenUniqueID(t)
	causationID := estesthelpers.GivenUniqueID(t)
	correlationID := estesthelpers.GivenUniqueID(t)
	eventMetadata := shared.BuildEventMetadata(messageID, causationID, correlationID)

	// act (append)
	err := es.Append(
		ctxWithTimeout,
		filter,
		maxSequenceNumberBeforeAppend,
		estesthelpers.ToStorableWithMetadata(t, bookCopyAddedToCirculation, eventMetadata),
	)

	// assert (append)
	assert.NoError(t, err)

	// act (query)
	actualEvents, _, queryErr := es.Query(ctxWithTimeout, filter)

	// assert (query)
	assert.NoError(t, queryErr)
	assert.Len(t, actualEvents, 1)
	actualDomainEvent, mappingErr := fixtures.DomainEventFrom(actualEvents[0])
	assert.NoError(t, mappingErr)
	actualEventMetadata, mappingErr := shared.EventMetadataFrom(actualEvents[0])
	assert.NoError(t, mappingErr)
	assert.Equal(t, bookCopyAddedToCirculation, actualDomainEvent)
	assert.Equal(t, eventMetadata, actualEventMetadata)
}

// Test_QueryingWithFilter_WorksAsExpected is a comprehensive integration test covering multiple filter scenarios.
// High line count is acceptable for exhaustive testing of complex query filtering logic.
//
//nolint:funlen
func Test_QueryingWithFilter_WorksAsExpected(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)

	bookID1 := estesthelpers.GivenUniqueID(t)
	bookID2 := estesthelpers.GivenUniqueID(t)
	readerID1 := estesthelpers.GivenUniqueID(t)
	readerID2 := estesthelpers.GivenUniqueID(t)

	fakeClock = fakeClock.Add(time.Second)
	bookCopy1AddedToCirculationBook := estesthelpers.GivenBookCopyAddedToCirculationWasAppended(t, ctxWithTimeout, es, bookID1, fakeClock)
	fakeClock = fakeClock.Add(time.Second)
	bookCopy1LentToReader1 := estesthelpers.GivenBookCopyLentToReaderWasAppended(t, ctxWithTimeout, es, bookID1, readerID1, fakeClock)
	fakeClock = fakeClock.Add(time.Second)
	bookCopy1ReturnedByReader1 := estesthelpers.GivenBookCopyReturnedByReaderWasAppended(t, ctxWithTimeout, es, bookID1, readerID1, fakeClock)
	fakeClock = fakeClock.Add(time.Second)
	bookCopy1RemovedFromCirculationBook := estesthelpers.GivenBookCopyRemovedFromCirculationWasAppended(t, ctxWithTimeout, es, bookID1, fakeClock)

	fakeClock = fakeClock.Add(time.Second)
	bookCopy2AddedToCirculationBook := estesthelpers.GivenBookCopyAddedToCirculationWasAppended(t, ctxWithTimeout, es, bookID2, fakeClock)
	fakeClock = fakeClock.Add(time.Second)
	bookCopy2LentToReader2 := estesthelpers.GivenBookCopyLentToReaderWasAppended(t, ctxWithTimeout, es, bookID2, readerID2, fakeClock)
	fakeClock = fakeClock.Add(time.Second)
	bookCopy2ReturnedByReader2 := estesthelpers.GivenBookCopyReturnedByReaderWasAppended(t, ctxWithTimeout, es, bookID2, readerID2, fakeClock)
	fakeClock = fakeClock.Add(time.Second)
	bookCopy2LentToReader1 := estesthelpers.GivenBookCopyLentToReaderWasAppended(t, ctxWithTimeout, es, bookID2, readerID1, fakeClock)
	fakeClock = fakeClock.Add(time.Second)
	bookCopy2ReturnedByReader1 := estesthelpers.GivenBookCopyReturnedByReaderWasAppended(t, ctxWithTimeout, es, bookID2, readerID1, fakeClock)

	/******************************/

	testCases := []struct {
		description       string
		filter            eventstore.Filter
		expectedNumEvents int
		expectedEvents    shared.DomainEvents
	}{
		{
			description:       "empty filter",
			filter:            eventstore.BuildEventFilter().MatchingAnyEvent(),
			expectedNumEvents: 9,
			expectedEvents:    shared.DomainEvents{}, // we don't want to assert the concrete events here
		},
		{
			description: "only (occurredFrom)",
			filter: eventstore.BuildEventFilter().
				OccurredFrom(bookCopy2AddedToCirculationBook.HasOccurredAt()).
				Finalize(),
			expectedNumEvents: 5,
			expectedEvents:    shared.DomainEvents{}, // we don't want to assert the concrete events here
		},
		{
			description: "only (occurredUntil)",
			filter: eventstore.BuildEventFilter().
				OccurredUntil(bookCopy1AddedToCirculationBook.HasOccurredAt()).
				Finalize(),
			expectedNumEvents: 1,
			expectedEvents:    shared.DomainEvents{}, // we don't want to assert the concrete events here
		},
		{
			description: "only (occurredFrom to occurredUntil)",
			filter: eventstore.BuildEventFilter().
				OccurredFrom(bookCopy1LentToReader1.HasOccurredAt()).
				AndOccurredUntil(bookCopy2ReturnedByReader2.HasOccurredAt()).
				Finalize(),
			expectedNumEvents: 6,
			expectedEvents:    shared.DomainEvents{}, // we don't want to assert the concrete events here
		},
		{
			description: "(EventType)",
			filter: eventstore.BuildEventFilter().
				Matching().
				AnyEventTypeOf(fixtures.BookCopyAddedToCirculationEventType).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: shared.DomainEvents{
				bookCopy1AddedToCirculationBook,
				bookCopy2AddedToCirculationBook},
		},
		{
			description: "(EventType OR EventType...)",
			filter: eventstore.BuildEventFilter().
				Matching().
				AnyEventTypeOf(
					fixtures.BookCopyAddedToCirculationEventType,
					fixtures.BookCopyRemovedFromCirculationEventType).
				Finalize(),
			expectedNumEvents: 3,
			expectedEvents: shared.DomainEvents{
				bookCopy1AddedToCirculationBook,
				bookCopy1RemovedFromCirculationBook,
				bookCopy2AddedToCirculationBook},
		},
		{
			description: "(Predicate)",
			filter: eventstore.BuildEventFilter().
				Matching().
				AnyPredicateOf(eventstore.P("BookID", bookID1.String())).
				Finalize(),
			expectedNumEvents: 4,
			expectedEvents: shared.DomainEvents{
				bookCopy1AddedToCirculationBook,
				bookCopy1LentToReader1,
				bookCopy1ReturnedByReader1,
				bookCopy1RemovedFromCirculationBook},
		},
		{
			description: "(Predicate OR Predicate...)",
			filter: eventstore.BuildEventFilter().
				Matching().
				AnyPredicateOf(
					eventstore.P("BookID", bookID1.String()),
					eventstore.P("ReaderID", readerID1.String())).
				Finalize(),
			expectedNumEvents: 6,
			expectedEvents: shared.DomainEvents{
				bookCopy1AddedToCirculationBook,
				bookCopy1LentToReader1,
				bookCopy1ReturnedByReader1,
				bookCopy1RemovedFromCirculationBook,
				bookCopy2LentToReader1,
				bookCopy2ReturnedByReader1},
		},
		{
			description: "(Predicate AND Predicate...)",
			filter: eventstore.BuildEventFilter().
				Matching().
				AllPredicatesOf(
					eventstore.P("BookID", bookID1.String()),
					eventstore.P("ReaderID", readerID1.String())).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: shared.DomainEvents{
				bookCopy1LentToReader1,
				bookCopy1ReturnedByReader1},
		},
		{
			description: "(EventType AND Predicate)",
			filter: eventstore.BuildEventFilter().
				Matching().
				AnyEventTypeOf(fixtures.BookCopyLentToReaderEventType).
				AndAnyPredicateOf(eventstore.P("ReaderID", readerID1.String())).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: shared.DomainEvents{
				bookCopy1LentToReader1,
				bookCopy2LentToReader1},
		},
		{
			description: "(EventType AND (Predicate OR Predicate...))",
			filter: eventstore.BuildEventFilter().
				Matching().
				AnyEventTypeOf(fixtures.BookCopyLentToReaderEventType).
				AndAnyPredicateOf(
					eventstore.P("BookID", bookID1.String()),
					eventstore.P("ReaderID", readerID2.String())).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: shared.DomainEvents{
				bookCopy1LentToReader1,
				bookCopy2LentToReader2},
		},
		{
			description: "(EventType AND (Predicate AND Predicate...))",
			filter: eventstore.BuildEventFilter().
				Matching().
				AnyEventTypeOf(fixtures.BookCopyLentToReaderEventType).
				AndAllPredicatesOf(
					eventstore.P("BookID", bookID2.String()),
					eventstore.P("ReaderID", readerID1.String())).
				Finalize(),
			expectedNumEvents: 1,
			expectedEvents:    shared.DomainEvents{bookCopy2LentToReader1},
		},
		{
			description: "((EventType OR EventType...) AND Predicate...)",
			filter: eventstore.BuildEventFilter().
				Matching().
				AnyEventTypeOf(
					fixtures.BookCopyAddedToCirculationEventType,
					fixtures.BookCopyRemovedFromCirculationEventType).
				AndAnyPredicateOf(eventstore.P("BookID", bookID1.String())).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: shared.DomainEvents{
				bookCopy1AddedToCirculationBook,
				bookCopy1RemovedFromCirculationBook},
		},
		{
			description: "((EventType OR EventType...) AND (Predicate OR Predicate...))",
			filter: eventstore.BuildEventFilter().
				Matching().
				AnyEventTypeOf(
					fixtures.BookCopyLentToReaderEventType,
					fixtures.BookCopyReturnedByReaderEventType).
				AndAnyPredicateOf(
					eventstore.P("BookID", bookID1.String()),
					eventstore.P("BookID", bookID2.String())).
				Finalize(),
			expectedNumEvents: 6,
			expectedEvents: shared.DomainEvents{
				bookCopy1LentToReader1,
				bookCopy1ReturnedByReader1,
				bookCopy2LentToReader2,
				bookCopy2ReturnedByReader2,
				bookCopy2LentToReader1,
				bookCopy2ReturnedByReader1},
		},
		{
			description: "((EventType OR EventType...) AND (Predicate AND Predicate...))",
			filter: eventstore.BuildEventFilter().
				Matching().
				AnyEventTypeOf(
					fixtures.BookCopyLentToReaderEventType,
					fixtures.BookCopyReturnedByReaderEventType).
				AndAllPredicatesOf(
					eventstore.P("BookID", bookID2.String()),
					eventstore.P("ReaderID", readerID1.String())).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: shared.DomainEvents{
				bookCopy2LentToReader1,
				bookCopy2ReturnedByReader1},
		},
		{
			description: "((EventType AND Predicate) OR (EventType AND Predicate)...)",
			filter: eventstore.BuildEventFilter().
				Matching().
				AnyPredicateOf(eventstore.P("BookID", bookID1.String())).
				AndAnyEventTypeOf(fixtures.BookCopyLentToReaderEventType).
				OrMatching().
				AnyPredicateOf(eventstore.P("BookID", bookID2.String())).
				AndAnyEventTypeOf(fixtures.BookCopyReturnedByReaderEventType).
				Finalize(),
			expectedNumEvents: 3,
			expectedEvents: shared.DomainEvents{
				bookCopy1LentToReader1,
				bookCopy2ReturnedByReader2,
				bookCopy2ReturnedByReader1},
		},
		{
			description: "... (occurredFrom)",
			filter: eventstore.BuildEventFilter().
				Matching().
				AnyPredicateOf(eventstore.P("BookID", bookID1.String())).
				AndAnyEventTypeOf(fixtures.BookCopyLentToReaderEventType).
				OrMatching().
				AnyPredicateOf(eventstore.P("BookID", bookID2.String())).
				AndAnyEventTypeOf(fixtures.BookCopyReturnedByReaderEventType).
				OccurredFrom(bookCopy2ReturnedByReader2.HasOccurredAt()).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: shared.DomainEvents{
				bookCopy2ReturnedByReader2,
				bookCopy2ReturnedByReader1},
		},
		{
			description: "... (occurredUntil)",
			filter: eventstore.BuildEventFilter().
				Matching().
				AnyPredicateOf(eventstore.P("BookID", bookID1.String())).
				AndAnyEventTypeOf(fixtures.BookCopyLentToReaderEventType).
				OrMatching().
				AnyPredicateOf(eventstore.P("BookID", bookID2.String())).
				AndAnyEventTypeOf(fixtures.BookCopyReturnedByReaderEventType).
				OccurredUntil(bookCopy2ReturnedByReader2.HasOccurredAt()).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: shared.DomainEvents{
				bookCopy1LentToReader1,
				bookCopy2ReturnedByReader2},
		},
		{
			description: "... (occurredFrom to occurredUntil)",
			filter: eventstore.BuildEventFilter().
				Matching().
				AnyPredicateOf(eventstore.P("BookID", bookID1.String())).
				AndAnyEventTypeOf(fixtures.BookCopyLentToReaderEventType).
				OrMatching().
				AnyPredicateOf(eventstore.P("BookID", bookID2.String())).
				AndAnyEventTypeOf(fixtures.BookCopyReturnedByReaderEventType).
				OccurredFrom(bookCopy2ReturnedByReader2.HasOccurredAt()).
				AndOccurredUntil(bookCopy2ReturnedByReader2.HasOccurredAt()).
				Finalize(),
			expectedNumEvents: 1,
			expectedEvents:    shared.DomainEvents{bookCopy2ReturnedByReader2},
		},
		{
			description: "... (sequenceNumberHigherThan)",
			filter: eventstore.BuildEventFilter().
				Matching().
				AnyPredicateOf(eventstore.P("BookID", bookID1.String())).
				AndAnyEventTypeOf(fixtures.BookCopyLentToReaderEventType).
				OrMatching().
				AnyPredicateOf(eventstore.P("BookID", bookID2.String())).
				AndAnyEventTypeOf(fixtures.BookCopyReturnedByReaderEventType).
				WithSequenceNumberHigherThan(8).
				Finalize(),
			expectedNumEvents: 1,
			expectedEvents:    shared.DomainEvents{bookCopy2ReturnedByReader1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// act
			actualEvents, _, queryErr := es.Query(ctxWithTimeout, tc.filter)

			// assert
			assert.NoError(t, queryErr)
			assert.Len(t, actualEvents, tc.expectedNumEvents)

			actualDomainEvents, mappingErr := fixtures.DomainEventsFrom(actualEvents)
			assert.NoError(t, mappingErr)

			for i := 0; i < len(tc.expectedEvents); i++ {
				assert.Equal(t, tc.expectedEvents[i], actualDomainEvents[i])
			}
		})
	}
}

func Test_Append_When_Context_Is_Cancelled(t *testing.T) {
	// setup
	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := estesthelpers.GivenUniqueID(t)
	filter := estesthelpers.FilterAllEventTypesForOneBook(bookID)

	maxSequenceNumberBeforeAppend := estesthelpers.QueryMaxSequenceNumberBeforeAppend(t, context.Background(), es, filter)

	ctxWithCancel, cancel := context.WithCancel(context.Background())

	// act
	cancel()
	fakeClock = fakeClock.Add(time.Second)
	err := es.Append(
		ctxWithCancel,
		filter,
		maxSequenceNumberBeforeAppend,
		estesthelpers.ToStorable(t, estesthelpers.FixtureBookCopyAddedToCirculation(bookID, fakeClock)),
	)

	// assert
	assert.ErrorContains(t, err, "context canceled")
	events, _, queryErr := es.Query(context.Background(), filter)
	assert.NoError(t, queryErr, "verification query should succeed")
	assert.Empty(t, events, "no events should have been inserted when context was canceled")
}

func Test_Append_When_Context_Times_out(t *testing.T) {
	// setup
	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := estesthelpers.GivenUniqueID(t)
	filter := estesthelpers.FilterAllEventTypesForOneBook(bookID)

	maxSequenceNumberBeforeAppend := estesthelpers.QueryMaxSequenceNumberBeforeAppend(t, context.Background(), es, filter)

	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), time.Microsecond)
	defer cancel()

	time.Sleep(5 * time.Microsecond) // Give the context time to expire

	// act
	fakeClock = fakeClock.Add(time.Second)
	err := es.Append(
		ctxWithTimeout,
		filter,
		maxSequenceNumberBeforeAppend,
		estesthelpers.ToStorable(t, estesthelpers.FixtureBookCopyAddedToCirculation(bookID, fakeClock)),
	)

	// assert
	assert.ErrorContains(t, err, "context deadline exceeded")
	events, _, queryErr := es.Query(context.Background(), filter)
	assert.NoError(t, queryErr, "verification query should succeed")
	assert.Empty(t, events, "no events should have been inserted when context was canceled")
}

func Test_Query_When_Context_Is_Canceled(t *testing.T) {
	// setup
	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := estesthelpers.GivenUniqueID(t)

	fakeClock = fakeClock.Add(time.Second)
	estesthelpers.GivenBookCopyAddedToCirculationWasAppended(t, context.Background(), es, bookID, fakeClock)

	filter := estesthelpers.FilterAllEventTypesForOneBook(bookID)

	ctxWithCancel, cancel := context.WithCancel(context.Background())

	// act
	cancel()
	events, maxSeq, err := es.Query(ctxWithCancel, filter)

	// assert
	assert.ErrorContains(t, err, "context canceled")
	assert.Empty(t, events, "no events should be returned when context is canceled")
	assert.Equal(t, eventstore.MaxSequenceNumberUint(0), maxSeq, "max sequence should be 0 when context is canceled")
}

func Test_Query_When_Context_Times_Out(t *testing.T) {
	// setup
	wrapper := pgtesthelpers.CreateWrapperWithTestConfig(t)
	defer wrapper.Close()
	es := wrapper.GetEventStore()
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	pgtesthelpers.CleanUp(t, wrapper)
	bookID := estesthelpers.GivenUniqueID(t)

	fakeClock = fakeClock.Add(time.Second)
	estesthelpers.GivenBookCopyAddedToCirculationWasAppended(t, context.Background(), es, bookID, fakeClock)

	filter := estesthelpers.FilterAllEventTypesForOneBook(bookID)

	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), time.Microsecond)
	defer cancel()

	time.Sleep(5 * time.Microsecond) // Give the context time to expire

	// act
	events, maxSeq, err := es.Query(ctxWithTimeout, filter)

	// assert
	assert.ErrorContains(t, err, "context deadline exceeded")
	assert.Empty(t, events, "no events should be returned when context times out")
	assert.Equal(t, eventstore.MaxSequenceNumberUint(0), maxSeq, "max sequence should be 0 when context times out")
}
