package test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"

	. "dynamic-streams-eventstore/eventstore"
	. "dynamic-streams-eventstore/eventstore/engine"
	"dynamic-streams-eventstore/test/userland/core"
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

func FilterAllEvenTypesForOneBookOrReader(bookID uuid.UUID, readerID uuid.UUID) Filter {
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

func BuildBookCopyAddedToCirculation(bookID uuid.UUID) core.DomainEvent {
	event := core.BookCopyAddedToCirculation{
		BookID:          bookID.String(),
		ISBN:            "978-1-098-10013-1",
		Title:           "Learning Domain-Driven Design",
		Authors:         "Vlad Khononov",
		Edition:         "First Edition",
		Publisher:       "O'Reilly Media, Inc.",
		PublicationYear: 2021,
	}

	return event
}

func BuildBookCopyRemovedFromCirculation(bookID uuid.UUID) core.DomainEvent {
	event := core.BookCopyRemovedFromCirculation{
		BookID: bookID.String(),
	}

	return event
}

func BuildBookCopyLentToReader(bookID uuid.UUID, readerID uuid.UUID) core.DomainEvent {
	event := core.BookCopyLentToReader{
		BookID:   bookID.String(),
		ReaderID: readerID.String(),
	}

	return event
}

func BuildBookCopyReturnedFromReader(bookID uuid.UUID, readerID uuid.UUID) core.DomainEvent {
	event := core.BookCopyReturnedByReader{
		BookID:   bookID.String(),
		ReaderID: readerID.String(),
	}

	return event
}

func ToStorable(t testing.TB, event core.DomainEvent) StorableEvent {
	payloadJSON, err := json.Marshal(event)
	assert.NoError(t, err, "error in arranging test data")

	esEvent := BuildStorableEvent(event.EventType(), payloadJSON)

	return esEvent
}

func GivenBookCopyAddedToCirculationWasAppended(t testing.TB, es PostgresEventStore, bookID uuid.UUID) core.DomainEvent {
	filter := FilterAllEventTypesForOneBook(bookID)
	event := BuildBookCopyAddedToCirculation(bookID)
	err := es.Append(ToStorable(t, event), filter, QueryMaxSequenceNumberBeforeAppend(t, es, filter))
	assert.NoError(t, err, "error in arranging test data")

	return event
}

func GivenBookCopyRemovedFromCirculationWasAppended(t testing.TB, es PostgresEventStore, bookID uuid.UUID) core.DomainEvent {
	filter := FilterAllEventTypesForOneBook(bookID)
	event := BuildBookCopyRemovedFromCirculation(bookID)
	err := es.Append(ToStorable(t, event), filter, QueryMaxSequenceNumberBeforeAppend(t, es, filter))
	assert.NoError(t, err, "error in arranging test data")

	return event
}

func GivenBookCopyLentToReaderWasAppended(t testing.TB, es PostgresEventStore, bookID uuid.UUID, readerID uuid.UUID) core.DomainEvent {
	filter := FilterAllEvenTypesForOneBookOrReader(bookID, readerID)
	event := BuildBookCopyLentToReader(bookID, readerID)
	err := es.Append(ToStorable(t, event), filter, QueryMaxSequenceNumberBeforeAppend(t, es, filter))
	assert.NoError(t, err, "error in arranging test data")

	return event
}

func GivenBookCopyReturnedByReaderWasAppended(t testing.TB, es PostgresEventStore, bookID uuid.UUID, readerID uuid.UUID) core.DomainEvent {
	filter := FilterAllEvenTypesForOneBookOrReader(bookID, readerID)
	event := BuildBookCopyReturnedFromReader(bookID, readerID)
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
				fmt.Printf("appended %d %s events into the DB\n", totalEvent, core.SomethingHasHappenedEventTypePrefix)
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

	fmt.Printf("appended %d %s events into the DB\n", totalEvent, core.SomethingHasHappenedEventTypePrefix)
}

func CleanUpEvents(t testing.TB, connPool *pgxpool.Pool) {
	_, err := connPool.Exec(
		context.Background(),
		"TRUNCATE TABLE events RESTART IDENTITY",
	)

	assert.NoError(t, err, "error cleaning up the events table")
	fmt.Println("events table truncated")
}
