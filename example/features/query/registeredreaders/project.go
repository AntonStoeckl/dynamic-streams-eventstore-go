package registeredreaders

import (
	"slices"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// Project implements the query logic to determine all readers currently registered.
// This is a pure function with no side effects - it takes the current domain events and optionally
// a base projection to build upon, returning the projected state showing all readers currently registered.
//
// Query Logic:
//
//	GIVEN: All events in the system (or incremental events since base projection)
//	WHEN: RegisteredReaders query is executed
//	THEN: RegisteredReaders struct is returned with current registration state
//	INCLUDES: Readers currently registered (registered but not canceled)
//	EXCLUDES: Readers that have had their contracts canceled
func Project(history core.DomainEvents, _ Query, maxSequence uint, base ...RegisteredReaders) RegisteredReaders {
	// Track reader registration state and reader information
	var readers map[string]*ReaderInfo

	if len(base) > 0 {
		readers = convertReadersToMap(base[0].Readers) // Start from an existing projection (incremental update)
	} else {
		readers = make(map[string]*ReaderInfo) // Start fresh (full projection)
	}

	for _, event := range history {
		switch e := event.(type) {
		case core.ReaderRegistered:
			// Add the reader to the registration state
			readers[e.ReaderID] = &ReaderInfo{
				ReaderID:     e.ReaderID,
				Name:         e.Name,
				RegisteredAt: e.OccurredAt,
			}

		case core.ReaderContractCanceled:
			// Remove reader from registered state
			delete(readers, e.ReaderID)
		}
	}

	// Convert map to slice and sort by RegisteredAt (oldest first)
	readerList := make([]ReaderInfo, 0, len(readers))
	for _, readerPtr := range readers {
		readerList = append(readerList, *readerPtr)
	}
	slices.SortFunc(readerList, func(a, b ReaderInfo) int {
		return a.RegisteredAt.Compare(b.RegisteredAt)
	})

	return RegisteredReaders{
		Readers:        readerList,
		Count:          len(readerList),
		SequenceNumber: maxSequence,
	}
}

// BuildEventFilter creates the filter for querying all reader registration events
// which are relevant for this query/use-case.
func BuildEventFilter() eventstore.Filter {
	return eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf(
			core.ReaderRegisteredEventType,
			core.ReaderContractCanceledEventType,
		).
		Finalize()
}

// convertReadersToMap converts a slice of ReaderInfo to a map keyed by ReaderID for efficient lookup.
// This is used when starting from a base projection for incremental updates.
func convertReadersToMap(readers []ReaderInfo) map[string]*ReaderInfo {
	readerInfos := make(map[string]*ReaderInfo, len(readers))
	for i := range readers {
		reader := readers[i] // Create a copy to avoid taking address of loop variable
		readerInfos[reader.ReaderID] = &reader
	}
	return readerInfos
}
