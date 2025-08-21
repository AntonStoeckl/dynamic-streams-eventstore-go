package eventstore_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

//nolint:funlen
func Test_FilterBuilder_ValidCombinations(t *testing.T) {
	tests := []struct {
		name     string
		build    func() eventstore.Filter
		validate func(t *testing.T, filter eventstore.Filter)
	}{
		{
			name: "matching_any_event_creates_empty_filter",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().MatchingAnyEvent()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Empty(t, f.Items())
				assert.True(t, f.OccurredFrom().IsZero())
				assert.True(t, f.OccurredUntil().IsZero())
				assert.Equal(t, uint(0), f.SequenceNumberHigherThan())
			},
		},
		{
			name: "sequence_only_filter",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					WithSequenceNumberHigherThan(12345).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Equal(t, uint(12345), f.SequenceNumberHigherThan())
				assert.True(t, f.OccurredFrom().IsZero())
				assert.True(t, f.OccurredUntil().IsZero())
				assert.Len(t, f.Items(), 1)
				assert.Empty(t, f.Items()[0].EventTypes())
				assert.Empty(t, f.Items()[0].Predicates())
			},
		},
		{
			name: "occurred_from_only_filter",
			build: func() eventstore.Filter {
				timeFrom := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
				return eventstore.BuildEventFilter().
					OccurredFrom(timeFrom).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				expectedTime := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
				assert.Equal(t, expectedTime, f.OccurredFrom())
				assert.True(t, f.OccurredUntil().IsZero())
				assert.Equal(t, uint(0), f.SequenceNumberHigherThan())
				assert.Len(t, f.Items(), 1)
				assert.Empty(t, f.Items()[0].EventTypes())
				assert.Empty(t, f.Items()[0].Predicates())
			},
		},
		{
			name: "occurred_until_only_filter",
			build: func() eventstore.Filter {
				timeUntil := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)
				return eventstore.BuildEventFilter().
					OccurredUntil(timeUntil).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				expectedTime := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)
				assert.True(t, f.OccurredFrom().IsZero())
				assert.Equal(t, expectedTime, f.OccurredUntil())
				assert.Equal(t, uint(0), f.SequenceNumberHigherThan())
				assert.Len(t, f.Items(), 1)
				assert.Empty(t, f.Items()[0].EventTypes())
				assert.Empty(t, f.Items()[0].Predicates())
			},
		},
		{
			name: "occurred_from_and_until_filter",
			build: func() eventstore.Filter {
				timeFrom := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
				timeUntil := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)
				return eventstore.BuildEventFilter().
					OccurredFrom(timeFrom).
					AndOccurredUntil(timeUntil).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				expectedFrom := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
				expectedUntil := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)
				assert.Equal(t, expectedFrom, f.OccurredFrom())
				assert.Equal(t, expectedUntil, f.OccurredUntil())
				assert.Equal(t, uint(0), f.SequenceNumberHigherThan())
				assert.Len(t, f.Items(), 1)
				assert.Empty(t, f.Items()[0].EventTypes())
				assert.Empty(t, f.Items()[0].Predicates())
			},
		},
		{
			name: "single_event_type_filter",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("BookCopyLentToReader").
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.True(t, f.OccurredFrom().IsZero())
				assert.True(t, f.OccurredUntil().IsZero())
				assert.Equal(t, uint(0), f.SequenceNumberHigherThan())
				assert.Len(t, f.Items(), 1)
				assert.Equal(t, []string{"BookCopyLentToReader"}, f.Items()[0].EventTypes())
				assert.Empty(t, f.Items()[0].Predicates())
				assert.False(t, f.Items()[0].AllPredicatesMustMatch())
			},
		},
		{
			name: "multiple_event_types_filter",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("BookCopyLentToReader", "BookCopyReturnedByReader").
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Len(t, f.Items(), 1)
				assert.Equal(t, []string{"BookCopyLentToReader", "BookCopyReturnedByReader"}, f.Items()[0].EventTypes())
				assert.Empty(t, f.Items()[0].Predicates())
				assert.False(t, f.Items()[0].AllPredicatesMustMatch())
			},
		},
		{
			name: "single_predicate_any_filter",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyPredicateOf(eventstore.P("BookID", "book-123")).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Len(t, f.Items(), 1)
				assert.Empty(t, f.Items()[0].EventTypes())
				assert.Len(t, f.Items()[0].Predicates(), 1)
				assert.Equal(t, "BookID", f.Items()[0].Predicates()[0].Key())
				assert.Equal(t, "book-123", f.Items()[0].Predicates()[0].Val())
				assert.False(t, f.Items()[0].AllPredicatesMustMatch())
			},
		},
		{
			name: "single_predicate_all_filter",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AllPredicatesOf(eventstore.P("ReaderID", "reader-456")).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Len(t, f.Items(), 1)
				assert.Empty(t, f.Items()[0].EventTypes())
				assert.Len(t, f.Items()[0].Predicates(), 1)
				assert.Equal(t, "ReaderID", f.Items()[0].Predicates()[0].Key())
				assert.Equal(t, "reader-456", f.Items()[0].Predicates()[0].Val())
				assert.True(t, f.Items()[0].AllPredicatesMustMatch())
			},
		},
		{
			name: "multiple_predicates_any_filter",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyPredicateOf(
						eventstore.P("BookID", "book-123"),
						eventstore.P("ReaderID", "reader-456")).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Len(t, f.Items(), 1)
				assert.Empty(t, f.Items()[0].EventTypes())
				assert.Len(t, f.Items()[0].Predicates(), 2)
				assert.Equal(t, "BookID", f.Items()[0].Predicates()[0].Key())
				assert.Equal(t, "book-123", f.Items()[0].Predicates()[0].Val())
				assert.Equal(t, "ReaderID", f.Items()[0].Predicates()[1].Key())
				assert.Equal(t, "reader-456", f.Items()[0].Predicates()[1].Val())
				assert.False(t, f.Items()[0].AllPredicatesMustMatch())
			},
		},
		{
			name: "multiple_predicates_all_filter",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AllPredicatesOf(
						eventstore.P("BookID", "book-123"),
						eventstore.P("Status", "active")).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Len(t, f.Items(), 1)
				assert.Empty(t, f.Items()[0].EventTypes())
				assert.Len(t, f.Items()[0].Predicates(), 2)
				assert.Equal(t, "BookID", f.Items()[0].Predicates()[0].Key())
				assert.Equal(t, "book-123", f.Items()[0].Predicates()[0].Val())
				assert.Equal(t, "Status", f.Items()[0].Predicates()[1].Key())
				assert.Equal(t, "active", f.Items()[0].Predicates()[1].Val())
				assert.True(t, f.Items()[0].AllPredicatesMustMatch())
			},
		},
		{
			name: "event_types_and_predicates_any",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("BookCopyLentToReader").
					AndAnyPredicateOf(eventstore.P("ReaderID", "reader-123")).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Len(t, f.Items(), 1)
				assert.Equal(t, []string{"BookCopyLentToReader"}, f.Items()[0].EventTypes())
				assert.Len(t, f.Items()[0].Predicates(), 1)
				assert.Equal(t, "ReaderID", f.Items()[0].Predicates()[0].Key())
				assert.Equal(t, "reader-123", f.Items()[0].Predicates()[0].Val())
				assert.False(t, f.Items()[0].AllPredicatesMustMatch())
			},
		},
		{
			name: "event_types_and_predicates_all",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("BookCopyLentToReader", "BookCopyReturnedByReader").
					AndAllPredicatesOf(
						eventstore.P("BookID", "book-123"),
						eventstore.P("ReaderID", "reader-456")).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Len(t, f.Items(), 1)
				assert.Equal(t, []string{"BookCopyLentToReader", "BookCopyReturnedByReader"}, f.Items()[0].EventTypes())
				assert.Len(t, f.Items()[0].Predicates(), 2)
				assert.Equal(t, "BookID", f.Items()[0].Predicates()[0].Key())
				assert.Equal(t, "book-123", f.Items()[0].Predicates()[0].Val())
				assert.Equal(t, "ReaderID", f.Items()[0].Predicates()[1].Key())
				assert.Equal(t, "reader-456", f.Items()[0].Predicates()[1].Val())
				assert.True(t, f.Items()[0].AllPredicatesMustMatch())
			},
		},
		{
			name: "predicates_then_event_types",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyPredicateOf(eventstore.P("BookID", "book-789")).
					AndAnyEventTypeOf("BookCopyAddedToCirculation").
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Len(t, f.Items(), 1)
				assert.Equal(t, []string{"BookCopyAddedToCirculation"}, f.Items()[0].EventTypes())
				assert.Len(t, f.Items()[0].Predicates(), 1)
				assert.Equal(t, "BookID", f.Items()[0].Predicates()[0].Key())
				assert.Equal(t, "book-789", f.Items()[0].Predicates()[0].Val())
				assert.False(t, f.Items()[0].AllPredicatesMustMatch())
			},
		},
		{
			name: "event_types_with_time_boundaries",
			build: func() eventstore.Filter {
				timeFrom := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
				timeUntil := time.Date(2025, 6, 30, 18, 0, 0, 0, time.UTC)
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("ReaderRegistered").
					OccurredFrom(timeFrom).
					AndOccurredUntil(timeUntil).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				expectedFrom := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
				expectedUntil := time.Date(2025, 6, 30, 18, 0, 0, 0, time.UTC)
				assert.Equal(t, expectedFrom, f.OccurredFrom())
				assert.Equal(t, expectedUntil, f.OccurredUntil())
				assert.Equal(t, uint(0), f.SequenceNumberHigherThan())
				assert.Len(t, f.Items(), 1)
				assert.Equal(t, []string{"ReaderRegistered"}, f.Items()[0].EventTypes())
				assert.Empty(t, f.Items()[0].Predicates())
			},
		},
		{
			name: "predicates_with_sequence_boundary",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AllPredicatesOf(eventstore.P("Status", "cancelled")).
					WithSequenceNumberHigherThan(9876).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.True(t, f.OccurredFrom().IsZero())
				assert.True(t, f.OccurredUntil().IsZero())
				assert.Equal(t, uint(9876), f.SequenceNumberHigherThan())
				assert.Len(t, f.Items(), 1)
				assert.Empty(t, f.Items()[0].EventTypes())
				assert.Len(t, f.Items()[0].Predicates(), 1)
				assert.Equal(t, "Status", f.Items()[0].Predicates()[0].Key())
				assert.Equal(t, "cancelled", f.Items()[0].Predicates()[0].Val())
				assert.True(t, f.Items()[0].AllPredicatesMustMatch())
			},
		},
		{
			name: "complex_combination_with_time",
			build: func() eventstore.Filter {
				timeFrom := time.Date(2025, 3, 15, 9, 30, 0, 0, time.UTC)
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("BookCopyLentToReader", "BookCopyReturnedByReader").
					AndAllPredicatesOf(
						eventstore.P("BookID", "book-complex"),
						eventstore.P("LibraryBranch", "main")).
					OccurredFrom(timeFrom).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				expectedFrom := time.Date(2025, 3, 15, 9, 30, 0, 0, time.UTC)
				assert.Equal(t, expectedFrom, f.OccurredFrom())
				assert.True(t, f.OccurredUntil().IsZero())
				assert.Equal(t, uint(0), f.SequenceNumberHigherThan())
				assert.Len(t, f.Items(), 1)
				assert.Equal(t, []string{"BookCopyLentToReader", "BookCopyReturnedByReader"}, f.Items()[0].EventTypes())
				assert.Len(t, f.Items()[0].Predicates(), 2)
				assert.Equal(t, "BookID", f.Items()[0].Predicates()[0].Key())
				assert.Equal(t, "book-complex", f.Items()[0].Predicates()[0].Val())
				assert.Equal(t, "LibraryBranch", f.Items()[0].Predicates()[1].Key())
				assert.Equal(t, "main", f.Items()[0].Predicates()[1].Val())
				assert.True(t, f.Items()[0].AllPredicatesMustMatch())
			},
		},
		{
			name: "multiple_filter_items_with_or_matching",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("BookCopyLentToReader").
					AndAnyPredicateOf(eventstore.P("ReaderID", "reader-1")).
					OrMatching().
					AnyEventTypeOf("BookCopyReturnedByReader").
					AndAnyPredicateOf(eventstore.P("ReaderID", "reader-2")).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.True(t, f.OccurredFrom().IsZero())
				assert.True(t, f.OccurredUntil().IsZero())
				assert.Equal(t, uint(0), f.SequenceNumberHigherThan())
				assert.Len(t, f.Items(), 2)

				// First FilterItem
				assert.Equal(t, []string{"BookCopyLentToReader"}, f.Items()[0].EventTypes())
				assert.Len(t, f.Items()[0].Predicates(), 1)
				assert.Equal(t, "ReaderID", f.Items()[0].Predicates()[0].Key())
				assert.Equal(t, "reader-1", f.Items()[0].Predicates()[0].Val())
				assert.False(t, f.Items()[0].AllPredicatesMustMatch())

				// Second FilterItem
				assert.Equal(t, []string{"BookCopyReturnedByReader"}, f.Items()[1].EventTypes())
				assert.Len(t, f.Items()[1].Predicates(), 1)
				assert.Equal(t, "ReaderID", f.Items()[1].Predicates()[0].Key())
				assert.Equal(t, "reader-2", f.Items()[1].Predicates()[0].Val())
				assert.False(t, f.Items()[1].AllPredicatesMustMatch())
			},
		},
		{
			name: "three_filter_items_with_different_patterns",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("EventA").
					OrMatching().
					AnyPredicateOf(eventstore.P("Type", "special")).
					OrMatching().
					AllPredicatesOf(
						eventstore.P("Category", "urgent"),
						eventstore.P("Priority", "high")).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Len(t, f.Items(), 3)

				// First FilterItem: only event types
				assert.Equal(t, []string{"EventA"}, f.Items()[0].EventTypes())
				assert.Empty(t, f.Items()[0].Predicates())
				assert.False(t, f.Items()[0].AllPredicatesMustMatch())

				// Second FilterItem: only predicates (ANY)
				assert.Empty(t, f.Items()[1].EventTypes())
				assert.Len(t, f.Items()[1].Predicates(), 1)
				assert.Equal(t, "Type", f.Items()[1].Predicates()[0].Key())
				assert.Equal(t, "special", f.Items()[1].Predicates()[0].Val())
				assert.False(t, f.Items()[1].AllPredicatesMustMatch())

				// Third FilterItem: only predicates (ALL)
				assert.Empty(t, f.Items()[2].EventTypes())
				assert.Len(t, f.Items()[2].Predicates(), 2)
				assert.Equal(t, "Category", f.Items()[2].Predicates()[0].Key())
				assert.Equal(t, "urgent", f.Items()[2].Predicates()[0].Val())
				assert.Equal(t, "Priority", f.Items()[2].Predicates()[1].Key())
				assert.Equal(t, "high", f.Items()[2].Predicates()[1].Val())
				assert.True(t, f.Items()[2].AllPredicatesMustMatch())
			},
		},
		{
			name: "multiple_filter_items_with_sequence_boundary",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("EventX").
					AndAnyPredicateOf(eventstore.P("X", "1")).
					OrMatching().
					AnyEventTypeOf("EventY").
					AndAnyPredicateOf(eventstore.P("Y", "2")).
					WithSequenceNumberHigherThan(5555).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.True(t, f.OccurredFrom().IsZero())
				assert.True(t, f.OccurredUntil().IsZero())
				assert.Equal(t, uint(5555), f.SequenceNumberHigherThan())
				assert.Len(t, f.Items(), 2)

				// First FilterItem
				assert.Equal(t, []string{"EventX"}, f.Items()[0].EventTypes())
				assert.Len(t, f.Items()[0].Predicates(), 1)
				assert.Equal(t, "X", f.Items()[0].Predicates()[0].Key())
				assert.Equal(t, "1", f.Items()[0].Predicates()[0].Val())

				// Second FilterItem
				assert.Equal(t, []string{"EventY"}, f.Items()[1].EventTypes())
				assert.Len(t, f.Items()[1].Predicates(), 1)
				assert.Equal(t, "Y", f.Items()[1].Predicates()[0].Key())
				assert.Equal(t, "2", f.Items()[1].Predicates()[0].Val())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := tt.build()
			tt.validate(t, filter)
		})
	}
}

//nolint:funlen
func Test_FilterBuilder_InputSanitization(t *testing.T) {
	tests := []struct {
		name     string
		build    func() eventstore.Filter
		validate func(t *testing.T, filter eventstore.Filter)
	}{
		{
			name: "empty_event_types_are_removed",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("", "ValidEvent", "", "AnotherEvent", "").
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Len(t, f.Items(), 1)
				assert.Equal(t, []string{"AnotherEvent", "ValidEvent"}, f.Items()[0].EventTypes())
			},
		},
		{
			name: "duplicate_event_types_are_removed_and_sorted",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("EventZ", "EventA", "EventZ", "EventB", "EventA").
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Len(t, f.Items(), 1)
				assert.Equal(t, []string{"EventA", "EventB", "EventZ"}, f.Items()[0].EventTypes())
			},
		},
		{
			name: "empty_predicates_are_removed",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyPredicateOf(
						eventstore.P("", "value1"), // empty key
						eventstore.P("key2", ""),   // empty value
						eventstore.P("ValidKey", "ValidValue"),
						eventstore.P("", ""), // both empty
						eventstore.P("AnotherKey", "AnotherValue")).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Len(t, f.Items(), 1)
				assert.Len(t, f.Items()[0].Predicates(), 2)
				assert.Equal(t, "AnotherKey", f.Items()[0].Predicates()[0].Key())
				assert.Equal(t, "AnotherValue", f.Items()[0].Predicates()[0].Val())
				assert.Equal(t, "ValidKey", f.Items()[0].Predicates()[1].Key())
				assert.Equal(t, "ValidValue", f.Items()[0].Predicates()[1].Val())
			},
		},
		{
			name: "duplicate_predicates_are_removed_and_sorted_by_key",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AllPredicatesOf(
						eventstore.P("ZKey", "value1"),
						eventstore.P("AKey", "value2"),
						eventstore.P("ZKey", "value1"), // duplicate
						eventstore.P("BKey", "value3"),
						eventstore.P("AKey", "value2")). // duplicate
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Len(t, f.Items(), 1)
				assert.Len(t, f.Items()[0].Predicates(), 3)
				// Should be sorted by key
				assert.Equal(t, "AKey", f.Items()[0].Predicates()[0].Key())
				assert.Equal(t, "value2", f.Items()[0].Predicates()[0].Val())
				assert.Equal(t, "BKey", f.Items()[0].Predicates()[1].Key())
				assert.Equal(t, "value3", f.Items()[0].Predicates()[1].Val())
				assert.Equal(t, "ZKey", f.Items()[0].Predicates()[2].Key())
				assert.Equal(t, "value1", f.Items()[0].Predicates()[2].Val())
				assert.True(t, f.Items()[0].AllPredicatesMustMatch())
			},
		},
		{
			name: "combined_sanitization_event_types_and_predicates",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("", "EventB", "EventA", "", "EventB"). // empty and duplicates
					AndAnyPredicateOf(
						eventstore.P("", "invalid"), // empty key
						eventstore.P("Key2", "val2"),
						eventstore.P("Key1", "val1"),
						eventstore.P("Key2", "val2")). // duplicate
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Len(t, f.Items(), 1)
				// Event types should be cleaned and sorted
				assert.Equal(t, []string{"EventA", "EventB"}, f.Items()[0].EventTypes())
				// Predicates should be cleaned and sorted
				assert.Len(t, f.Items()[0].Predicates(), 2)
				assert.Equal(t, "Key1", f.Items()[0].Predicates()[0].Key())
				assert.Equal(t, "val1", f.Items()[0].Predicates()[0].Val())
				assert.Equal(t, "Key2", f.Items()[0].Predicates()[1].Key())
				assert.Equal(t, "val2", f.Items()[0].Predicates()[1].Val())
				assert.False(t, f.Items()[0].AllPredicatesMustMatch())
			},
		},
		{
			name: "all_empty_event_types_results_in_empty_list",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("", "", "").
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Len(t, f.Items(), 1)
				assert.Empty(t, f.Items()[0].EventTypes())
			},
		},
		{
			name: "all_empty_predicates_results_in_empty_list",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyPredicateOf(
						eventstore.P("", "val"),
						eventstore.P("key", ""),
						eventstore.P("", "")).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Len(t, f.Items(), 1)
				assert.Empty(t, f.Items()[0].Predicates())
			},
		},
		{
			name: "zero_sequence_number_boundary_test",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					WithSequenceNumberHigherThan(0).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Equal(t, uint(0), f.SequenceNumberHigherThan())
			},
		},
		{
			name: "small_sequence_number_boundary_test",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					WithSequenceNumberHigherThan(1).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Equal(t, uint(1), f.SequenceNumberHigherThan())
			},
		},
		{
			name: "positive_sequence_numbers_unchanged",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					WithSequenceNumberHigherThan(123).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Equal(t, uint(123), f.SequenceNumberHigherThan())
			},
		},
		{
			name: "zero_sequence_number_unchanged",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					WithSequenceNumberHigherThan(0).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Equal(t, uint(0), f.SequenceNumberHigherThan())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := tt.build()
			tt.validate(t, filter)
		})
	}
}

//nolint:funlen
func Test_FilterBuilder_MutualExclusion(t *testing.T) {
	tests := []struct {
		name     string
		build    func() eventstore.Filter
		validate func(t *testing.T, filter eventstore.Filter)
	}{
		{
			name: "time_boundaries_exclude_sequence_number",
			build: func() eventstore.Filter {
				timeFrom := time.Date(2025, 4, 1, 14, 30, 0, 0, time.UTC)
				return eventstore.BuildEventFilter().
					OccurredFrom(timeFrom).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				expectedTime := time.Date(2025, 4, 1, 14, 30, 0, 0, time.UTC)
				assert.Equal(t, expectedTime, f.OccurredFrom())
				assert.True(t, f.OccurredUntil().IsZero())
				assert.Equal(t, uint(0), f.SequenceNumberHigherThan()) // Should remain zero
			},
		},
		{
			name: "sequence_boundary_excludes_time_boundaries",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					WithSequenceNumberHigherThan(7890).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Equal(t, uint(7890), f.SequenceNumberHigherThan())
				assert.True(t, f.OccurredFrom().IsZero())  // Should remain zero
				assert.True(t, f.OccurredUntil().IsZero()) // Should remain zero
			},
		},
		{
			name: "complex_filter_with_time_boundaries_no_sequence",
			build: func() eventstore.Filter {
				timeFrom := time.Date(2025, 8, 1, 9, 0, 0, 0, time.UTC)
				timeUntil := time.Date(2025, 8, 31, 17, 0, 0, 0, time.UTC)
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("TestEvent").
					AndAllPredicatesOf(eventstore.P("TestKey", "TestValue")).
					OccurredFrom(timeFrom).
					AndOccurredUntil(timeUntil).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				expectedFrom := time.Date(2025, 8, 1, 9, 0, 0, 0, time.UTC)
				expectedUntil := time.Date(2025, 8, 31, 17, 0, 0, 0, time.UTC)
				assert.Equal(t, expectedFrom, f.OccurredFrom())
				assert.Equal(t, expectedUntil, f.OccurredUntil())
				assert.Equal(t, uint(0), f.SequenceNumberHigherThan()) // Should remain zero
			},
		},
		{
			name: "complex_filter_with_sequence_boundary_no_time",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("TestEvent1", "TestEvent2").
					AndAnyPredicateOf(
						eventstore.P("Key1", "Val1"),
						eventstore.P("Key2", "Val2")).
					WithSequenceNumberHigherThan(11111).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Equal(t, uint(11111), f.SequenceNumberHigherThan())
				assert.True(t, f.OccurredFrom().IsZero())  // Should remain zero
				assert.True(t, f.OccurredUntil().IsZero()) // Should remain zero
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := tt.build()
			tt.validate(t, filter)
		})
	}
}

//nolint:funlen
func Test_FilterBuilder_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		build    func() eventstore.Filter
		validate func(t *testing.T, filter eventstore.Filter)
	}{
		{
			name: "zero_sequence_number_boundary",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					WithSequenceNumberHigherThan(0).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Equal(t, uint(0), f.SequenceNumberHigherThan())
				assert.True(t, f.OccurredFrom().IsZero())
				assert.True(t, f.OccurredUntil().IsZero())
			},
		},
		{
			name: "small_sequence_number_preserved",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					WithSequenceNumberHigherThan(100).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Equal(t, uint(100), f.SequenceNumberHigherThan())
				assert.True(t, f.OccurredFrom().IsZero())
				assert.True(t, f.OccurredUntil().IsZero())
			},
		},
		{
			name: "minimal_sequence_number_preserved",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					WithSequenceNumberHigherThan(1).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Equal(t, uint(1), f.SequenceNumberHigherThan())
				assert.True(t, f.OccurredFrom().IsZero())
				assert.True(t, f.OccurredUntil().IsZero())
			},
		},
		{
			name: "large_sequence_number_boundary",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					WithSequenceNumberHigherThan(9223372036854775807). // max int64
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Equal(t, uint(9223372036854775807), f.SequenceNumberHigherThan())
				assert.True(t, f.OccurredFrom().IsZero())
				assert.True(t, f.OccurredUntil().IsZero())
			},
		},
		{
			name: "zero_time_boundaries_explicitly_set",
			build: func() eventstore.Filter {
				zeroTime := time.Time{}
				return eventstore.BuildEventFilter().
					OccurredFrom(zeroTime).
					AndOccurredUntil(zeroTime).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.True(t, f.OccurredFrom().IsZero())
				assert.True(t, f.OccurredUntil().IsZero())
				assert.Equal(t, uint(0), f.SequenceNumberHigherThan())
			},
		},
		{
			name: "single_character_event_type",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("A").
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Len(t, f.Items(), 1)
				assert.Equal(t, []string{"A"}, f.Items()[0].EventTypes())
			},
		},
		{
			name: "single_character_predicate_key_and_value",
			build: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyPredicateOf(eventstore.P("K", "V")).
					Finalize()
			},
			validate: func(t *testing.T, f eventstore.Filter) {
				assert.Len(t, f.Items(), 1)
				assert.Len(t, f.Items()[0].Predicates(), 1)
				assert.Equal(t, "K", f.Items()[0].Predicates()[0].Key())
				assert.Equal(t, "V", f.Items()[0].Predicates()[0].Val())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := tt.build()
			tt.validate(t, filter)
		})
	}
}

//nolint:funlen
func Test_FilterBuilder_InterfaceConstraints(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "build_event_filter_returns_filter_builder_interface",
			test: func(t *testing.T) {
				rootBuilder := eventstore.BuildEventFilter()

				assert.Implements(t, (*eventstore.FilterBuilder)(nil), rootBuilder)
			},
		},
		{
			name: "matching_returns_empty_filter_item_builder_interface",
			test: func(t *testing.T) {
				emptyBuilder := eventstore.BuildEventFilter().Matching()

				assert.Implements(t, (*eventstore.EmptyFilterItemBuilder)(nil), emptyBuilder)
			},
		},
		{
			name: "or_matching_returns_empty_filter_item_builder_interface",
			test: func(t *testing.T) {
				orBuilder := eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("Event1").
					OrMatching()

				assert.Implements(t, (*eventstore.EmptyFilterItemBuilder)(nil), orBuilder)
			},
		},
		{
			name: "with_sequence_number_returns_sequence_only_interface",
			test: func(t *testing.T) {
				sequenceBuilder := eventstore.BuildEventFilter().
					WithSequenceNumberHigherThan(123)

				assert.Implements(t, (*eventstore.CompletedFilterItemBuilderWithSequenceNumber)(nil), sequenceBuilder)
			},
		},
		{
			name: "occurred_from_returns_time_boundary_interface",
			test: func(t *testing.T) {
				timeFrom := time.Date(2025, 5, 1, 10, 0, 0, 0, time.UTC)
				timeBuilder := eventstore.BuildEventFilter().
					OccurredFrom(timeFrom)

				assert.Implements(t, (*eventstore.CompletedFilterItemBuilderWithOccurredFrom)(nil), timeBuilder)
			},
		},
		{
			name: "occurred_until_returns_finalize_only_interface",
			test: func(t *testing.T) {
				timeUntil := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)
				untilBuilder := eventstore.BuildEventFilter().
					OccurredUntil(timeUntil)

				assert.Implements(t, (*eventstore.CompletedFilterItemBuilderWithOccurredUntil)(nil), untilBuilder)
			},
		},
		{
			name: "occurred_from_and_until_returns_finalize_only_interface",
			test: func(t *testing.T) {
				timeFrom := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
				timeUntil := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)
				rangeBuilder := eventstore.BuildEventFilter().
					OccurredFrom(timeFrom).
					AndOccurredUntil(timeUntil)

				assert.Implements(t, (*eventstore.CompletedFilterItemBuilderWithOccurredFromToUntil)(nil), rangeBuilder)
			},
		},
		{
			name: "filter_item_builder_lacking_predicates_interface",
			test: func(t *testing.T) {
				eventTypeBuilder := eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("TestEvent")

				assert.Implements(t, (*eventstore.FilterItemBuilderLackingPredicates)(nil), eventTypeBuilder)
			},
		},
		{
			name: "filter_item_builder_lacking_event_types_interface",
			test: func(t *testing.T) {
				predicateBuilder := eventstore.BuildEventFilter().
					Matching().
					AnyPredicateOf(eventstore.P("Key", "Value"))

				assert.Implements(t, (*eventstore.FilterItemBuilderLackingEventTypes)(nil), predicateBuilder)
			},
		},
		{
			name: "completed_filter_item_builder_interface",
			test: func(t *testing.T) {
				completedBuilder := eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("Event1").
					AndAnyPredicateOf(eventstore.P("Key1", "Val1"))

				assert.Implements(t, (*eventstore.CompletedFilterItemBuilder)(nil), completedBuilder)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.test(t)
		})
	}
}

//nolint:funlen
func Test_Filter_Hash_Deterministic(t *testing.T) {
	tests := []struct {
		name   string
		filter func() eventstore.Filter
	}{
		{
			name: "simple_event_type_filter",
			filter: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("TestEvent").
					Finalize()
			},
		},
		{
			name: "multiple_event_types",
			filter: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("EventA", "EventB").
					Finalize()
			},
		},
		{
			name: "event_type_with_predicates",
			filter: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("TestEvent").
					AndAnyPredicateOf(eventstore.P("Key1", "Value1")).
					Finalize()
			},
		},
		{
			name: "complex_filter_with_time_boundaries",
			filter: func() eventstore.Filter {
				timeFrom := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
				timeUntil := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("EventA", "EventB").
					AndAllPredicatesOf(
						eventstore.P("Key1", "Value1"),
						eventstore.P("Key2", "Value2")).
					OccurredFrom(timeFrom).
					AndOccurredUntil(timeUntil).
					Finalize()
			},
		},
		{
			name: "filter_with_sequence_boundary",
			filter: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("TestEvent").
					WithSequenceNumberHigherThan(12345).
					Finalize()
			},
		},
		{
			name: "multiple_filter_items",
			filter: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("EventA").
					AndAnyPredicateOf(eventstore.P("Key1", "Value1")).
					OrMatching().
					AnyEventTypeOf("EventB").
					AndAllPredicatesOf(eventstore.P("Key2", "Value2")).
					Finalize()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := tt.filter()

			// Generate hash multiple times
			hash1 := filter.Hash()
			hash2 := filter.Hash()
			hash3 := filter.Hash()

			// All hashes should be identical
			assert.Equal(t, hash1, hash2, "Hash should be deterministic")
			assert.Equal(t, hash1, hash3, "Hash should be deterministic")

			// Hash should not be empty
			assert.NotEmpty(t, hash1, "Hash should not be empty")

			// Hash should start with sha256: prefix
			assert.Contains(t, hash1, "sha256:", "Hash should have sha256 prefix")

			// Hash should be a reasonable length (64 hex chars and prefix)
			assert.Len(t, hash1, len("sha256:")+64, "Hash should be correct length")
		})
	}
}

//nolint:funlen
func Test_Filter_Hash_DifferentFilters_DifferentHashes(t *testing.T) {
	tests := []struct {
		name    string
		filter1 func() eventstore.Filter
		filter2 func() eventstore.Filter
	}{
		{
			name: "different_event_types",
			filter1: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("EventA").
					Finalize()
			},
			filter2: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("EventB").
					Finalize()
			},
		},
		{
			name: "different_predicates",
			filter1: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyPredicateOf(eventstore.P("Key1", "Value1")).
					Finalize()
			},
			filter2: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyPredicateOf(eventstore.P("Key1", "Value2")).
					Finalize()
			},
		},
		{
			name: "different_predicate_logic",
			filter1: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyPredicateOf(
						eventstore.P("Key1", "Value1"),
						eventstore.P("Key2", "Value2")).
					Finalize()
			},
			filter2: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AllPredicatesOf(
						eventstore.P("Key1", "Value1"),
						eventstore.P("Key2", "Value2")).
					Finalize()
			},
		},
		{
			name: "different_time_boundaries",
			filter1: func() eventstore.Filter {
				timeFrom := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
				return eventstore.BuildEventFilter().
					OccurredFrom(timeFrom).
					Finalize()
			},
			filter2: func() eventstore.Filter {
				timeFrom := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
				return eventstore.BuildEventFilter().
					OccurredFrom(timeFrom).
					Finalize()
			},
		},
		{
			name: "different_sequence_boundaries",
			filter1: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					WithSequenceNumberHigherThan(100).
					Finalize()
			},
			filter2: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					WithSequenceNumberHigherThan(200).
					Finalize()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter1 := tt.filter1()
			filter2 := tt.filter2()

			hash1 := filter1.Hash()
			hash2 := filter2.Hash()

			assert.NotEqual(t, hash1, hash2, "Different filters should have different hashes")
		})
	}
}

func Test_Filter_Hash_SameFilter_SameHash(t *testing.T) {
	tests := []struct {
		name   string
		filter func() eventstore.Filter
	}{
		{
			name: "same_filter_built_twice",
			filter: func() eventstore.Filter {
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("EventA", "EventB").
					AndAnyPredicateOf(
						eventstore.P("Key1", "Value1"),
						eventstore.P("Key2", "Value2")).
					Finalize()
			},
		},
		{
			name: "same_filter_with_reordered_input_sanitized",
			filter: func() eventstore.Filter {
				// Note: The builder sanitizes input by sorting, so even if we provide
				// different order, the result should be the same
				return eventstore.BuildEventFilter().
					Matching().
					AnyEventTypeOf("EventB", "EventA"). // Different order
					AndAnyPredicateOf(
						eventstore.P("Key2", "Value2"), // Different order
						eventstore.P("Key1", "Value1")).
					Finalize()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build the filter twice
			filter1 := tt.filter()
			filter2 := tt.filter()

			hash1 := filter1.Hash()
			hash2 := filter2.Hash()

			assert.Equal(t, hash1, hash2, "Same filters should have same hash")
		})
	}
}

func Test_Filter_Serialize_IncludesAllComponents(t *testing.T) {
	timeFrom := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	timeUntil := time.Date(2025, 12, 31, 18, 0, 0, 0, time.UTC)

	filter := eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf("EventA", "EventB").
		AndAllPredicatesOf(
			eventstore.P("BookID", "book-123"),
			eventstore.P("Status", "active")).
		OrMatching().
		AnyPredicateOf(eventstore.P("Type", "special")).
		OccurredFrom(timeFrom).
		AndOccurredUntil(timeUntil).
		Finalize()

	serialized := filter.Serialize()

	// Check that all components are included in serialization
	assert.Contains(t, serialized, "EventA", "Should include event type EventA")
	assert.Contains(t, serialized, "EventB", "Should include event type EventB")
	assert.Contains(t, serialized, "BookID=book-123", "Should include BookID predicate")
	assert.Contains(t, serialized, "Status=active", "Should include Status predicate")
	assert.Contains(t, serialized, "Type=special", "Should include Type predicate")
	assert.Contains(t, serialized, "predicate_logic:AND", "Should include AND logic")
	assert.Contains(t, serialized, "predicate_logic:OR", "Should include OR logic")
	assert.Contains(t, serialized, "occurred_from:", "Should include occurred_from")
	assert.Contains(t, serialized, "occurred_until:", "Should include occurred_until")

	// Check structure markers
	assert.Contains(t, serialized, "item:0", "Should include first item marker")
	assert.Contains(t, serialized, "item:1", "Should include second item marker")
}

func Test_Filter_Serialize_WithSequenceBoundary(t *testing.T) {
	filter := eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf("TestEvent").
		WithSequenceNumberHigherThan(98765).
		Finalize()

	serialized := filter.Serialize()

	assert.Contains(t, serialized, "TestEvent", "Should include event type")
	assert.Contains(t, serialized, "sequence_higher_than:98765", "Should include sequence boundary")
	assert.NotContains(t, serialized, "occurred_from:", "Should not include time boundaries")
	assert.NotContains(t, serialized, "occurred_until:", "Should not include time boundaries")
}

func Test_Filter_Serialize_Empty_Components(t *testing.T) {
	// Filter with minimal components
	filter := eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf("TestEvent").
		Finalize()

	serialized := filter.Serialize()

	assert.Contains(t, serialized, "TestEvent", "Should include event type")
	assert.Contains(t, serialized, "predicate_logic:OR", "Should include default predicate logic")
	assert.NotContains(t, serialized, "predicates:", "Should not include empty predicates")
	assert.NotContains(t, serialized, "occurred_from:", "Should not include empty time boundaries")
	assert.NotContains(t, serialized, "sequence_higher_than:", "Should not include empty sequence boundary")
}

//nolint:funlen
func Test_Filter_ReopenForSequenceFiltering_Compatible(t *testing.T) {
	tests := []struct {
		name           string
		baseFilter     eventstore.Filter
		sequenceNumber uint
		validateResult func(t *testing.T, result eventstore.Filter)
	}{
		{
			name: "event_types_only_filter_can_reopen",
			baseFilter: eventstore.BuildEventFilter().
				Matching().
				AnyEventTypeOf("BookAdded", "BookRemoved").
				Finalize(),
			sequenceNumber: 12345,
			validateResult: func(t *testing.T, result eventstore.Filter) {
				assert.Equal(t, uint(12345), result.SequenceNumberHigherThan())
				assert.True(t, result.OccurredFrom().IsZero())
				assert.True(t, result.OccurredUntil().IsZero())
				assert.Len(t, result.Items(), 1)
				assert.ElementsMatch(t, []string{"BookAdded", "BookRemoved"}, result.Items()[0].EventTypes())
				assert.Empty(t, result.Items()[0].Predicates())
			},
		},
		{
			name: "event_types_with_predicates_can_reopen",
			baseFilter: eventstore.BuildEventFilter().
				Matching().
				AnyEventTypeOf("BookAdded").
				AndAnyPredicateOf(eventstore.P("BookID", "book-123")).
				Finalize(),
			sequenceNumber: 9876,
			validateResult: func(t *testing.T, result eventstore.Filter) {
				assert.Equal(t, uint(9876), result.SequenceNumberHigherThan())
				assert.True(t, result.OccurredFrom().IsZero())
				assert.True(t, result.OccurredUntil().IsZero())
				assert.Len(t, result.Items(), 1)
				assert.Equal(t, []string{"BookAdded"}, result.Items()[0].EventTypes())
				assert.Len(t, result.Items()[0].Predicates(), 1)
				assert.Equal(t, "BookID", result.Items()[0].Predicates()[0].Key())
				assert.Equal(t, "book-123", result.Items()[0].Predicates()[0].Val())
			},
		},
		{
			name: "predicates_only_filter_can_reopen",
			baseFilter: eventstore.BuildEventFilter().
				Matching().
				AnyPredicateOf(eventstore.P("ReaderID", "reader-456")).
				Finalize(),
			sequenceNumber: 5555,
			validateResult: func(t *testing.T, result eventstore.Filter) {
				assert.Equal(t, uint(5555), result.SequenceNumberHigherThan())
				assert.True(t, result.OccurredFrom().IsZero())
				assert.True(t, result.OccurredUntil().IsZero())
				assert.Len(t, result.Items(), 1)
				assert.Empty(t, result.Items()[0].EventTypes())
				assert.Len(t, result.Items()[0].Predicates(), 1)
				assert.Equal(t, "ReaderID", result.Items()[0].Predicates()[0].Key())
				assert.Equal(t, "reader-456", result.Items()[0].Predicates()[0].Val())
			},
		},
		{
			name: "existing_sequence_filter_can_reopen_with_new_sequence",
			baseFilter: eventstore.BuildEventFilter().
				Matching().
				AnyEventTypeOf("BookAdded").
				WithSequenceNumberHigherThan(1000).
				Finalize(),
			sequenceNumber: 2000,
			validateResult: func(t *testing.T, result eventstore.Filter) {
				assert.Equal(t, uint(2000), result.SequenceNumberHigherThan()) // New sequence number
				assert.True(t, result.OccurredFrom().IsZero())
				assert.True(t, result.OccurredUntil().IsZero())
				assert.Len(t, result.Items(), 1)
				assert.Equal(t, []string{"BookAdded"}, result.Items()[0].EventTypes())
			},
		},
		{
			name: "zero_sequence_number_preserved",
			baseFilter: eventstore.BuildEventFilter().
				Matching().
				AnyEventTypeOf("BookAdded").
				Finalize(),
			sequenceNumber: 0,
			validateResult: func(t *testing.T, result eventstore.Filter) {
				assert.Equal(t, uint(0), result.SequenceNumberHigherThan())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reopened := tt.baseFilter.ReopenForSequenceFiltering()

			// Should return SequenceFilteringCapable
			capable, ok := reopened.(eventstore.SequenceFilteringCapable)
			assert.True(t, ok, "Should return SequenceFilteringCapable interface")
			assert.NotNil(t, capable)

			// Should be able to add sequence filtering
			result := capable.WithSequenceNumberHigherThan(tt.sequenceNumber).Finalize()

			// Validate the result
			tt.validateResult(t, result)
		})
	}
}

func Test_Filter_ReopenForSequenceFiltering_Incompatible(t *testing.T) {
	tests := []struct {
		name           string
		baseFilter     eventstore.Filter
		expectedReason string
	}{
		{
			name: "filter_with_occurred_from_cannot_reopen",
			baseFilter: eventstore.BuildEventFilter().
				Matching().
				AnyEventTypeOf("BookAdded").
				OccurredFrom(time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)).
				Finalize(),
			expectedReason: "cannot add sequence filtering: time boundaries already present",
		},
		{
			name: "filter_with_occurred_until_cannot_reopen",
			baseFilter: eventstore.BuildEventFilter().
				Matching().
				AnyEventTypeOf("BookAdded").
				OccurredUntil(time.Date(2025, 1, 1, 20, 0, 0, 0, time.UTC)).
				Finalize(),
			expectedReason: "cannot add sequence filtering: time boundaries already present",
		},
		{
			name: "filter_with_both_time_boundaries_cannot_reopen",
			baseFilter: eventstore.BuildEventFilter().
				Matching().
				AnyEventTypeOf("BookAdded").
				OccurredFrom(time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)).
				AndOccurredUntil(time.Date(2025, 1, 1, 20, 0, 0, 0, time.UTC)).
				Finalize(),
			expectedReason: "cannot add sequence filtering: time boundaries already present",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reopened := tt.baseFilter.ReopenForSequenceFiltering()

			// Should return SequenceFilteringIncompatible
			incompatible, ok := reopened.(eventstore.SequenceFilteringIncompatible)
			assert.True(t, ok, "Should return SequenceFilteringIncompatible interface")
			assert.NotNil(t, incompatible)

			// Should document why it's incompatible
			reason := incompatible.CannotAddSequenceFiltering()
			assert.Equal(t, tt.expectedReason, reason)
		})
	}
}

func Test_Filter_ReopenForSequenceFiltering_CompileTimeSafety(t *testing.T) {
	// Test that the type system enforces compile-time safety

	// Compatible filter
	compatibleFilter := eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf("BookAdded").
		Finalize()

	reopened := compatibleFilter.ReopenForSequenceFiltering()

	// This should compile - compatible filter returns SequenceFilteringCapable
	if capable, ok := reopened.(eventstore.SequenceFilteringCapable); ok {
		result := capable.WithSequenceNumberHigherThan(123).Finalize()
		assert.Equal(t, uint(123), result.SequenceNumberHigherThan())
	} else {
		t.Fatal("Compatible filter should return SequenceFilteringCapable")
	}

	// Incompatible filter
	incompatibleFilter := eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf("BookAdded").
		OccurredFrom(time.Now()).
		Finalize()

	reopenedIncompatible := incompatibleFilter.ReopenForSequenceFiltering()

	// This should NOT be SequenceFilteringCapable
	if _, ok := reopenedIncompatible.(eventstore.SequenceFilteringCapable); ok {
		t.Fatal("Incompatible filter should NOT return SequenceFilteringCapable")
	}

	// This should be SequenceFilteringIncompatible
	if incompatible, ok := reopenedIncompatible.(eventstore.SequenceFilteringIncompatible); ok {
		reason := incompatible.CannotAddSequenceFiltering()
		assert.Contains(t, reason, "time boundaries already present")
	} else {
		t.Fatal("Incompatible filter should return SequenceFilteringIncompatible")
	}
}
