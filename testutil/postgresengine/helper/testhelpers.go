package helper

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/shell"
)

// GivenUniqueID generates a unique UUID for testing.
func GivenUniqueID(t testing.TB) uuid.UUID {
	bookID, err := uuid.NewV7()
	assert.NoError(t, err, "error in arranging test data")

	return bookID
}

// QueryMaxSequenceNumberBeforeAppend queries the current max sequence number for a filter.
func QueryMaxSequenceNumberBeforeAppend(
	t testing.TB,
	ctx context.Context, //nolint:revive //nolint:revive
	es *postgresengine.EventStore,
	filter eventstore.Filter,
) eventstore.MaxSequenceNumberUint {

	_, maxSequenceNumBeforeAppend, err := es.Query(ctx, filter)
	assert.NoError(t, err, "error in arranging test data")

	return maxSequenceNumBeforeAppend
}

// FilterAllEventTypesForOneBook creates a filter for all event types for a specific book.
func FilterAllEventTypesForOneBook(bookID uuid.UUID) eventstore.Filter {
	filter := eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			core.BookCopyAddedToCirculationEventType,
			core.BookCopyRemovedFromCirculationEventType,
			core.BookCopyLentToReaderEventType,
			core.BookCopyReturnedByReaderEventType).
		AndAnyPredicateOf(eventstore.P("BookID", bookID.String())).
		Finalize()

	return filter
}

// FilterAllEventTypesForOneBookOrReader creates a filter for book and reader events.
func FilterAllEventTypesForOneBookOrReader(bookID uuid.UUID, readerID uuid.UUID) eventstore.Filter {
	filter := eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			core.BookCopyAddedToCirculationEventType,
			core.BookCopyRemovedFromCirculationEventType,
			core.BookCopyLentToReaderEventType,
			core.BookCopyReturnedByReaderEventType).
		AndAnyPredicateOf(
			eventstore.P("BookID", bookID.String()),
			eventstore.P("ReaderID", readerID.String())).
		Finalize()

	return filter
}

// FixtureBookCopyAddedToCirculation creates a test event for adding a book to circulation.
func FixtureBookCopyAddedToCirculation(bookID uuid.UUID, fakeClock time.Time) core.DomainEvent {
	return core.BuildBookCopyAddedToCirculation(
		bookID,
		"978-1-098-10013-1",
		"Learning Domain-Driven Design",
		"Vlad Khononov",
		"First Edition",
		"O'Reilly Media, Inc.",
		2021,
		fakeClock,
	)
}

// FixtureBookCopyRemovedFromCirculation creates a test event for removing a book from circulation.
func FixtureBookCopyRemovedFromCirculation(bookID uuid.UUID, fakeClock time.Time) core.DomainEvent {
	return core.BuildBookCopyRemovedFromCirculation(bookID, fakeClock)
}

// FixtureBookCopyLentToReader creates a test event for lending a book to a reader.
func FixtureBookCopyLentToReader(
	bookID uuid.UUID,
	readerID uuid.UUID,
	fakeClock time.Time,
) core.DomainEvent {

	return core.BuildBookCopyLentToReader(bookID, readerID, fakeClock)
}

// FixtureBookCopyReturnedByReader creates a test event for returning a book.
func FixtureBookCopyReturnedByReader(
	bookID uuid.UUID,
	readerID uuid.UUID,
	fakeClock time.Time,
) core.DomainEvent {

	return core.BuildBookCopyReturnedFromReader(bookID, readerID, fakeClock)
}

// ToStorable converts a domain event to a storable event for testing.
func ToStorable(t testing.TB, domainEvent core.DomainEvent) eventstore.StorableEvent {
	storableEvent, err := shell.StorableEventWithEmptyMetadataFrom(domainEvent)
	assert.NoError(t, err, "error in arranging test data")

	return storableEvent
}

// ToStorableWithMetadata converts a domain event to a storable event with metadata.
func ToStorableWithMetadata(
	t testing.TB,
	domainEvent core.DomainEvent,
	eventMetadata shell.EventMetadata,
) eventstore.StorableEvent {

	storableEvent, err := shell.StorableEventFrom(domainEvent, eventMetadata)
	assert.NoError(t, err, "error in arranging test data")

	return storableEvent
}

// GivenBookCopyAddedToCirculationWasAppended appends a book addition event for testing.
func GivenBookCopyAddedToCirculationWasAppended(
	t testing.TB,
	ctx context.Context, //nolint:revive //nolint:revive
	es *postgresengine.EventStore,
	bookID uuid.UUID,
	fakeClock time.Time,
) core.DomainEvent {

	filter := FilterAllEventTypesForOneBook(bookID)
	event := FixtureBookCopyAddedToCirculation(bookID, fakeClock)
	err := es.Append(
		ctx,
		filter,
		QueryMaxSequenceNumberBeforeAppend(t, ctx, es, filter),
		ToStorable(t, event),
	)
	assert.NoError(t, err, "error in arranging test data")

	return event
}

// GivenBookCopyRemovedFromCirculationWasAppended appends a book removal event for testing.
func GivenBookCopyRemovedFromCirculationWasAppended(
	t testing.TB,
	ctx context.Context, //nolint:revive
	es *postgresengine.EventStore,
	bookID uuid.UUID,
	fakeClock time.Time,
) core.DomainEvent {

	filter := FilterAllEventTypesForOneBook(bookID)
	event := FixtureBookCopyRemovedFromCirculation(bookID, fakeClock)
	err := es.Append(
		ctx,
		filter,
		QueryMaxSequenceNumberBeforeAppend(t, ctx, es, filter),
		ToStorable(t, event),
	)
	assert.NoError(t, err, "error in arranging test data")

	return event
}

// GivenBookCopyLentToReaderWasAppended appends a book lending event for testing.
func GivenBookCopyLentToReaderWasAppended(
	t testing.TB,
	ctx context.Context, //nolint:revive
	es *postgresengine.EventStore,
	bookID uuid.UUID,
	readerID uuid.UUID,
	fakeClock time.Time,
) core.DomainEvent {

	filter := FilterAllEventTypesForOneBookOrReader(bookID, readerID)
	event := FixtureBookCopyLentToReader(bookID, readerID, fakeClock)
	err := es.Append(
		ctx,
		filter,
		QueryMaxSequenceNumberBeforeAppend(t, ctx, es, filter),
		ToStorable(t, event),
	)
	assert.NoError(t, err, "error in arranging test data")

	return event
}

// GivenBookCopyReturnedByReaderWasAppended appends a book return event for testing.
func GivenBookCopyReturnedByReaderWasAppended(
	t testing.TB,
	ctx context.Context, //nolint:revive
	es *postgresengine.EventStore,
	bookID uuid.UUID,
	readerID uuid.UUID,
	fakeClock time.Time,
) core.DomainEvent {

	filter := FilterAllEventTypesForOneBookOrReader(bookID, readerID)
	event := FixtureBookCopyReturnedByReader(bookID, readerID, fakeClock)
	err := es.Append(
		ctx,
		filter,
		QueryMaxSequenceNumberBeforeAppend(t, ctx, es, filter),
		ToStorable(t, event),
	)
	assert.NoError(t, err, "error in arranging test data")

	return event
}
