// Package eventstore provides core abstractions and types for event sourcing
// with dynamic event streams.
//
// This package defines the fundamental interfaces and types used across
// different event store implementations, including filters, storable events,
// and common error definitions.
//
// The event store supports dynamic filtering of events based on:
//   - Event types
//   - JSON payload predicates
//   - Time ranges (occurred from/until)
//
// Key types:
//   - Filter: Defines criteria for querying events
//   - StorableEvent: Represents an event that can be stored and retrieved
//   - StorableEvents: Collection of storable events
//
// Common usage pattern:
//
//	// Create a filter for multiple event types with a predicate
//	filter := BuildEventFilter().
//		Matching().
//		AnyEventTypeOf(
//			core.BookCopyLentToReaderEventType,
//			core.BookCopyReturnedByReaderEventType).
//		AndAnyPredicateOf(P("BookID", bookID.String())).
//		Finalize()
//
//	events, maxSeq, err := store.Query(ctx, filter)
//	if err != nil {
//		// handle error
//	}
//
//	newEvent := eventstore.BuildStorableEvent(eventType, time.Now(), payload, metadata)
//	err = store.Append(ctx, filter, maxSeq, newEvent)
package eventstore
