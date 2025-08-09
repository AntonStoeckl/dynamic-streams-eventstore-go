package registeredreaders

import (
	"maps"
	"slices"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// ProjectRegisteredReaders implements the query logic to determine all readers currently registered.
// This is a pure function with no side effects - it takes the current domain events and a query
// and returns the projected state showing all readers currently registered with active contracts.
//
// Query Logic:
//
//	GIVEN: All events in the system
//	WHEN: RegisteredReaders query is executed
//	THEN: RegisteredReaders struct is returned with current registration state
//	INCLUDES: Readers currently registered (registered but not canceled)
//	EXCLUDES: Readers that have had their contracts canceled
func ProjectRegisteredReaders(history core.DomainEvents) RegisteredReaders {
	// Track reader registration state and reader information
	readers := make(map[string]ReaderInfo)

	for _, event := range history {
		switch e := event.(type) {
		case core.ReaderRegistered:
			// Add the reader to the registration state
			readers[e.ReaderID] = ReaderInfo{
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
	readerList := slices.Collect(maps.Values(readers))
	slices.SortFunc(readerList, func(a, b ReaderInfo) int {
		return a.RegisteredAt.Compare(b.RegisteredAt)
	})

	return RegisteredReaders{
		Readers: readerList,
		Count:   len(readerList),
	}
}
