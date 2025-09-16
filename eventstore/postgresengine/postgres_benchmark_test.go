package postgresengine_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/pgtesthelpers"
)

func Benchmark_SingleAppend_With_Many_Events_InTheStore(b *testing.B) {
	// setup
	ctx := context.Background()
	wrapper := pgtesthelpers.CreateWrapperWithBenchmarkConfig(b)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	pgtesthelpers.GuardThatThereAreEnoughFixtureEventsInStore(wrapper, 1000)
	fakeClock := pgtesthelpers.GetGreatestOccurredAtTimeFromDB(b, wrapper).Add(time.Second)

	bookID := helper.GivenUniqueID(b)
	filter := helper.FilterAllEventTypesForOneBook(bookID)

	// act
	b.Run("append 1 event", func(b *testing.B) {
		b.ResetTimer()
		var appendTime time.Duration

		for i := 0; i < b.N; i++ {
			b.StopTimer()
			maxSequenceNumberBeforeAppend := helper.QueryMaxSequenceNumberBeforeAppend(b, ctx, es, filter)

			fakeClock = fakeClock.Add(time.Second)
			event := helper.ToStorable(b, helper.FixtureBookCopyAddedToCirculation(bookID, fakeClock))

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

			rowsAffected, dbErr := pgtesthelpers.CleanUpBookEvents(ctx, wrapper, bookID)
			assert.NoError(b, dbErr)
			assert.Equal(b, int64(1), rowsAffected)

			if i%100 == 0 {
				dbErr = pgtesthelpers.OptimizeDBWhileBenchmarking(ctx, wrapper)
				assert.NoError(b, dbErr)
			}
		}

		b.ReportMetric(float64(appendTime.Milliseconds())/float64(b.N), "ms/append-op")
	})
}

func Benchmark_MultipleAppend_With_Many_Events_InTheStore(b *testing.B) {
	// setup
	ctx := context.Background()
	wrapper := pgtesthelpers.CreateWrapperWithBenchmarkConfig(b)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	pgtesthelpers.GuardThatThereAreEnoughFixtureEventsInStore(wrapper, 1000)
	fakeClock := pgtesthelpers.GetGreatestOccurredAtTimeFromDB(b, wrapper).Add(time.Second)

	bookID := helper.GivenUniqueID(b)
	filter := helper.FilterAllEventTypesForOneBook(bookID)

	// act
	b.Run("append 5 events", func(b *testing.B) {
		b.ResetTimer()
		var appendTime time.Duration

		for i := 0; i < b.N; i++ {
			b.StopTimer()

			maxSequenceNumberBeforeAppend := helper.QueryMaxSequenceNumberBeforeAppend(b, ctx, es, filter)

			fakeClock = fakeClock.Add(time.Second)
			event1 := helper.ToStorable(b, helper.FixtureBookCopyAddedToCirculation(bookID, fakeClock))
			fakeClock = fakeClock.Add(time.Second)
			event2 := helper.ToStorable(b, helper.FixtureBookCopyAddedToCirculation(bookID, fakeClock))
			fakeClock = fakeClock.Add(time.Second)
			event3 := helper.ToStorable(b, helper.FixtureBookCopyAddedToCirculation(bookID, fakeClock))
			fakeClock = fakeClock.Add(time.Second)
			event4 := helper.ToStorable(b, helper.FixtureBookCopyAddedToCirculation(bookID, fakeClock))
			fakeClock = fakeClock.Add(time.Second)
			event5 := helper.ToStorable(b, helper.FixtureBookCopyAddedToCirculation(bookID, fakeClock))

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

			rowsAffected, dbErr := pgtesthelpers.CleanUpBookEvents(ctx, wrapper, bookID)
			assert.NoError(b, dbErr)
			assert.Equal(b, int64(5), rowsAffected)

			if i%100 == 0 {
				dbErr = pgtesthelpers.OptimizeDBWhileBenchmarking(ctx, wrapper)
				assert.NoError(b, dbErr)
			}
		}

		b.ReportMetric(float64(appendTime.Milliseconds())/float64(b.N), "ms/append-op")
	})
}

func Benchmark_Query_With_Many_Events_InTheStore(b *testing.B) {
	// setup
	ctx := context.Background()
	wrapper := pgtesthelpers.CreateWrapperWithBenchmarkConfig(b)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	pgtesthelpers.GuardThatThereAreEnoughFixtureEventsInStore(wrapper, 1000)
	bookID := pgtesthelpers.GetLatestBookIDFromDB(b, wrapper)

	filter := helper.FilterAllEventTypesForOneBook(bookID)

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
