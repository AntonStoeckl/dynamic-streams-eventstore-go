package postgresengine_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"

	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/test"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/test/userland/config"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/test/userland/core"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/test/userland/shell"
)

func Test_Append_When_NoEvent_MatchesTheQuery_BeforeAppend(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolTestConfig())
	defer connPool.Close()
	assert.NoError(t, err, "error connecting to DB pool in test setup")

	es, err := NewEventStoreFromPGXPoolWithTableName(connPool, "events")
	assert.NoError(t, err, "creating the event store failed")

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUpEvents(t, connPool)
	fakeClock = GivenSomeOtherEventsWereAppended(t, ctxWithTimeout, es, rand.IntN(5)+1, 0, fakeClock)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)
	maxSequenceNumberBeforeAppend := QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter)

	// act
	fakeClock = fakeClock.Add(time.Second)
	err = es.Append(
		ctxWithTimeout,
		filter,
		maxSequenceNumberBeforeAppend,
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock)),
	)

	// assert
	assert.NoError(t, err, "error in appending the event")
}

func Test_Append_When_SomeEvents_MatchTheQuery_BeforeAppend(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolTestConfig())
	defer connPool.Close()
	assert.NoError(t, err, "error connecting to DB pool in test setup")

	es := NewEventStoreFromPGXPool(connPool)

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUpEvents(t, connPool)
	fakeClock = GivenSomeOtherEventsWereAppended(t, ctxWithTimeout, es, rand.IntN(5)+1, 0, fakeClock)
	bookID := GivenUniqueID(t)
	fakeClock = fakeClock.Add(time.Second)
	GivenBookCopyAddedToCirculationWasAppended(t, ctxWithTimeout, es, bookID, fakeClock)
	filter := FilterAllEventTypesForOneBook(bookID)
	maxSequenceNumberBeforeAppend := QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter)

	// act
	fakeClock = fakeClock.Add(time.Second)
	appendErr := es.Append(
		ctxWithTimeout,
		filter,
		maxSequenceNumberBeforeAppend,
		ToStorable(t, FixtureBookCopyRemovedFromCirculation(bookID, fakeClock)),
	)

	// assert
	assert.NoError(t, appendErr, "error in appending the events")
}

func Test_Append_When_A_ConcurrencyConflict_ShouldHappen(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolTestConfig())
	defer connPool.Close()
	assert.NoError(t, err, "error connecting to DB pool in test setup")

	es := NewEventStoreFromPGXPool(connPool)

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUpEvents(t, connPool)
	fakeClock = GivenSomeOtherEventsWereAppended(t, ctxWithTimeout, es, rand.IntN(5)+1, 0, fakeClock)
	bookID := GivenUniqueID(t)
	readerID := GivenUniqueID(t)
	fakeClock = fakeClock.Add(time.Second)
	GivenBookCopyAddedToCirculationWasAppended(t, ctxWithTimeout, es, bookID, fakeClock)
	filter := FilterAllEventTypesForOneBook(bookID)
	maxSequenceNumberBeforeAppend := QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter)
	fakeClock = fakeClock.Add(time.Second)
	GivenBookCopyLentToReaderWasAppended(t, ctxWithTimeout, es, bookID, readerID, fakeClock) // concurrent append

	// act
	fakeClock = fakeClock.Add(time.Second)
	err = es.Append(
		ctxWithTimeout,
		filter,
		maxSequenceNumberBeforeAppend,
		ToStorable(t, FixtureBookCopyRemovedFromCirculation(bookID, fakeClock)),
	)

	// assert
	assert.Error(t, err)
	assert.ErrorContains(t, err, ErrConcurrencyConflict.Error())
}

func Test_AppendMultiple(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolTestConfig())
	defer connPool.Close()
	assert.NoError(t, err, "error connecting to DB pool in test setup")

	es := NewEventStoreFromPGXPool(connPool)

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUpEvents(t, connPool)
	fakeClock = GivenSomeOtherEventsWereAppended(t, ctxWithTimeout, es, rand.IntN(5)+1, 0, fakeClock)
	bookID := GivenUniqueID(t)
	readerID := GivenUniqueID(t)
	fakeClock = fakeClock.Add(time.Second)
	GivenBookCopyAddedToCirculationWasAppended(t, ctxWithTimeout, es, bookID, fakeClock)
	filter := FilterAllEventTypesForOneBook(bookID)
	maxSequenceNumberBeforeAppend := QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter)

	// act
	fakeClock = fakeClock.Add(time.Second)
	appendErr := es.Append(
		ctxWithTimeout,
		filter,
		maxSequenceNumberBeforeAppend,
		ToStorable(t, FixtureBookCopyLentToReader(bookID, readerID, fakeClock)),
		ToStorable(t, FixtureBookCopyReturnedByReader(bookID, readerID, fakeClock)),
	)

	// assert
	assert.NoError(t, appendErr, "error in appending the event")
	actualEvents, _, queryErr := es.Query(ctxWithTimeout, filter)
	assert.NoError(t, queryErr, "error in querying the appended events back")
	assert.Len(t, actualEvents, 3, "there should be exactly 3 events") // 1 in arrange and 2 in act
}

func Test_AppendMultiple_When_A_ConcurrencyConflict_ShouldHappen(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolTestConfig())
	defer connPool.Close()
	assert.NoError(t, err, "error connecting to DB pool in test setup")

	es := NewEventStoreFromPGXPool(connPool)

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUpEvents(t, connPool)
	fakeClock = GivenSomeOtherEventsWereAppended(t, ctxWithTimeout, es, rand.IntN(5)+1, 0, fakeClock)
	bookID := GivenUniqueID(t)
	readerID := GivenUniqueID(t)
	fakeClock = fakeClock.Add(time.Second)
	GivenBookCopyAddedToCirculationWasAppended(t, ctxWithTimeout, es, bookID, fakeClock)
	filter := FilterAllEventTypesForOneBook(bookID)
	maxSequenceNumberBeforeAppend := QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter)
	fakeClock = fakeClock.Add(time.Second)
	GivenBookCopyLentToReaderWasAppended(t, ctxWithTimeout, es, bookID, readerID, fakeClock) // concurrent append

	// act
	fakeClock = fakeClock.Add(time.Second)
	appendErr := es.Append(
		ctxWithTimeout,
		filter,
		maxSequenceNumberBeforeAppend,
		ToStorable(t, FixtureBookCopyLentToReader(bookID, readerID, fakeClock)),
		ToStorable(t, FixtureBookCopyReturnedByReader(bookID, readerID, fakeClock)),
	)

	// assert
	assert.Error(t, appendErr)
	assert.ErrorContains(t, appendErr, ErrConcurrencyConflict.Error())
}

func Test_Append_Concurrent(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	connPool, configErr := pgxpool.NewWithConfig(ctxWithTimeout, config.PostgresPGXPoolTestConfig())
	defer connPool.Close()
	assert.NoError(t, configErr, "error connecting to DB pool in test setup")

	es := NewEventStoreFromPGXPool(connPool)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUpEvents(t, connPool)
	bookID := GivenUniqueID(t)
	readerID := GivenUniqueID(t)

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

		go func(routineNum int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				filter := FilterAllEventTypesForOneBookOrReader(bookID, readerID)
				maxSeq := QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter)

				// Randomly choose between appending single and multiple event(s)
				if rand.IntN(2)%2 == 0 {
					// Single event
					err := es.Append(
						ctxWithTimeout,
						filter,
						maxSeq,
						ToStorable(t, FixtureBookCopyLentToReader(bookID, readerID, fakeClock)),
					)
					if err == nil {
						successCountSingle.Add(1)
						eventCount.Add(1)
					} else if errors.Is(err, ErrConcurrencyConflict) {
						conflictCountSingle.Add(1)
					} else {
						assert.FailNow(t, "unexpected error")
					}
				} else {
					// Multiple events
					event1 := ToStorable(t, FixtureBookCopyLentToReader(bookID, readerID, fakeClock))
					event2 := ToStorable(t, FixtureBookCopyReturnedByReader(bookID, readerID, fakeClock))
					err := es.Append(
						ctxWithTimeout,
						filter,
						maxSeq,
						event1,
						event2,
					)
					if err == nil {
						successCountMultiple.Add(1)
						eventCount.Add(2) // Count both events
					} else if errors.Is(err, ErrConcurrencyConflict) {
						conflictCountMultiple.Add(1)
					} else {
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

	events, _, err := es.Query(ctxWithTimeout, FilterAllEventTypesForOneBookOrReader(bookID, readerID))
	assert.NoError(t, err)
	assert.Equal(t, int(eventCount.Load()), len(events))
}

func Test_Append_EventWithMetadata(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolTestConfig())
	assert.NoError(t, err, "error connecting to DB pool in test setup")
	defer connPool.Close()

	es := NewEventStoreFromPGXPool(connPool)

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUpEvents(t, connPool)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)
	maxSequenceNumberBeforeAppend := QueryMaxSequenceNumberBeforeAppend(t, ctxWithTimeout, es, filter)
	fakeClock = fakeClock.Add(time.Second)
	bookCopyAddedToCirculation := FixtureBookCopyAddedToCirculation(bookID, fakeClock)

	messageID := GivenUniqueID(t)
	causationID := GivenUniqueID(t)
	correlationID := GivenUniqueID(t)
	eventMetadata := shell.BuildEventMetadata(messageID, causationID, correlationID)

	// act (append)
	err = es.Append(
		ctxWithTimeout,
		filter,
		maxSequenceNumberBeforeAppend,
		ToStorableWithMetadata(t, bookCopyAddedToCirculation, eventMetadata),
	)

	// assert (append)
	assert.NoError(t, err, "error in appending the event")

	// act (query)
	actualEvents, _, queryErr := es.Query(ctxWithTimeout, filter)

	// assert (query)
	assert.NoError(t, queryErr, "error in querying the events")
	assert.Len(t, actualEvents, 1, "there should be exactly 1 event")
	actualEventEnvelopes, mappingFooErr := shell.EventEnvelopesFrom(actualEvents)
	assert.NoError(t, mappingFooErr, "error in mapping the storable events to event envelopes")
	assert.Equal(t, bookCopyAddedToCirculation, actualEventEnvelopes[0].DomainEvent, "the queried domain event should be equal to the appended event")
	assert.Equal(t, eventMetadata, actualEventEnvelopes[0].EventMetadata, "the queried event metadata should be equal to the appended event")
}

func Test_QueryingWithFilter_WorksAsExpected(t *testing.T) {
	// setup
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolTestConfig())
	assert.NoError(t, err, "error connecting to DB pool in test setup")
	defer connPool.Close()

	es := NewEventStoreFromPGXPool(connPool)

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUpEvents(t, connPool)
	numOtherEvents := 10
	fakeClock = GivenSomeOtherEventsWereAppended(t, ctxWithTimeout, es, numOtherEvents, 0, fakeClock)

	bookID1 := GivenUniqueID(t)
	bookID2 := GivenUniqueID(t)
	readerID1 := GivenUniqueID(t)
	readerID2 := GivenUniqueID(t)

	fakeClock = fakeClock.Add(time.Second)
	bookCopy1AddedToCirculationBook := GivenBookCopyAddedToCirculationWasAppended(t, ctxWithTimeout, es, bookID1, fakeClock)
	fakeClock = fakeClock.Add(time.Second)
	bookCopy1LentToReader1 := GivenBookCopyLentToReaderWasAppended(t, ctxWithTimeout, es, bookID1, readerID1, fakeClock)
	fakeClock = fakeClock.Add(time.Second)
	bookCopy1ReturnedByReader1 := GivenBookCopyReturnedByReaderWasAppended(t, ctxWithTimeout, es, bookID1, readerID1, fakeClock)
	fakeClock = fakeClock.Add(time.Second)
	bookCopy1RemovedFromCirculationBook := GivenBookCopyRemovedFromCirculationWasAppended(t, ctxWithTimeout, es, bookID1, fakeClock)

	fakeClock = fakeClock.Add(time.Second)
	bookCopy2AddedToCirculationBook := GivenBookCopyAddedToCirculationWasAppended(t, ctxWithTimeout, es, bookID2, fakeClock)
	fakeClock = fakeClock.Add(time.Second)
	bookCopy2LentToReader2 := GivenBookCopyLentToReaderWasAppended(t, ctxWithTimeout, es, bookID2, readerID2, fakeClock)
	fakeClock = fakeClock.Add(time.Second)
	bookCopy2ReturnedByReader2 := GivenBookCopyReturnedByReaderWasAppended(t, ctxWithTimeout, es, bookID2, readerID2, fakeClock)
	fakeClock = fakeClock.Add(time.Second)
	bookCopy2LentToReader1 := GivenBookCopyLentToReaderWasAppended(t, ctxWithTimeout, es, bookID2, readerID1, fakeClock)
	fakeClock = fakeClock.Add(time.Second)
	bookCopy2ReturnedByReader1 := GivenBookCopyReturnedByReaderWasAppended(t, ctxWithTimeout, es, bookID2, readerID1, fakeClock)

	/******************************/

	testCases := []struct {
		description       string
		filter            Filter
		expectedNumEvents int
		expectedEvents    core.DomainEvents
	}{
		{
			description:       "empty filter",
			filter:            BuildEventFilter().MatchingAnyEvent(),
			expectedNumEvents: numOtherEvents + 9,
			expectedEvents:    core.DomainEvents{}, // we don't want to assert the concrete events here
		},
		{
			description: "only (occurredFrom)",
			filter: BuildEventFilter().
				OccurredFrom(bookCopy2AddedToCirculationBook.HasOccurredAt()).
				Finalize(),
			expectedNumEvents: 5,
			expectedEvents:    core.DomainEvents{}, // we don't want to assert the concrete events here
		},
		{
			description: "only (occurredUntil)",
			filter: BuildEventFilter().
				OccurredUntil(bookCopy1AddedToCirculationBook.HasOccurredAt()).
				Finalize(),
			expectedNumEvents: numOtherEvents + 1,
			expectedEvents:    core.DomainEvents{}, // we don't want to assert the concrete events here
		},
		{
			description: "only (occurredFrom to occurredUntil)",
			filter: BuildEventFilter().
				OccurredFrom(bookCopy1LentToReader1.HasOccurredAt()).
				AndOccurredUntil(bookCopy2ReturnedByReader2.HasOccurredAt()).
				Finalize(),
			expectedNumEvents: 6,
			expectedEvents:    core.DomainEvents{}, // we don't want to assert the concrete events here
		},
		{
			description: "(EventType)",
			filter: BuildEventFilter().
				Matching().
				AnyEventTypeOf(core.BookCopyAddedToCirculationEventType).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: core.DomainEvents{
				bookCopy1AddedToCirculationBook,
				bookCopy2AddedToCirculationBook},
		},
		{
			description: "(EventType OR EventType...)",
			filter: BuildEventFilter().
				Matching().
				AnyEventTypeOf(
					core.BookCopyAddedToCirculationEventType,
					core.BookCopyRemovedFromCirculationEventType).
				Finalize(),
			expectedNumEvents: 3,
			expectedEvents: core.DomainEvents{
				bookCopy1AddedToCirculationBook,
				bookCopy1RemovedFromCirculationBook,
				bookCopy2AddedToCirculationBook},
		},
		{
			description: "(Predicate)",
			filter: BuildEventFilter().
				Matching().
				AnyPredicateOf(P("BookID", bookID1.String())).
				Finalize(),
			expectedNumEvents: 4,
			expectedEvents: core.DomainEvents{
				bookCopy1AddedToCirculationBook,
				bookCopy1LentToReader1,
				bookCopy1ReturnedByReader1,
				bookCopy1RemovedFromCirculationBook},
		},
		{
			description: "(Predicate OR Predicate...)",
			filter: BuildEventFilter().
				Matching().
				AnyPredicateOf(
					P("BookID", bookID1.String()),
					P("ReaderID", readerID1.String())).
				Finalize(),
			expectedNumEvents: 6,
			expectedEvents: core.DomainEvents{
				bookCopy1AddedToCirculationBook,
				bookCopy1LentToReader1,
				bookCopy1ReturnedByReader1,
				bookCopy1RemovedFromCirculationBook,
				bookCopy2LentToReader1,
				bookCopy2ReturnedByReader1},
		},
		{
			description: "(Predicate AND Predicate...)",
			filter: BuildEventFilter().
				Matching().
				AllPredicatesOf(
					P("BookID", bookID1.String()),
					P("ReaderID", readerID1.String())).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: core.DomainEvents{
				bookCopy1LentToReader1,
				bookCopy1ReturnedByReader1},
		},
		{
			description: "(EventType AND Predicate)",
			filter: BuildEventFilter().
				Matching().
				AnyEventTypeOf(core.BookCopyLentToReaderEventType).
				AndAnyPredicateOf(P("ReaderID", readerID1.String())).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: core.DomainEvents{
				bookCopy1LentToReader1,
				bookCopy2LentToReader1},
		},
		{
			description: "(EventType AND (Predicate OR Predicate...))",
			filter: BuildEventFilter().
				Matching().
				AnyEventTypeOf(core.BookCopyLentToReaderEventType).
				AndAnyPredicateOf(
					P("BookID", bookID1.String()),
					P("ReaderID", readerID2.String())).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: core.DomainEvents{
				bookCopy1LentToReader1,
				bookCopy2LentToReader2},
		},
		{
			description: "(EventType AND (Predicate AND Predicate...))",
			filter: BuildEventFilter().
				Matching().
				AnyEventTypeOf(core.BookCopyLentToReaderEventType).
				AndAllPredicatesOf(
					P("BookID", bookID2.String()),
					P("ReaderID", readerID1.String())).
				Finalize(),
			expectedNumEvents: 1,
			expectedEvents:    core.DomainEvents{bookCopy2LentToReader1},
		},
		{
			description: "((EventType OR EventType...) AND Predicate...)",
			filter: BuildEventFilter().
				Matching().
				AnyEventTypeOf(
					core.BookCopyAddedToCirculationEventType,
					core.BookCopyRemovedFromCirculationEventType).
				AndAnyPredicateOf(P("BookID", bookID1.String())).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: core.DomainEvents{
				bookCopy1AddedToCirculationBook,
				bookCopy1RemovedFromCirculationBook},
		},
		{
			description: "((EventType OR EventType...) AND (Predicate OR Predicate...))",
			filter: BuildEventFilter().
				Matching().
				AnyEventTypeOf(
					core.BookCopyLentToReaderEventType,
					core.BookCopyReturnedByReaderEventType).
				AndAnyPredicateOf(
					P("BookID", bookID1.String()),
					P("BookID", bookID2.String())).
				Finalize(),
			expectedNumEvents: 6,
			expectedEvents: core.DomainEvents{
				bookCopy1LentToReader1,
				bookCopy1ReturnedByReader1,
				bookCopy2LentToReader2,
				bookCopy2ReturnedByReader2,
				bookCopy2LentToReader1,
				bookCopy2ReturnedByReader1},
		},
		{
			description: "((EventType OR EventType...) AND (Predicate AND Predicate...))",
			filter: BuildEventFilter().
				Matching().
				AnyEventTypeOf(
					core.BookCopyLentToReaderEventType,
					core.BookCopyReturnedByReaderEventType).
				AndAllPredicatesOf(
					P("BookID", bookID2.String()),
					P("ReaderID", readerID1.String())).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: core.DomainEvents{
				bookCopy2LentToReader1,
				bookCopy2ReturnedByReader1},
		},
		{
			description: "((EventType AND Predicate) OR (EventType AND Predicate)...)",
			filter: BuildEventFilter().
				Matching().
				AnyPredicateOf(P("BookID", bookID1.String())).
				AndAnyEventTypeOf(core.BookCopyLentToReaderEventType).
				OrMatching().
				AnyPredicateOf(P("BookID", bookID2.String())).
				AndAnyEventTypeOf(core.BookCopyReturnedByReaderEventType).
				Finalize(),
			expectedNumEvents: 3,
			expectedEvents: core.DomainEvents{
				bookCopy1LentToReader1,
				bookCopy2ReturnedByReader2,
				bookCopy2ReturnedByReader1},
		},
		{
			description: "... (occurredFrom)",
			filter: BuildEventFilter().
				Matching().
				AnyPredicateOf(P("BookID", bookID1.String())).
				AndAnyEventTypeOf(core.BookCopyLentToReaderEventType).
				OrMatching().
				AnyPredicateOf(P("BookID", bookID2.String())).
				AndAnyEventTypeOf(core.BookCopyReturnedByReaderEventType).
				OccurredFrom(bookCopy2ReturnedByReader2.HasOccurredAt()).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: core.DomainEvents{
				bookCopy2ReturnedByReader2,
				bookCopy2ReturnedByReader1},
		},
		{
			description: "... (occurredUntil)",
			filter: BuildEventFilter().
				Matching().
				AnyPredicateOf(P("BookID", bookID1.String())).
				AndAnyEventTypeOf(core.BookCopyLentToReaderEventType).
				OrMatching().
				AnyPredicateOf(P("BookID", bookID2.String())).
				AndAnyEventTypeOf(core.BookCopyReturnedByReaderEventType).
				OccurredUntil(bookCopy2ReturnedByReader2.HasOccurredAt()).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: core.DomainEvents{
				bookCopy1LentToReader1,
				bookCopy2ReturnedByReader2},
		},
		{
			description: "... (occurredFrom to occurredUntil)",
			filter: BuildEventFilter().
				Matching().
				AnyPredicateOf(P("BookID", bookID1.String())).
				AndAnyEventTypeOf(core.BookCopyLentToReaderEventType).
				OrMatching().
				AnyPredicateOf(P("BookID", bookID2.String())).
				AndAnyEventTypeOf(core.BookCopyReturnedByReaderEventType).
				OccurredFrom(bookCopy2ReturnedByReader2.HasOccurredAt()).
				AndOccurredUntil(bookCopy2ReturnedByReader2.HasOccurredAt()).
				Finalize(),
			expectedNumEvents: 1,
			expectedEvents:    core.DomainEvents{bookCopy2ReturnedByReader2},
		},
	}

	BuildEventFilter().OccurredFrom(time.Now()).AndOccurredUntil(time.Now()).Finalize()

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// act
			actualEvents, _, queryErr := es.Query(ctxWithTimeout, tc.filter)

			// assert
			assert.NoError(t, queryErr, "error in querying the events")
			assert.Len(t, actualEvents, tc.expectedNumEvents, fmt.Sprintf("there should be exactly %d events", tc.expectedNumEvents))

			actualDomainEvents, mappingErr := shell.DomainEventsFrom(actualEvents)
			assert.NoError(t, mappingErr, "error in mapping the storable events to domain events")

			for i := 0; i < len(tc.expectedEvents); i++ {
				assert.Equal(t, tc.expectedEvents[i], actualDomainEvents[i], "the queried event should be equal to the appended event")
			}
		})
	}
}

func Test_Append_When_Context_Is_Cancelled(t *testing.T) {
	// setup
	connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolTestConfig())
	defer connPool.Close()
	assert.NoError(t, err, "error connecting to DB pool in test setup")

	es := NewEventStoreFromPGXPool(connPool)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUpEvents(t, connPool)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	maxSequenceNumberBeforeAppend := QueryMaxSequenceNumberBeforeAppend(t, context.Background(), es, filter)

	ctxWithCancel, cancel := context.WithCancel(context.Background())

	// act
	cancel()
	fakeClock = fakeClock.Add(time.Second)
	err = es.Append(
		ctxWithCancel,
		filter,
		maxSequenceNumberBeforeAppend,
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock)),
	)

	// assert
	assert.Error(t, err, "expected error due to cancelled context")
	assert.Contains(t, err.Error(), "context canceled")
	events, _, queryErr := es.Query(context.Background(), filter)
	assert.NoError(t, queryErr, "verification query should succeed")
	assert.Empty(t, events, "no events should have been inserted when context was cancelled")
}

func Test_Append_When_Context_Times_out(t *testing.T) {
	// setup
	connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolTestConfig())
	defer connPool.Close()
	assert.NoError(t, err, "error connecting to DB pool in test setup")

	es := NewEventStoreFromPGXPool(connPool)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUpEvents(t, connPool)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)

	maxSequenceNumberBeforeAppend := QueryMaxSequenceNumberBeforeAppend(t, context.Background(), es, filter)

	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), time.Microsecond)
	defer cancel()

	time.Sleep(5 * time.Microsecond) // Give the context time to expire

	// act
	fakeClock = fakeClock.Add(time.Second)
	err = es.Append(
		ctxWithTimeout,
		filter,
		maxSequenceNumberBeforeAppend,
		ToStorable(t, FixtureBookCopyAddedToCirculation(bookID, fakeClock)),
	)

	// assert
	assert.Error(t, err, "expected error due to context timeout")
	assert.Contains(t, err.Error(), "context deadline exceeded")
	events, _, queryErr := es.Query(context.Background(), filter)
	assert.NoError(t, queryErr, "verification query should succeed")
	assert.Empty(t, events, "no events should have been inserted when context was cancelled")
}

func Test_Query_When_Context_Is_Cancelled(t *testing.T) {
	// setup
	connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolTestConfig())
	defer connPool.Close()
	assert.NoError(t, err, "error connecting to DB pool in test setup")

	es := NewEventStoreFromPGXPool(connPool)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUpEvents(t, connPool)
	bookID := GivenUniqueID(t)

	fakeClock = fakeClock.Add(time.Second)
	GivenBookCopyAddedToCirculationWasAppended(t, context.Background(), es, bookID, fakeClock)

	filter := FilterAllEventTypesForOneBook(bookID)

	ctxWithCancel, cancel := context.WithCancel(context.Background())

	// act
	cancel()
	events, maxSeq, err := es.Query(ctxWithCancel, filter)

	// assert
	assert.Error(t, err, "expected error due to cancelled context")
	assert.Contains(t, err.Error(), "context canceled")
	assert.Empty(t, events, "no events should be returned when context is cancelled")
	assert.Equal(t, MaxSequenceNumberUint(0), maxSeq, "max sequence should be 0 when context is cancelled")
}

func Test_Query_When_Context_Times_Out(t *testing.T) {
	// setup
	connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresPGXPoolTestConfig())
	defer connPool.Close()
	assert.NoError(t, err, "error connecting to DB pool in test setup")

	es := NewEventStoreFromPGXPool(connPool)
	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	CleanUpEvents(t, connPool)
	bookID := GivenUniqueID(t)

	fakeClock = fakeClock.Add(time.Second)
	GivenBookCopyAddedToCirculationWasAppended(t, context.Background(), es, bookID, fakeClock)

	filter := FilterAllEventTypesForOneBook(bookID)

	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), time.Microsecond)
	defer cancel()

	time.Sleep(5 * time.Microsecond) // Give the context time to expire

	// act
	events, maxSeq, err := es.Query(ctxWithTimeout, filter)

	// assert
	assert.Error(t, err, "expected error due to context timeout")
	assert.Contains(t, err.Error(), "context deadline exceeded")
	assert.Empty(t, events, "no events should be returned when context times out")
	assert.Equal(t, MaxSequenceNumberUint(0), maxSeq, "max sequence should be 0 when context times out")
}
