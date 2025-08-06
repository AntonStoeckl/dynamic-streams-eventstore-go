package postgresengine_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/features/removebookcopy"
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper"                 //nolint:revive
	. "github.com/AntonStoeckl/dynamic-streams-eventstore-go/testutil/postgresengine/helper/postgreswrapper" //nolint:revive
)

func Benchmark_SingleAppend_With_Many_Events_InTheStore(b *testing.B) {
	// setup
	ctx := context.Background()
	wrapper := CreateWrapperWithBenchmarkConfig(b)
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	// arrange
	GuardThatThereAreEnoughFixtureEventsInStore(wrapper, 1000)
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

			rowsAffected, dbErr := CleanUpBookEvents(ctx, wrapper, bookID)
			assert.NoError(b, dbErr)
			assert.Equal(b, int64(1), rowsAffected)

			if i%100 == 0 {
				dbErr = OptimizeDBWhileBenchmarking(ctx, wrapper)
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
	GuardThatThereAreEnoughFixtureEventsInStore(wrapper, 1000)
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

			rowsAffected, dbErr := CleanUpBookEvents(ctx, wrapper, bookID)
			assert.NoError(b, dbErr)
			assert.Equal(b, int64(5), rowsAffected)

			if i%100 == 0 {
				dbErr = OptimizeDBWhileBenchmarking(ctx, wrapper)
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
	GuardThatThereAreEnoughFixtureEventsInStore(wrapper, 1000)
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

func Benchmark_TypicalWorkload_With_Many_Events_InTheStore(b *testing.B) { //nolint:funlen // Benchmark function requires setup and validation logic
	// setup
	ctx := context.Background()

	metricsSpy := NewMetricsCollectorSpy(true)

	wrapper := CreateWrapperWithBenchmarkConfig(b, postgresengine.WithMetrics(metricsSpy))
	defer wrapper.Close()
	es := wrapper.GetEventStore()

	commandHandler, err := removebookcopy.NewCommandHandler(es, removebookcopy.WithMetrics(metricsSpy))
	assert.NoError(b, err)

	// arrange
	GuardThatThereAreEnoughFixtureEventsInStore(wrapper, 1000)
	fakeClock := GetGreatestOccurredAtTimeFromDB(b, wrapper).Add(time.Second)
	bookID := GivenUniqueID(b)

	// act
	b.Run("query decide append", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			b.StopTimer()

			fakeClock = fakeClock.Add(time.Second)
			GivenBookCopyAddedToCirculationWasAppended(b, ctx, es, bookID, fakeClock)

			fakeClock = fakeClock.Add(time.Second)
			command := removebookcopy.BuildCommand(bookID, fakeClock)

			b.StartTimer()
			handleErr := commandHandler.Handle(ctx, command)
			b.StopTimer()

			assert.NoError(b, handleErr)

			rowsAffected, dbErr := CleanUpBookEvents(ctx, wrapper, bookID)
			assert.NoError(b, dbErr)
			assert.Equal(b, int64(2), rowsAffected)

			if i%100 == 0 {
				dbErr = OptimizeDBWhileBenchmarking(ctx, wrapper)
				assert.NoError(b, dbErr)
			}
		}

		// Validate that metrics were captured
		assert.True(b, metricsSpy.HasDurationRecord("eventstore_query_duration_seconds"), "Missing query duration metrics")
		assert.True(b, metricsSpy.HasDurationRecord("eventstore_append_duration_seconds"), "Missing append duration metrics")
		assert.True(b, metricsSpy.HasDurationRecord("commandhandler_handle_duration_seconds"), "Missing command handler duration metrics")

		// Report average durations from captured metrics
		queryRecords := metricsSpy.GetDurationRecords()
		var totalQueryTime, totalAppendTime, totalCommandTime time.Duration
		var queryCount, appendCount, commandCount int

		for _, record := range queryRecords {
			switch record.Metric {
			case "eventstore_query_duration_seconds":
				totalQueryTime += record.Duration
				queryCount++
			case "eventstore_append_duration_seconds":
				totalAppendTime += record.Duration
				appendCount++
			case "commandhandler_handle_duration_seconds":
				totalCommandTime += record.Duration
				commandCount++
			}
		}

		if queryCount > 0 {
			b.ReportMetric(float64(totalQueryTime.Milliseconds())/float64(queryCount), "ms/query-op")
		}
		if appendCount > 0 {
			b.ReportMetric(float64(totalAppendTime.Milliseconds())/float64(appendCount), "ms/append-op")
		}
		if commandCount > 0 {
			b.ReportMetric(float64(totalCommandTime.Milliseconds())/float64(commandCount), "ms/total-op")
		}
	})
}
