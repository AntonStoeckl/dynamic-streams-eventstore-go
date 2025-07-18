package engine_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"

	"dynamic-streams-eventstore/config"
	. "dynamic-streams-eventstore/eventstore"
	. "dynamic-streams-eventstore/eventstore/engine"
	. "dynamic-streams-eventstore/test"
	"dynamic-streams-eventstore/test/userland/core"
	"dynamic-streams-eventstore/test/userland/shell"
)

func Test_Append_When_NoEvent_Matches_TheQuery_BeforeAppend(t *testing.T) {
	// setup
	connPool, err := pgxpool.NewWithConfig(context.Background(), config.DBTestConfig())
	defer connPool.Close()
	assert.NoError(t, err, "error connecting to DB pool in test setup")

	es := NewPostgresEventStore(connPool)

	// arrange
	CleanUpEvents(t, connPool)
	GivenSomeOtherEventsWereAppended(t, es, rand.IntN(5)+1, 0)
	bookID := GivenUniqueID(t)
	filter := FilterAllEventTypesForOneBook(bookID)
	maxSequenceNumberBeforeAppend := QueryMaxSequenceNumberBeforeAppend(t, es, filter)

	// act
	err = es.Append(
		ToStorable(t, BuildBookCopyAddedToCirculation(bookID)),
		filter,
		maxSequenceNumberBeforeAppend,
	)

	// assert
	assert.NoError(t, err, "error in appending the event")
}

func Test_Append_When_SomeEvents_Match_TheQuery_BeforeAppend(t *testing.T) {
	// setup
	connPool, err := pgxpool.NewWithConfig(context.Background(), config.DBTestConfig())
	defer connPool.Close()
	assert.NoError(t, err, "error connecting to DB pool in test setup")

	es := NewPostgresEventStore(connPool)

	// arrange
	CleanUpEvents(t, connPool)
	GivenSomeOtherEventsWereAppended(t, es, rand.IntN(5)+1, 0)
	bookID := GivenUniqueID(t)
	GivenBookCopyAddedToCirculationWasAppended(t, es, bookID)
	filter := FilterAllEventTypesForOneBook(bookID)
	maxSequenceNumberBeforeAppend := QueryMaxSequenceNumberBeforeAppend(t, es, filter)

	// act
	err = es.Append(
		ToStorable(t, BuildBookCopyRemovedFromCirculation(bookID)),
		filter,
		maxSequenceNumberBeforeAppend,
	)

	// assert
	assert.NoError(t, err, "error in appending the event")
}

func Test_Append_When_A_ConcurrencyConflict_ShouldHappen(t *testing.T) {
	// setup
	connPool, err := pgxpool.NewWithConfig(context.Background(), config.DBTestConfig())
	defer connPool.Close()
	assert.NoError(t, err, "error connecting to DB pool in test setup")

	es := NewPostgresEventStore(connPool)

	// arrange
	CleanUpEvents(t, connPool)
	GivenSomeOtherEventsWereAppended(t, es, rand.IntN(5)+1, 0)
	bookID := GivenUniqueID(t)
	readerID := GivenUniqueID(t)
	GivenBookCopyAddedToCirculationWasAppended(t, es, bookID)
	filter := FilterAllEventTypesForOneBook(bookID)
	maxSequenceNumberBeforeAppend := QueryMaxSequenceNumberBeforeAppend(t, es, filter)

	GivenBookCopyLentToReaderWasAppended(t, es, bookID, readerID) // concurrent append

	// act
	err = es.Append(
		ToStorable(t, BuildBookCopyRemovedFromCirculation(bookID)),
		filter,
		maxSequenceNumberBeforeAppend,
	)

	// assert
	assert.Error(t, err)
	assert.ErrorContains(t, err, ErrConcurrencyConflict.Error())
}

func Test_Querying_With_Filter_Works_As_Expected(t *testing.T) {
	// setup
	connPool, err := pgxpool.NewWithConfig(context.Background(), config.DBTestConfig())
	assert.NoError(t, err, "error connecting to DB pool in test setup")
	defer connPool.Close()

	es := NewPostgresEventStore(connPool)

	// arrange
	CleanUpEvents(t, connPool)
	numOtherEvents := 10
	GivenSomeOtherEventsWereAppended(t, es, numOtherEvents, 0)

	bookID1 := GivenUniqueID(t)
	bookID2 := GivenUniqueID(t)
	readerID1 := GivenUniqueID(t)
	readerID2 := GivenUniqueID(t)

	bookCopy1AddedToCirculationBook := GivenBookCopyAddedToCirculationWasAppended(t, es, bookID1)
	bookCopy1LentToReader1 := GivenBookCopyLentToReaderWasAppended(t, es, bookID1, readerID1)
	bookCopy1ReturnedByReader1 := GivenBookCopyReturnedByReaderWasAppended(t, es, bookID1, readerID1)
	bookCopy1RemovedFromCirculationBook := GivenBookCopyRemovedFromCirculationWasAppended(t, es, bookID1)

	bookCopy2AddedToCirculationBook := GivenBookCopyAddedToCirculationWasAppended(t, es, bookID2)
	bookCopy2LentToReader2 := GivenBookCopyLentToReaderWasAppended(t, es, bookID2, readerID2)
	bookCopy2ReturnedByReader2 := GivenBookCopyReturnedByReaderWasAppended(t, es, bookID2, readerID2)
	bookCopy2LentToReader1 := GivenBookCopyLentToReaderWasAppended(t, es, bookID2, readerID1)
	bookCopy2ReturnedByReader1 := GivenBookCopyReturnedByReaderWasAppended(t, es, bookID2, readerID1)

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
			expectedEvents:    core.DomainEvents{}, // we don't want to assert the random "something has happened" events
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
				AndAnyPredicateOf(P("BookID", bookID1.String()), P("ReaderID", readerID2.String())).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: core.DomainEvents{
				bookCopy1LentToReader1,
				bookCopy2LentToReader2},
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
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// act
			actualEvents, _, queryErr := es.Query(tc.filter)

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

//func Benchmark_Append_With_1000000_Events_InTheStore(b *testing.B) {
//	// setup
//	factor := 1
//	connPool, err := pgxpool.NewWithConfig(context.Background(), config.DBTestConfig())
//	defer connPool.Close()
//	assert.NoError(b, err, "error connecting to DB pool in test setup")
//
//	es := NewPostgresEventStore(connPool)
//
//	// arrange
//	row := connPool.QueryRow(context.Background(), `SELECT count(*) FROM events`)
//	var cnt int
//	err = row.Scan(&cnt)
//
//	fmt.Printf("found %d events in the DB\n", cnt)
//
//	assert.NoError(b, err, "error in arranging test data")
//
//	if cnt < 1000*factor {
//		fmt.Println("DomainEvent setup will run")
//		CleanUpEvents(b, connPool)
//		GivenSomeOtherEventsWereAppended(b, es, 900*factor, 0)
//
//		var totalEvents int
//		for i := 0; i < 10*factor; i++ {
//			bookID := GivenUniqueID(b)
//
//			for j := 0; j < 5; j++ {
//				GivenBookCopyAddedToCirculationWasAppended(b, es, bookID)
//				totalEvents++
//				GivenBookCopyRemovedFromCirculationWasAppended(b, es, bookID)
//				totalEvents++
//
//				if totalEvents%5000 == 0 {
//					fmt.Printf("appended %d events into the DB\n", totalEvents)
//				}
//			}
//		}
//
//		fmt.Printf("appended %d events into the DB\n", totalEvents)
//	} else {
//		fmt.Println("DomainEvent setup will NOT run")
//	}
//
//	bookID := GivenUniqueID(b)
//	filter := FilterAllEventTypesForOneBook(bookID)
//
//	// act
//	b.Run("append", func(b *testing.B) {
//		b.ResetTimer()
//
//		for i := 0; i < b.N; i++ {
//			b.StopTimer()
//			maxSequenceNumberBeforeAppend := QueryMaxSequenceNumberBeforeAppend(b, es, filter)
//			b.StartTimer()
//
//			err = es.Append(
//				ToStorable(b, BuildBookCopyAddedToCirculation(bookID)),
//				filter,
//				maxSequenceNumberBeforeAppend,
//			)
//			assert.NoError(b, err, "error in running benchmark action")
//
//			b.StopTimer()
//			cmdTag, dbErr := connPool.Exec(
//				context.Background(),
//				fmt.Sprintf(`DELETE FROM events WHERE payload @> '{"BookID": "%s"}'`, bookID.String()),
//			)
//			assert.NoError(b, dbErr, "error in cleaning up benchmark artefacts")
//			assert.Equal(b, 1, int(cmdTag.RowsAffected()))
//			b.StartTimer()
//		}
//	})
//}

//func Benchmark_Query_With_1000000_Events_InTheStore(b *testing.B) {
//	// setup
//	factor := 1
//	connPool, err := pgxpool.NewWithConfig(context.Background(), config.DBTestConfig())
//	defer connPool.Close()
//	assert.NoError(b, err, "error connecting to DB pool in test setup")
//
//	es := NewPostgresEventStore(connPool)
//
//	// arrange
//	row := connPool.QueryRow(context.Background(), `SELECT count(*) FROM events`)
//	var cnt int
//	err = row.Scan(&cnt)
//	assert.NoError(b, err, "error in arranging test data")
//
//	var totalEvents int
//	if cnt < 1000*factor {
//		fmt.Println("DomainEvent setup will run")
//		CleanUpEvents(b, connPool)
//		GivenSomeOtherEventsWereAppended(b, es, 900*factor, 0)
//
//		for i := 0; i < 10*factor; i++ {
//			bookID := GivenUniqueID(b)
//
//			for j := 0; j < 5; j++ {
//				GivenBookCopyAddedToCirculationWasAppended(b, es, bookID)
//				totalEvents++
//				GivenBookCopyRemovedFromCirculationWasAppended(b, es, bookID)
//				totalEvents++
//
//				if totalEvents%5000 == 0 {
//					fmt.Printf("appended %d events in the DB\n", totalEvents)
//				}
//			}
//		}
//
//		fmt.Printf("appended %d events into the DB\n", totalEvents)
//	} else {
//		fmt.Println("DomainEvent setup will NOT run")
//	}
//
//	row = connPool.QueryRow(
//		context.Background(),
//		`select payload->'BookID' as bookID from events where sequence_number = (select max(sequence_number) from events)`,
//	)
//	var bookIDString string
//	err = row.Scan(&bookIDString)
//	assert.NoError(b, err, "error in arranging test data")
//	bookID, err := uuid.Parse(bookIDString)
//	assert.NoError(b, err, "error in arranging test data")
//
//	filter := FilterAllEventTypesForOneBook(bookID)
//
//	// act
//	b.Run("query", func(b *testing.B) {
//		b.ResetTimer()
//
//		for i := 0; i < b.N; i++ {
//			QueryMaxSequenceNumberBeforeAppend(b, es, filter)
//		}
//	})
//}
