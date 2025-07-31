package postgresengine_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/core"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shell"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper"
)

func Benchmark_SingleAppend_With_Many_Events_InTheStore(b *testing.B) {
	// setup
	ctx := context.Background()
	wrapper := CreateWrapperWithBenchmarkConfig(b)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	GuardThatThereAreEnoughFixtureEventsInStore(wrapper, 10000)
	fakeClock := GetGreatestOccurredAtTimeFromDB(b, wrapper).Add(time.Second)

	bookID := GivenUniqueID(b)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	b.Run("append 1 event", func(b *testing.B) {
		b.ResetTimer()
		var appendTime time.Duration

		for i := 0; i < b.N; i++ {
			b.StopTimer()
			maxSequenceNumberBeforeAppend := QueryMaxSequenceNumberBeforeAppend(b, ctx, es, filter)

			fakeClock = fakeClock.Add(time.Second)
			event := ToStorable(b, FixtureBookCopyAddedToCirculation(bookID, fakeClock))

			b.StartTimer()
			start := time.Now()
			err := es.Append(
				ctx,
				filter,
				maxSequenceNumberBeforeAppend,
				event,
			)
			appendTime += time.Since(start)
			b.StopTimer()

			assert.NoError(b, err)

			rowsAffected, dbErr := CleanUpBookEvents(wrapper, bookID)
			assert.NoError(b, dbErr)
			assert.Equal(b, int64(1), rowsAffected)

			if i%100 == 0 {
				dbErr = OptimizeDBWhileBenchmarking(wrapper)
				assert.NoError(b, dbErr)
			}
		}

		b.ReportMetric(float64(appendTime.Milliseconds())/float64(b.N), "ms/append-op")
	})
}

func Benchmark_MultipleAppend_With_Many_Events_InTheStore(b *testing.B) {
	// setup
	ctx := context.Background()
	wrapper := CreateWrapperWithBenchmarkConfig(b)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	GuardThatThereAreEnoughFixtureEventsInStore(wrapper, 10000)
	fakeClock := GetGreatestOccurredAtTimeFromDB(b, wrapper).Add(time.Second)

	bookID := GivenUniqueID(b)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	b.Run("append 5 events", func(b *testing.B) {
		b.ResetTimer()
		var appendTime time.Duration

		for i := 0; i < b.N; i++ {
			b.StopTimer()

			maxSequenceNumberBeforeAppend := QueryMaxSequenceNumberBeforeAppend(b, ctx, es, filter)

			fakeClock = fakeClock.Add(time.Second)
			event1 := ToStorable(b, FixtureBookCopyAddedToCirculation(bookID, fakeClock))
			fakeClock = fakeClock.Add(time.Second)
			event2 := ToStorable(b, FixtureBookCopyAddedToCirculation(bookID, fakeClock))
			fakeClock = fakeClock.Add(time.Second)
			event3 := ToStorable(b, FixtureBookCopyAddedToCirculation(bookID, fakeClock))
			fakeClock = fakeClock.Add(time.Second)
			event4 := ToStorable(b, FixtureBookCopyAddedToCirculation(bookID, fakeClock))
			fakeClock = fakeClock.Add(time.Second)
			event5 := ToStorable(b, FixtureBookCopyAddedToCirculation(bookID, fakeClock))

			b.StartTimer()
			start := time.Now()
			err := es.Append(
				ctx,
				filter,
				maxSequenceNumberBeforeAppend,
				event1, event2, event3, event4, event5,
			)
			appendTime += time.Since(start)
			b.StopTimer()

			assert.NoError(b, err)

			rowsAffected, dbErr := CleanUpBookEvents(wrapper, bookID)
			assert.NoError(b, dbErr)
			assert.Equal(b, int64(5), rowsAffected)

			if i%100 == 0 {
				dbErr = OptimizeDBWhileBenchmarking(wrapper)
				assert.NoError(b, dbErr)
			}
		}

		b.ReportMetric(float64(appendTime.Milliseconds())/float64(b.N), "ms/append-op")
	})
}

func Benchmark_Query_With_Many_Events_InTheStore(b *testing.B) {
	// setup
	ctx := context.Background()
	wrapper := CreateWrapperWithBenchmarkConfig(b)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	GuardThatThereAreEnoughFixtureEventsInStore(wrapper, 10000)
	bookID := GetLatestBookIDFromDB(b, wrapper)

	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	b.Run("query", func(b *testing.B) {
		b.ResetTimer()
		var queryTime time.Duration

		for i := 0; i < b.N; i++ {
			b.StartTimer()
			start := time.Now()
			_, _, queryErr := es.Query(ctx, filter)
			queryTime += time.Since(start)
			b.StopTimer()
			assert.NoError(b, queryErr)
		}

		b.ReportMetric(float64(queryTime.Milliseconds())/float64(b.N), "ms/query-op")
	})
}

func Benchmark_TypicalWorkload_With_Many_Events_InTheStore(b *testing.B) {
	// setup
	ctx := context.Background()
	wrapper := CreateWrapperWithBenchmarkConfig(b)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	GuardThatThereAreEnoughFixtureEventsInStore(wrapper, 10000)
	fakeClock := GetGreatestOccurredAtTimeFromDB(b, wrapper).Add(time.Second)

	bookID := GivenUniqueID(b)
	filter := FilterAllEventTypesForOneBook(bookID)

	// act
	b.Run("query decide append", func(b *testing.B) {
		b.ResetTimer()
		var queryTime, appendTime, unmarshalTime, bizTime time.Duration

		for i := 0; i < b.N; i++ {
			b.StopTimer()

			fakeClock = fakeClock.Add(time.Second)
			GivenBookCopyAddedToCirculationWasAppended(b, ctx, es, bookID, fakeClock)

			b.StartTimer()
			start := time.Now()
			storableEvents, maxSequenceNumberBeforeAppend, queryErr := es.Query(ctx, filter)
			queryTime += time.Since(start)
			b.StopTimer()

			assert.NoError(b, queryErr)

			b.StartTimer()
			start = time.Now()
			domainEvents, mappingErr := shell.DomainEventsFrom(storableEvents)
			unmarshalTime += time.Since(start)
			b.StopTimer()
			assert.NoError(b, mappingErr)

			// business logic for this feature/use-case
			b.StartTimer()
			start = time.Now()
			bookExists := false
			for _, domainEvent := range domainEvents {
				switch domainEvent.EventType() {
				case core.BookCopyAddedToCirculationEventType:
					bookExists = true

				case core.BookCopyRemovedFromCirculationEventType:
					bookExists = false
				}
			}
			bizTime += time.Since(start)
			b.StopTimer()

			assert.True(b, bookExists, "book should exist, seems the business logic is wrong")

			fakeClock = fakeClock.Add(time.Second)
			event := ToStorable(b, FixtureBookCopyRemovedFromCirculation(bookID, fakeClock))

			b.StartTimer()
			start = time.Now()

			err := es.Append(
				ctx,
				filter,
				maxSequenceNumberBeforeAppend,
				event,
			)
			appendTime += time.Since(start)
			b.StopTimer()

			assert.NoError(b, err)

			rowsAffected, dbErr := CleanUpBookEvents(wrapper, bookID)
			assert.NoError(b, dbErr)
			assert.Equal(b, int64(2), rowsAffected)

			if i%100 == 0 {
				dbErr = OptimizeDBWhileBenchmarking(wrapper)
				assert.NoError(b, dbErr)
			}
		}

		totalTime := queryTime + appendTime + unmarshalTime + bizTime
		b.ReportMetric(float64(totalTime.Milliseconds())/float64(b.N), "ms/total-op")
		b.ReportMetric(float64(appendTime.Milliseconds())/float64(b.N), "ms/append-op")
		b.ReportMetric(float64(queryTime.Milliseconds())/float64(b.N), "ms/query-op")
	})
}
