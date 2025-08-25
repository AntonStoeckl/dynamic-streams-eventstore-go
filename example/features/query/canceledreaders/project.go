package canceledreaders

import (
	"slices"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// Project implements the query logic to determine all readers that have canceled their contracts.
// This is a pure function with no side effects - it takes the current domain events and optionally
// a base projection to build upon, returning the projected state showing all canceled readers.
//
// Query Logic:
//
//	GIVEN: All relevant events in the system (or incremental events since base projection)
//	WHEN: CanceledReaders query is executed
//	THEN: CanceledReaders struct is returned
//	INCLUDES: Readers that have canceled their contracts canceled
func Project(history core.DomainEvents, _ Query, maxSequence uint, base ...CanceledReaders) CanceledReaders {
	// Track reader cancellation state
	readers := make(map[string]*ReaderInfo) // Start fresh (full projection)

	if len(base) > 0 {
		readers = convertReadersToMap(base[0].Readers) // Start from an existing projection (incremental update)
	}

	for _, event := range history {
		if e, ok := event.(core.ReaderContractCanceled); ok {
			readers[e.ReaderID] = &ReaderInfo{
				ReaderID:   e.ReaderID,
				CanceledAt: e.OccurredAt,
			}
		}
	}

	// Convert map to slice and sort by CanceledAt (oldest first)
	readerList := make([]ReaderInfo, 0, len(readers))
	for _, readerPtr := range readers {
		readerList = append(readerList, *readerPtr)
	}
	slices.SortFunc(readerList, func(a, b ReaderInfo) int {
		return a.CanceledAt.Compare(b.CanceledAt)
	})

	return CanceledReaders{
		Readers:        readerList,
		Count:          len(readerList),
		SequenceNumber: maxSequence,
	}
}

// BuildEventFilter creates the filter for querying all reader cancellation events
// which are relevant for this query/use-case.
func BuildEventFilter() eventstore.Filter {
	return eventstore.BuildEventFilter().
		Matching().
		AnyEventTypeOf(
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
