package test

import (
	"context"
	"math/rand/v2"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"

	. "dynamic-streams-eventstore/eventstore"
	. "dynamic-streams-eventstore/eventstore/engine"
	"dynamic-streams-eventstore/test/userland/core"
	"dynamic-streams-eventstore/test/userland/shell"
)

func GivenUniqueID(t testing.TB) uuid.UUID {
	bookID, err := uuid.NewV7()
	assert.NoError(t, err, "error in arranging test data")

	return bookID
}

func QueryMaxSequenceNumberBeforeAppend(t testing.TB, es PostgresEventStore, filter Filter) MaxSequenceNumberUint {
	_, maxSequenceNumBeforeAppend, err := es.Query(filter)
	assert.NoError(t, err, "error in arranging test data")

	return maxSequenceNumBeforeAppend
}

func FilterAllEventTypesForOneBook(bookID uuid.UUID) Filter {
	filter := BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			core.BookCopyAddedToCirculationEventType,
			core.BookCopyRemovedFromCirculationEventType,
			core.BookCopyLentToReaderEventType,
			core.BookCopyReturnedByReaderEventType).
		AndAnyPredicateOf(P("BookID", bookID.String())).
		Finalize()

	return filter
}

func FilterAllEventTypesForOneBookOrReader(bookID uuid.UUID, readerID uuid.UUID) Filter {
	filter := BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			core.BookCopyAddedToCirculationEventType,
			core.BookCopyRemovedFromCirculationEventType,
			core.BookCopyLentToReaderEventType,
			core.BookCopyReturnedByReaderEventType).
		AndAnyPredicateOf(
			P("BookID", bookID.String()),
			P("ReaderID", readerID.String())).
		Finalize()

	return filter
}

func FixtureBookCopyAddedToCirculation(bookID uuid.UUID) core.DomainEvent {
	return core.BuildBookCopyAddedToCirculation(
		bookID,
		"978-1-098-10013-1",
		"Learning Domain-Driven Design",
		"Vlad Khononov",
		"First Edition",
		"O'Reilly Media, Inc.",
		2021,
		time.Now(),
	)
}

func FixtureBookCopyRemovedFromCirculation(bookID uuid.UUID) core.DomainEvent {
	return core.BuildBookCopyRemovedFromCirculation(bookID, time.Now())
}

func FixtureBookCopyLentToReader(bookID uuid.UUID, readerID uuid.UUID) core.DomainEvent {
	return core.BuildBookCopyLentToReader(bookID, readerID, time.Now())
}

func FixtureBookCopyReturnedByReader(bookID uuid.UUID, readerID uuid.UUID) core.DomainEvent {
	return core.BuildBookCopyReturnedFromReader(bookID, readerID, time.Now())
}

func ToStorable(t testing.TB, domainEvent core.DomainEvent) StorableEvent {
	storableEvent, err := shell.StorableEventFrom(domainEvent)
	assert.NoError(t, err, "error in arranging test data")

	return storableEvent
}

func GivenBookCopyAddedToCirculationWasAppended(t testing.TB, es PostgresEventStore, bookID uuid.UUID) core.DomainEvent {
	filter := FilterAllEventTypesForOneBook(bookID)
	event := FixtureBookCopyAddedToCirculation(bookID)
	err := es.Append(ToStorable(t, event), filter, QueryMaxSequenceNumberBeforeAppend(t, es, filter))
	assert.NoError(t, err, "error in arranging test data")

	return event
}

func GivenBookCopyRemovedFromCirculationWasAppended(t testing.TB, es PostgresEventStore, bookID uuid.UUID) core.DomainEvent {
	filter := FilterAllEventTypesForOneBook(bookID)
	event := FixtureBookCopyRemovedFromCirculation(bookID)
	err := es.Append(ToStorable(t, event), filter, QueryMaxSequenceNumberBeforeAppend(t, es, filter))
	assert.NoError(t, err, "error in arranging test data")

	return event
}

func GivenBookCopyLentToReaderWasAppended(t testing.TB, es PostgresEventStore, bookID uuid.UUID, readerID uuid.UUID) core.DomainEvent {
	filter := FilterAllEventTypesForOneBookOrReader(bookID, readerID)
	event := FixtureBookCopyLentToReader(bookID, readerID)
	err := es.Append(ToStorable(t, event), filter, QueryMaxSequenceNumberBeforeAppend(t, es, filter))
	assert.NoError(t, err, "error in arranging test data")

	return event
}

func GivenBookCopyReturnedByReaderWasAppended(t testing.TB, es PostgresEventStore, bookID uuid.UUID, readerID uuid.UUID) core.DomainEvent {
	filter := FilterAllEventTypesForOneBookOrReader(bookID, readerID)
	event := FixtureBookCopyReturnedByReader(bookID, readerID)
	err := es.Append(ToStorable(t, event), filter, QueryMaxSequenceNumberBeforeAppend(t, es, filter))
	assert.NoError(t, err, "error in arranging test data")

	return event
}

func GivenSomeOtherEventsWereAppended(t testing.TB, es PostgresEventStore, numEvents int, startFrom MaxSequenceNumberUint) {
	maxSequenceNumber := startFrom
	totalEvent := 0
	eventPostfix := 0

	for {
		id, err := uuid.NewV7()
		assert.NoError(t, err, "error in arranging test data")

		event := core.BuildSomethingHasHappened(
			id.String(),
			"lorem ipsum dolor sit amet: "+id.String(),
			time.Now().UTC().Truncate(time.Microsecond),
			core.SomethingHasHappenedEventTypePrefix+strconv.Itoa(eventPostfix))

		amountOfSameEvents := rand.IntN(3) + 1

		for j := 0; j < amountOfSameEvents; j++ {
			filter := BuildEventFilter().
				Matching().
				AnyEventTypeOf(core.SomethingHasHappenedEventTypePrefix + strconv.Itoa(eventPostfix)).
				AndAnyPredicateOf(P("ID", id.String())).
				Finalize()

			maxSequenceNumberForThisEventType := maxSequenceNumber
			if j == 0 {
				maxSequenceNumberForThisEventType = 0
			}

			err = es.Append(ToStorable(t, event), filter, maxSequenceNumberForThisEventType)
			assert.NoError(t, err, "error in arranging test data")

			totalEvent++
			maxSequenceNumber++

			if totalEvent%5000 == 0 {
				//fmt.Printf("appended %d %s events into the DB\n", totalEvent, core.SomethingHasHappenedEventTypePrefix)
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

	//fmt.Printf("appended %d %s events into the DB\n", totalEvent, core.SomethingHasHappenedEventTypePrefix)
}

func CleanUpEvents(t testing.TB, connPool *pgxpool.Pool) {
	_, err := connPool.Exec(
		context.Background(),
		"TRUNCATE TABLE events RESTART IDENTITY",
	)

	assert.NoError(t, err, "error cleaning up the events table")
	//fmt.Println("events table truncated")
}
