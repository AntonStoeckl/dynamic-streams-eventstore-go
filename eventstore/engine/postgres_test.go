package engine_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"

	"dynamic-streams-eventstore/config"
	. "dynamic-streams-eventstore/eventstore"
	. "dynamic-streams-eventstore/eventstore/engine"
	"dynamic-streams-eventstore/test/userland"
)

type maxSequenceNumberUint = uint

func Test_Append_When_NoEvent_Matches_TheQuery_BeforeAppend(t *testing.T) {
	// setup
	connPool, err := pgxpool.NewWithConfig(context.Background(), config.DBConfig())
	defer connPool.Close()
	assert.NoError(t, err, "error connecting to DB pool in test setup")

	es := NewPostgresEventStore(connPool, userland.EventFromJSON)

	// arrange
	cleanUpEvents(t, connPool)
	givenSomeOtherEventsWereAppended(t, es, rand.IntN(5)+1, 0)
	bookID := givenUniqueID(t)
	filter := filterAllEventTypesForOneBook(bookID)
	maxSequenceNumberBeforeAppend := queryMaxSequenceNumberBeforeAppend(t, es, filter)

	// act
	err = es.Append(
		buildBookCopyAddedToCirculation(bookID),
		filter,
		maxSequenceNumberBeforeAppend,
	)

	// assert
	assert.NoError(t, err, "error in appending the event")
}

func Test_Append_When_SomeEvents_Match_TheQuery_BeforeAppend(t *testing.T) {
	// setup
	connPool, err := pgxpool.NewWithConfig(context.Background(), config.DBConfig())
	defer connPool.Close()
	assert.NoError(t, err, "error connecting to DB pool in test setup")

	es := NewPostgresEventStore(connPool, userland.EventFromJSON)

	// arrange
	cleanUpEvents(t, connPool)
	givenSomeOtherEventsWereAppended(t, es, rand.IntN(5)+1, 0)
	bookID := givenUniqueID(t)
	givenBookCopyAddedToCirculationWasAppended(t, es, bookID)
	filter := filterAllEventTypesForOneBook(bookID)
	maxSequenceNumberBeforeAppend := queryMaxSequenceNumberBeforeAppend(t, es, filter)

	// act
	err = es.Append(
		buildBookCopyRemovedFromCirculation(bookID),
		filter,
		maxSequenceNumberBeforeAppend,
	)

	// assert
	assert.NoError(t, err, "error in appending the event")
}

func Test_Append_When_A_ConcurrencyConflict_ShouldHappen(t *testing.T) {
	// setup
	connPool, err := pgxpool.NewWithConfig(context.Background(), config.DBConfig())
	defer connPool.Close()
	assert.NoError(t, err, "error connecting to DB pool in test setup")

	es := NewPostgresEventStore(connPool, userland.EventFromJSON)

	// arrange
	cleanUpEvents(t, connPool)
	givenSomeOtherEventsWereAppended(t, es, rand.IntN(5)+1, 0)
	bookID := givenUniqueID(t)
	readerID := givenUniqueID(t)
	givenBookCopyAddedToCirculationWasAppended(t, es, bookID)
	filter := filterAllEventTypesForOneBook(bookID)
	maxSequenceNumberBeforeAppend := queryMaxSequenceNumberBeforeAppend(t, es, filter)

	givenBookCopyLentToReaderWasAppended(t, es, bookID, readerID) // concurrent append

	// act
	err = es.Append(
		buildBookCopyRemovedFromCirculation(bookID),
		filter,
		maxSequenceNumberBeforeAppend,
	)

	// assert
	assert.Error(t, err)
	assert.ErrorContains(t, err, ErrConcurrencyConflict.Error())
}

func Test_Querying_With_Filter_Works_As_Expected(t *testing.T) {
	// setup
	connPool, err := pgxpool.NewWithConfig(context.Background(), config.DBConfig())
	assert.NoError(t, err, "error connecting to DB pool in test setup")
	defer connPool.Close()

	es := NewPostgresEventStore(connPool, userland.EventFromJSON)

	// arrange
	cleanUpEvents(t, connPool)
	numOtherEvents := 10
	givenSomeOtherEventsWereAppended(t, es, numOtherEvents, 0)

	bookID1 := givenUniqueID(t)
	bookID2 := givenUniqueID(t)
	readerID1 := givenUniqueID(t)
	readerID2 := givenUniqueID(t)

	bookCopy1AddedToCirculationBook := givenBookCopyAddedToCirculationWasAppended(t, es, bookID1)
	bookCopy1LentToReader1 := givenBookCopyLentToReaderWasAppended(t, es, bookID1, readerID1)
	bookCopy1ReturnedByReader1 := givenBookCopyReturnedByReaderWasAppended(t, es, bookID1, readerID1)
	bookCopy1RemovedFromCirculationBook := givenBookCopyRemovedFromCirculationWasAppended(t, es, bookID1)

	bookCopy2AddedToCirculationBook := givenBookCopyAddedToCirculationWasAppended(t, es, bookID2)
	bookCopy2LentToReader2 := givenBookCopyLentToReaderWasAppended(t, es, bookID2, readerID2)
	bookCopy2ReturnedByReader2 := givenBookCopyReturnedByReaderWasAppended(t, es, bookID2, readerID2)
	bookCopy2LentToReader1 := givenBookCopyLentToReaderWasAppended(t, es, bookID2, readerID1)
	bookCopy2ReturnedByReader1 := givenBookCopyReturnedByReaderWasAppended(t, es, bookID2, readerID1)

	/******************************/

	testCases := []struct {
		description       string
		filter            Filter
		expectedNumEvents int
		expectedEvents    userland.Events
	}{
		{
			description:       "empty filter",
			filter:            BuildEventFilter().MatchingAnyEvent(),
			expectedNumEvents: numOtherEvents + 9,
			expectedEvents:    userland.Events{}, // we don't want to assert the random "something has happened" events
		},
		{
			description: "(EventType)",
			filter: BuildEventFilter().
				Matching().
				AnyEventTypeOf(userland.BookCopyAddedToCirculationEventType).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: userland.Events{
				bookCopy1AddedToCirculationBook,
				bookCopy2AddedToCirculationBook},
		},
		{
			description: "(EventType OR EventType...)",
			filter: BuildEventFilter().
				Matching().
				AnyEventTypeOf(
					userland.BookCopyAddedToCirculationEventType,
					userland.BookCopyRemovedFromCirculationEventType).
				Finalize(),
			expectedNumEvents: 3,
			expectedEvents: userland.Events{
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
			expectedEvents: userland.Events{
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
			expectedEvents: userland.Events{
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
				AnyEventTypeOf(userland.BookCopyLentToReaderEventType).
				AndAnyPredicateOf(P("ReaderID", readerID1.String())).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: userland.Events{
				bookCopy1LentToReader1,
				bookCopy2LentToReader1},
		},
		{
			description: "(EventType AND (Predicate OR Predicate...))",
			filter: BuildEventFilter().
				Matching().
				AnyEventTypeOf(userland.BookCopyLentToReaderEventType).
				AndAnyPredicateOf(P("BookID", bookID1.String()), P("ReaderID", readerID2.String())).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: userland.Events{
				bookCopy1LentToReader1,
				bookCopy2LentToReader2},
		},
		{
			description: "((EventType OR EventType...) AND Predicate...)",
			filter: BuildEventFilter().
				Matching().
				AnyEventTypeOf(
					userland.BookCopyAddedToCirculationEventType,
					userland.BookCopyRemovedFromCirculationEventType).
				AndAnyPredicateOf(P("BookID", bookID1.String())).
				Finalize(),
			expectedNumEvents: 2,
			expectedEvents: userland.Events{
				bookCopy1AddedToCirculationBook,
				bookCopy1RemovedFromCirculationBook},
		},
		{
			description: "((EventType OR EventType...) AND (Predicate OR Predicate...))",
			filter: BuildEventFilter().
				Matching().
				AnyEventTypeOf(
					userland.BookCopyLentToReaderEventType,
					userland.BookCopyReturnedByReaderEventType).
				AndAnyPredicateOf(
					P("BookID", bookID1.String()),
					P("BookID", bookID2.String())).
				Finalize(),
			expectedNumEvents: 6,
			expectedEvents: userland.Events{
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
				AndAnyEventTypeOf(userland.BookCopyLentToReaderEventType).
				OrMatching().
				AnyPredicateOf(P("BookID", bookID2.String())).
				AndAnyEventTypeOf(userland.BookCopyReturnedByReaderEventType).
				Finalize(),
			expectedNumEvents: 3,
			expectedEvents: userland.Events{
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
			for i := 0; i < len(tc.expectedEvents); i++ {
				assert.Equal(t, tc.expectedEvents[i], actualEvents[i], "the queried event should be equal to the appended event")
			}
		})
	}
}

func Benchmark_Append_With_1000000_Events_InTheStore(b *testing.B) {
	// setup
	connPool, err := pgxpool.NewWithConfig(context.Background(), config.DBConfig())
	defer connPool.Close()
	assert.NoError(b, err, "error connecting to DB pool in test setup")

	es := NewPostgresEventStore(connPool, userland.EventFromJSON)

	// arrange
	row := connPool.QueryRow(context.Background(), `SELECT count(*) FROM events`)
	var cnt int
	err = row.Scan(&cnt)

	fmt.Printf("found %d events in the DB\n", cnt)

	assert.NoError(b, err, "error in arranging test data")

	if cnt < 1000000 {
		fmt.Println("Event setup will run")
		cleanUpEvents(b, connPool)
		givenSomeOtherEventsWereAppended(b, es, 900000, 0)

		var totalEvents int
		for i := 0; i < 10000; i++ {
			bookID := givenUniqueID(b)

			for j := 0; j < 5; j++ {
				filter := filterAllEventTypesForOneBook(bookID)
				err = es.Append(
					buildBookCopyAddedToCirculation(bookID),
					filter,
					queryMaxSequenceNumberBeforeAppend(b, es, filter),
				)
				assert.NoError(b, err, "error in arranging test data")

				totalEvents++

				err = es.Append(
					buildBookCopyRemovedFromCirculation(bookID),
					filter,
					queryMaxSequenceNumberBeforeAppend(b, es, filter),
				)
				assert.NoError(b, err, "error in arranging test data")

				totalEvents++

				if totalEvents%5000 == 0 {
					fmt.Printf("appended %d events in the DB\n", totalEvents)
				}
			}
		}
	} else {
		fmt.Println("Event setup will NOT run")
	}

	bookID := givenUniqueID(b)
	filter := filterAllEventTypesForOneBook(bookID)

	// act
	b.Run("append", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			b.StopTimer()
			maxSequenceNumberBeforeAppend := queryMaxSequenceNumberBeforeAppend(b, es, filter)
			b.StartTimer()

			err = es.Append(
				buildBookCopyAddedToCirculation(bookID),
				filter,
				maxSequenceNumberBeforeAppend,
			)
			assert.NoError(b, err, "error in running benchmark action")

			b.StopTimer()
			cmdTag, dbErr := connPool.Exec(
				context.Background(),
				fmt.Sprintf(`DELETE FROM events WHERE payload @> '{"BookID": "%s"}'`, bookID.String()),
			)
			assert.NoError(b, dbErr, "error in cleaning up benchmark artefacts")
			assert.Equal(b, 1, int(cmdTag.RowsAffected()))
			b.StartTimer()
		}
	})
}

func Benchmark_Query_With_1000000_Events_InTheStore(b *testing.B) {
	// setup
	connPool, err := pgxpool.NewWithConfig(context.Background(), config.DBConfig())
	defer connPool.Close()
	assert.NoError(b, err, "error connecting to DB pool in test setup")

	es := NewPostgresEventStore(connPool, userland.EventFromJSON)

	// arrange
	row := connPool.QueryRow(context.Background(), `SELECT count(*) FROM events`)
	var cnt int
	err = row.Scan(&cnt)
	assert.NoError(b, err, "error in arranging test data")

	bookID := uuid.MustParse("01980de3-9296-7598-b929-557d3ab67686")

	if cnt < 1000000 {
		fmt.Println("Event setup will run")
		cleanUpEvents(b, connPool)
		givenSomeOtherEventsWereAppended(b, es, 900000, 0)

		for i := 0; i < 10000; i++ {
			bookID = givenUniqueID(b)

			for j := 0; j < 5; j++ {
				filter := filterAllEventTypesForOneBook(bookID)
				err = es.Append(
					buildBookCopyAddedToCirculation(bookID),
					filter,
					queryMaxSequenceNumberBeforeAppend(b, es, filter),
				)
				assert.NoError(b, err, "error in arranging test data")

				err = es.Append(
					buildBookCopyRemovedFromCirculation(bookID),
					filter,
					queryMaxSequenceNumberBeforeAppend(b, es, filter),
				)
				assert.NoError(b, err, "error in arranging test data")
			}
		}
	} else {
		fmt.Println("Event setup will NOT run")
	}

	filter := filterAllEventTypesForOneBook(bookID)

	// act
	b.Run("query", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			queryMaxSequenceNumberBeforeAppend(b, es, filter)
		}
	})
}

/***** Test HELPER Functions *****/

func givenUniqueID(t testing.TB) uuid.UUID {
	bookID, err := uuid.NewV7()
	assert.NoError(t, err, "error in arranging test data")

	return bookID
}

func queryMaxSequenceNumberBeforeAppend(t testing.TB, es PostgresEventStore, filter Filter) maxSequenceNumberUint {
	_, maxSequenceNumBeforeAppend, err := es.Query(filter)
	assert.NoError(t, err, "error in arranging test data")

	return maxSequenceNumBeforeAppend
}

func filterAllEventTypesForOneBook(bookID uuid.UUID) Filter {
	filter := BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			userland.BookCopyAddedToCirculationEventType,
			userland.BookCopyRemovedFromCirculationEventType,
			userland.BookCopyLentToReaderEventType,
			userland.BookCopyReturnedByReaderEventType).
		AndAnyPredicateOf(P("BookID", bookID.String())).
		Finalize()

	return filter
}

func filterAllEvenTypesForOneBookOrReader(bookID uuid.UUID, readerID uuid.UUID) Filter {
	filter := BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			userland.BookCopyAddedToCirculationEventType,
			userland.BookCopyRemovedFromCirculationEventType,
			userland.BookCopyLentToReaderEventType,
			userland.BookCopyReturnedByReaderEventType).
		AndAnyPredicateOf(
			P("BookID", bookID.String()),
			P("ReaderID", readerID.String())).
		Finalize()

	return filter
}

func buildBookCopyAddedToCirculation(bookID uuid.UUID) userland.BookCopyAddedToCirculation {
	event := userland.BookCopyAddedToCirculationFromPayload(
		userland.BookCopyAddedToCirculationPayload{
			BookID:          bookID.String(),
			ISBN:            "978-1-098-10013-1",
			Title:           "Learning Domain-Driven Design",
			Authors:         "Vlad Khononov",
			Edition:         "First Edition",
			Publisher:       "O'Reilly Media, Inc.",
			PublicationYear: 2021,
		},
	)

	return event
}

func buildBookCopyRemovedFromCirculation(bookID uuid.UUID) userland.BookCopyRemovedFromCirculation {
	event := userland.BookCopyRemovedFromCirculationFromPayload(
		userland.BookCopyRemovedFromCirculationPayload{
			BookID: bookID.String(),
		},
	)

	return event
}

func buildBookCopyLentToReader(bookID uuid.UUID, readerID uuid.UUID) userland.BookCopyLentToReader {
	event := userland.BookCopyLentToReaderFromPayload(
		userland.BookCopyLentToReaderPayload{
			BookID:   bookID.String(),
			ReaderID: readerID.String(),
		},
	)

	return event
}

func buildBookCopyReturnedFromReader(bookID uuid.UUID, readerID uuid.UUID) userland.BookCopyReturnedByReader {
	event := userland.BookCopyReturnedByReaderFromPayload(
		userland.BookCopyReturnedByReaderPayload{
			BookID:   bookID.String(),
			ReaderID: readerID.String(),
		},
	)

	return event
}

func givenBookCopyAddedToCirculationWasAppended(t *testing.T, es PostgresEventStore, bookID uuid.UUID) userland.Event {
	filter := filterAllEventTypesForOneBook(bookID)
	event := buildBookCopyAddedToCirculation(bookID)
	err := es.Append(event, filter, queryMaxSequenceNumberBeforeAppend(t, es, filter))
	assert.NoError(t, err, "error in arranging test data")

	return event
}

func givenBookCopyRemovedFromCirculationWasAppended(t *testing.T, es PostgresEventStore, bookID uuid.UUID) userland.Event {
	filter := filterAllEventTypesForOneBook(bookID)
	event := buildBookCopyRemovedFromCirculation(bookID)
	err := es.Append(event, filter, queryMaxSequenceNumberBeforeAppend(t, es, filter))
	assert.NoError(t, err, "error in arranging test data")

	return event
}

func givenBookCopyLentToReaderWasAppended(t *testing.T, es PostgresEventStore, bookID uuid.UUID, readerID uuid.UUID) userland.Event {
	filter := filterAllEvenTypesForOneBookOrReader(bookID, readerID)
	event := buildBookCopyLentToReader(bookID, readerID)
	err := es.Append(event, filter, queryMaxSequenceNumberBeforeAppend(t, es, filter))
	assert.NoError(t, err, "error in arranging test data")

	return event
}

func givenBookCopyReturnedByReaderWasAppended(t *testing.T, es PostgresEventStore, bookID uuid.UUID, readerID uuid.UUID) userland.Event {
	filter := filterAllEvenTypesForOneBookOrReader(bookID, readerID)
	event := buildBookCopyReturnedFromReader(bookID, readerID)
	err := es.Append(event, filter, queryMaxSequenceNumberBeforeAppend(t, es, filter))
	assert.NoError(t, err, "error in arranging test data")

	return event
}

func givenSomeOtherEventsWereAppended(t testing.TB, es PostgresEventStore, numEvents int, startFrom maxSequenceNumberUint) {
	maxSequenceNumber := startFrom
	totalEvent := 0
	eventPostfix := 0

	for {
		id, err := uuid.NewV7()
		assert.NoError(t, err, "error in arranging test data")

		event := userland.SomethingHasHappenedFromPayload(
			userland.SomethingHasHappenedPayload{
				ID:              id.String(),
				SomeInformation: "lorem ipsum dolor sit amet: " + id.String(),
			},
			eventPostfix,
		)

		amountOfSameEvents := rand.IntN(3) + 1

		for j := 0; j < amountOfSameEvents; j++ {
			filter := BuildEventFilter().
				Matching().
				AnyEventTypeOf(userland.SomethingHasHappenedEventTypePrefix + strconv.Itoa(eventPostfix)).
				AndAnyPredicateOf(P("ID", id.String())).
				Finalize()

			maxSequenceNumberForThisEventType := maxSequenceNumber
			if j == 0 {
				maxSequenceNumberForThisEventType = 0
			}

			err = es.Append(event, filter, maxSequenceNumberForThisEventType)
			assert.NoError(t, err, "error in arranging test data")

			totalEvent++
			maxSequenceNumber++

			if totalEvent%5000 == 0 {
				fmt.Printf("inserted %d %s events\n", totalEvent, userland.SomethingHasHappenedEventTypePrefix)
			}

			if totalEvent == numEvents {
				break
			}
		}

		eventPostfix++

		if totalEvent == numEvents {
			break
		}
	}

	fmt.Printf("inserted %d %s events\n", totalEvent, userland.SomethingHasHappenedEventTypePrefix)
}

func cleanUpEvents(t testing.TB, connPool *pgxpool.Pool) {
	_, err := connPool.Exec(
		context.Background(),
		"TRUNCATE TABLE events RESTART IDENTITY",
	)

	assert.NoError(t, err, "error cleaning up the events table")
	fmt.Println("events table truncated")
}
