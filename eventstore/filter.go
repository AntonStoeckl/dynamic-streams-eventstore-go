package eventstore

import (
	"slices"
)

type FilterEventTypeString = string
type FilterKeyString = string
type FilterValString = string

/***** Filter *****/

type Filter struct {
	items []FilterItem
}

func (f Filter) Items() []FilterItem {
	return f.items
}

/***** FilterItem *****/

type FilterItem struct {
	eventTypes             []FilterEventTypeString
	predicates             []FilterPredicate
	allPredicatesMustMatch bool
}

func (fi FilterItem) EventTypes() []FilterEventTypeString {
	return fi.eventTypes
}

func (fi FilterItem) Predicates() []FilterPredicate {
	return fi.predicates
}

func (fi FilterItem) AllPredicatesMustMatch() bool {
	return fi.allPredicatesMustMatch
}

/***** FilterPredicate *****/

type FilterPredicate struct {
	key FilterKeyString
	val FilterValString
}

func P(key FilterKeyString, val FilterValString) FilterPredicate {
	return FilterPredicate{key: key, val: val}
}

func (fp FilterPredicate) Key() FilterKeyString {
	return fp.key
}

func (fp FilterPredicate) Val() FilterValString {
	return fp.val
}

/***** FilterBuilder *****/

// FilterBuilder builds a generic event filter to be used in DB type-specific eventstore implementations to build queries for
// the specific query language, e.g.: Postgres, Mysql, MongoDB, ...
// It is designed with the idea to only allow "useful" filter combinations for event-sourced workflows:
//
//   - empty filter
//   - (eventType)
//   - (eventType OR eventType...)
//   - (predicate)
//   - (predicate OR predicate...)
//   - (predicate AND predicate...)
//   - (eventType AND predicate)
//   - (eventType AND (predicate OR predicate...))
//   - (eventType AND (predicate AND predicate...))
//   - ((eventType OR eventType...) AND (predicate OR predicate...))
//   - ((eventType OR eventType...) AND (predicate AND predicate...))
//   - ((eventType AND predicate) OR (eventType AND predicate)...) -> multiple FilterItem(s)
type FilterBuilder interface {
	// Matching starts a new FilterItem.
	Matching() EmptyFilterItemBuilder

	// MatchingAnyEvent directly creates an empty Filter.
	MatchingAnyEvent() Filter
}

type EmptyFilterItemBuilder interface {
	// AnyEventTypeOf adds one or multiple EventTypes to the current FilterItem.
	//
	// It sanitizes the input:
	//	- removing empty EventTypes ("")
	//	- sorting the EventTypes
	//	- removing duplicate EventTypes
	AnyEventTypeOf(eventType FilterEventTypeString, eventTypes ...FilterEventTypeString) FilterItemBuilderLackingPredicates

	// AnyPredicateOf adds one or multiple FilterPredicate(s) to the current FilterItem.
	//
	// It sanitizes the input:
	//	- removing empty/partial FilterPredicate(s) (key or val is "")
	//	- sorting the FilterPredicate(s)
	//	- removing duplicate FilterPredicate(s)
	AnyPredicateOf(predicate FilterPredicate, predicates ...FilterPredicate) FilterItemBuilderLackingEventTypes

	AllPredicatesOf(predicate FilterPredicate, predicates ...FilterPredicate) FilterItemBuilderLackingEventTypes
}

type FilterItemBuilderLackingPredicates interface {
	// AndAnyPredicateOf adds one or multiple FilterPredicate(s) to the current FilterItem.
	//
	// It sanitizes the input:
	//	- removing empty/partial FilterPredicate(s) (key or val is "")
	//	- sorting the FilterPredicate(s)
	//	- removing duplicate FilterPredicate(s)

	AndAnyPredicateOf(predicate FilterPredicate, predicates ...FilterPredicate) CompletedFilterItemBuilder

	AndAllPredicatesOf(predicate FilterPredicate, predicates ...FilterPredicate) CompletedFilterItemBuilder

	// OrMatching finalizes the current FilterItem and starts a new one.
	OrMatching() EmptyFilterItemBuilder

	// Finalize returns the Filter once it has at least one FilterItem with at least one EventType OR one Predicate.
	Finalize() Filter
}

type FilterItemBuilderLackingEventTypes interface {
	// AndAnyEventTypeOf adds one or multiple EventTypes to the current FilterItem.
	//
	// It sanitizes the input:
	//	- removing empty EventTypes ("")
	//	- sorting the EventTypes
	//	- removing duplicate EventTypes
	AndAnyEventTypeOf(eventType FilterEventTypeString, eventTypes ...FilterEventTypeString) CompletedFilterItemBuilder

	// OrMatching finalizes the current FilterItem and starts a new one.
	OrMatching() EmptyFilterItemBuilder

	// Finalize returns the Filter once it has at least one FilterItem with at least one EventType OR one Predicate.
	Finalize() Filter
}

type CompletedFilterItemBuilder interface {
	// OrMatching finalizes the current FilterItem and starts a new one.
	OrMatching() EmptyFilterItemBuilder

	// Finalize returns the Filter once it has at least one FilterItem with at least one EventType OR one Predicate.
	Finalize() Filter
}

// filterBuilder implements all the interfaces of FilterBuilder
type filterBuilder struct {
	filter            Filter
	currentFilterItem FilterItem
}

// BuildEventFilter creates a FilterBuilder which must eventually be finalized with Finalize() or MatchingAnyEvent().
func BuildEventFilter() FilterBuilder {
	return filterBuilder{}
}

// Matching starts a new FilterItem.
func (fb filterBuilder) Matching() EmptyFilterItemBuilder {
	fb.currentFilterItem = FilterItem{}

	return fb
}

// AnyEventTypeOf adds one or multiple EventTypes to the current FilterItem expecting ANY EventType to match.
//
// It sanitizes the input:
//   - removing empty EventTypes ("")
//   - sorting the EventTypes
//   - removing duplicate EventTypes
func (fb filterBuilder) AnyEventTypeOf(
	eventType FilterEventTypeString,
	eventTypes ...FilterEventTypeString,
) FilterItemBuilderLackingPredicates {

	fb.currentFilterItem.eventTypes = append(
		fb.currentFilterItem.eventTypes,
		fb.sanitizeEventTypes(eventType, eventTypes...)...,
	)

	return fb
}

// AndAnyEventTypeOf adds one or multiple EventTypes to the current FilterItem expecting ANY EventType to match.
//
// It sanitizes the input:
//   - removing empty EventTypes ("")
//   - sorting the EventTypes
//   - removing duplicate EventTypes
func (fb filterBuilder) AndAnyEventTypeOf(
	eventType FilterEventTypeString,
	eventTypes ...FilterEventTypeString,
) CompletedFilterItemBuilder {

	return fb.AnyEventTypeOf(eventType, eventTypes...)
}

func (fb filterBuilder) sanitizeEventTypes(
	eventType FilterEventTypeString,
	eventTypes ...FilterEventTypeString,
) []FilterEventTypeString {

	allEventTypes := append([]FilterEventTypeString{eventType}, eventTypes...)
	allEventTypes = slices.DeleteFunc(
		allEventTypes,
		func(e FilterEventTypeString) bool {
			return e == ""
		})
	slices.Sort(allEventTypes)
	allEventTypes = slices.Compact(allEventTypes)
	allEventTypes = slices.Clip(allEventTypes)

	return allEventTypes
}

// AnyPredicateOf adds one or multiple FilterPredicate(s) to the current FilterItem expecting ANY predicate to match.
//
// It sanitizes the input:
//   - removing empty/partial FilterPredicate(s) (key or val is "")
//   - sorting the FilterPredicate(s)
//   - removing duplicate FilterPredicate(s)
func (fb filterBuilder) AnyPredicateOf(
	predicate FilterPredicate,
	predicates ...FilterPredicate,
) FilterItemBuilderLackingEventTypes {

	fb.currentFilterItem.predicates = append(
		fb.currentFilterItem.predicates,
		fb.sanitizePredicates(predicate, predicates...)...,
	)

	return fb
}

// AndAnyPredicateOf adds one or multiple FilterPredicate(s) to the current FilterItem expecting ANY predicate to match.
//
// It sanitizes the input:
//   - removing empty/partial FilterPredicate(s) (key or val is "")
//   - sorting the FilterPredicate(s)
//   - removing duplicate FilterPredicate(s)
func (fb filterBuilder) AndAnyPredicateOf(
	predicate FilterPredicate,
	predicates ...FilterPredicate,
) CompletedFilterItemBuilder {

	return fb.AnyPredicateOf(predicate, predicates...)
}

// AllPredicatesOf adds one or multiple FilterPredicate(s) to the current FilterItem expecting ALL predicates to match.
//
// It sanitizes the input:
//   - removing empty/partial FilterPredicate(s) (key or val is "")
//   - sorting the FilterPredicate(s)
//   - removing duplicate FilterPredicate(s)
func (fb filterBuilder) AllPredicatesOf(
	predicate FilterPredicate,
	predicates ...FilterPredicate,
) FilterItemBuilderLackingEventTypes {

	fb.currentFilterItem.allPredicatesMustMatch = true

	fb.currentFilterItem.predicates = append(
		fb.currentFilterItem.predicates,
		fb.sanitizePredicates(predicate, predicates...)...,
	)

	return fb
}

// AndAllPredicatesOf adds one or multiple FilterPredicate(s) to the current FilterItem expecting ALL predicates to match.
//
// It sanitizes the input:
//   - removing empty/partial FilterPredicate(s) (key or val is "")
//   - sorting the FilterPredicate(s)
//   - removing duplicate FilterPredicate(s)
func (fb filterBuilder) AndAllPredicatesOf(
	predicate FilterPredicate,
	predicates ...FilterPredicate,
) CompletedFilterItemBuilder {

	return fb.AllPredicatesOf(predicate, predicates...)
}

func (fb filterBuilder) sanitizePredicates(
	predicate FilterPredicate,
	predicates ...FilterPredicate,
) []FilterPredicate {

	allPredicates := append([]FilterPredicate{predicate}, predicates...)
	allPredicates = slices.DeleteFunc(allPredicates, func(e FilterPredicate) bool { return len(e.key) == 0 || len(e.val) == 0 })
	slices.SortFunc(
		allPredicates,
		func(a, b FilterPredicate) int {
			if a.key > b.key {
				return 1
			}

			if a.key < b.key {
				return -1
			}

			return 0
		})

	allPredicates = slices.Compact(allPredicates)
	allPredicates = slices.Clip(allPredicates)

	return allPredicates
}

// OrMatching finalizes the current FilterItem and starts a new one.
func (fb filterBuilder) OrMatching() EmptyFilterItemBuilder {
	fb.filter.items = append(fb.filter.items, fb.currentFilterItem)
	fb.currentFilterItem = FilterItem{}

	return fb
}

// MatchingAnyEvent directly creates an empty filter.
func (fb filterBuilder) MatchingAnyEvent() Filter {
	return fb.filter
}

// Finalize returns the Filter once it has at least one FilterItem with at least one EventType OR one Predicate.
func (fb filterBuilder) Finalize() Filter {
	fb.filter.items = append(fb.filter.items, fb.currentFilterItem)

	return fb.filter
}
