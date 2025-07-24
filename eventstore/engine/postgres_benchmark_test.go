package engine_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"

	. "dynamic-streams-eventstore/eventstore/engine"
	. "dynamic-streams-eventstore/test"
	"dynamic-streams-eventstore/test/userland/config"
	"dynamic-streams-eventstore/test/userland/core"
	"dynamic-streams-eventstore/test/userland/shell"
)

func Benchmark_Append_With_Many_Events_InTheStore(b *testing.B) {
	// setup
	factor := 1000 // multiplied by 1000 -> total num of fixture events
	connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresBenchmarkConfig())
	defer connPool.Close()
	assert.NoError(b, err, "error connecting to DB pool in test setup")

	es := NewPostgresEventStore(connPool)

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	appendFixtureEvents(b, connPool, es, &fakeClock, factor)

	bookID := GivenUniqueID(b)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	b.Run("append", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			b.StopTimer()
			maxSequenceNumberBeforeAppend := QueryMaxSequenceNumberBeforeAppend(b, es, filter)
			b.StartTimer()

			err = es.Append(
				filter,
				maxSequenceNumberBeforeAppend,
				ToStorable(b, FixtureBookCopyAddedToCirculation(bookID, &fakeClock)),
			)
			assert.NoError(b, err, "error in running benchmark action")

			b.StopTimer()

			cmdTag, dbErr := connPool.Exec(
				context.Background(),
				fmt.Sprintf(`DELETE FROM events WHERE payload @> '{"BookID": "%s"}'`, bookID.String()),
			)
			assert.NoError(b, dbErr, "error in cleaning up benchmark artefacts")
			assert.Equal(b, 1, int(cmdTag.RowsAffected()))

			if i%100 == 0 {
				_, dbErr = connPool.Exec(context.Background(), `vacuum analyze events`)
				assert.NoError(b, dbErr, "error in cleaning up benchmark artefacts")
			}

			b.StartTimer()
		}
	})
}

func Benchmark_Query_With_Many_Events_InTheStore(b *testing.B) {
	// setup
	factor := 1000 // multiplied by 1000 -> total num of fixture events
	connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresBenchmarkConfig())
	defer connPool.Close()
	assert.NoError(b, err, "error connecting to DB pool in test setup")

	es := NewPostgresEventStore(connPool)

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	appendFixtureEvents(b, connPool, es, &fakeClock, factor)

	row := connPool.QueryRow(
		context.Background(),
		`select payload->'BookID' as bookID from events where sequence_number = (select max(sequence_number) from events)`,
	)
	var bookIDString string
	err = row.Scan(&bookIDString)
	assert.NoError(b, err, "error in arranging test data")
	bookID, err := uuid.Parse(bookIDString)
	assert.NoError(b, err, "error in arranging test data")

	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	b.Run("query", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			QueryMaxSequenceNumberBeforeAppend(b, es, filter)
		}
	})
}

func Benchmark_TypicalWorkload_With_Many_Events_InTheStore(b *testing.B) {
	// setup
	factor := 1000 // multiplied by 1000 -> total num of fixture events
	connPool, err := pgxpool.NewWithConfig(context.Background(), config.PostgresBenchmarkConfig())
	defer connPool.Close()
	assert.NoError(b, err, "error connecting to DB pool in test setup")

	es := NewPostgresEventStore(connPool)

	fakeClock := time.Unix(0, 0).UTC()

	// arrange
	appendFixtureEvents(b, connPool, es, &fakeClock, factor)

	bookID := GivenUniqueID(b)
	readerID := GivenUniqueID(b)
	filter := FilterAllEventTypesForOneBookOrReader(bookID, readerID)

	// act
	b.Run("append", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			b.StopTimer()

			GivenBookCopyAddedToCirculationWasAppended(b, es, bookID, &fakeClock)
			GivenBookCopyLentToReaderWasAppended(b, es, bookID, readerID, &fakeClock)

			b.StartTimer()

			storableEvents, maxSequenceNumberBeforeAppend, queryErr := es.Query(filter)
			assert.NoError(b, queryErr, "error in running benchmark query")
			domainEvents, mappingErr := shell.DomainEventsFrom(storableEvents)
			assert.NoError(b, mappingErr, "error in mapping events for benchmark")

			// business logic for this feature/use-case
			bookExists := false
			bookCurrentlyLentOutByThisReader := false
			for _, domainEvent := range domainEvents {
				switch domainEvent.EventType() {
				case core.BookCopyAddedToCirculationEventType:
					bookExists = true

				case core.BookCopyRemovedFromCirculationEventType:
					bookExists = false

				case core.BookCopyLentToReaderEventType:
					actualEvent := domainEvent.(core.BookCopyLentToReader)
					bookCurrentlyLentOutByThisReader = actualEvent.ReaderID == readerID.String()

				case core.BookCopyReturnedByReaderEventType:
					actualEvent := domainEvent.(core.BookCopyReturnedByReader)
					bookCurrentlyLentOutByThisReader = actualEvent.ReaderID == readerID.String()
				}
			}

			assert.True(b, bookExists, "book should exist")
			assert.True(b, bookCurrentlyLentOutByThisReader, "book should be lent out by this reader")

			b.StartTimer()

			err = es.Append(
				filter,
				maxSequenceNumberBeforeAppend,
				ToStorable(b, FixtureBookCopyReturnedByReader(bookID, readerID, &fakeClock)),
			)
			assert.NoError(b, err, "error in running benchmark action")

			b.StopTimer()

			cmdTag, dbErr := connPool.Exec(
				context.Background(),
				fmt.Sprintf(`DELETE FROM events WHERE payload @> '{"BookID": "%s"}'`, bookID.String()),
			)
			assert.NoError(b, dbErr, "error in cleaning up benchmark artefacts")
			assert.Equal(b, 3, int(cmdTag.RowsAffected()))

			if i%100 == 0 {
				_, dbErr = connPool.Exec(context.Background(), `vacuum analyze events`)
				assert.NoError(b, dbErr, "error in cleaning up benchmark artefacts")
			}

			b.StartTimer()
		}
	})
}

func appendFixtureEvents(b *testing.B, connPool *pgxpool.Pool, es PostgresEventStore, fakeClock *time.Time, factor int) {
	row := connPool.QueryRow(context.Background(), `SELECT count(*) FROM events`)
	var cnt int
	err := row.Scan(&cnt)
	assert.NoError(b, err, "error in arranging test data")
	//fmt.Printf("found %d events in the DB\n", cnt)

	if cnt < 1000*factor {
		fmt.Println("DomainEvent setup will run")
		CleanUpEvents(b, connPool)
		GivenSomeOtherEventsWereAppended(b, es, 900*factor, 0, fakeClock)

		var totalEvents int
		for i := 0; i < 10*factor; i++ {
			bookID := GivenUniqueID(b)

			for j := 0; j < 5; j++ {
				GivenBookCopyAddedToCirculationWasAppended(b, es, bookID, fakeClock)
				totalEvents++
				GivenBookCopyRemovedFromCirculationWasAppended(b, es, bookID, fakeClock)
				totalEvents++

				if totalEvents%5000 == 0 {
					fmt.Printf("appended %d events into the DB\n", totalEvents)
				}
			}
		}

		//fmt.Printf("appended %d events into the DB\n", totalEvents)
	} else {
		//fmt.Println("DomainEvent setup will NOT run")
	}
}
