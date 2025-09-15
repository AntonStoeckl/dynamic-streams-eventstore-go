// Package snapshot provides clean, observability-free snapshot optimization for query handlers.
//
// This package implements a pure, type-safe wrapper that adds incremental projection capabilities
// to any CoreQueryHandler, significantly improving performance by avoiding full event history replay.
// The wrapper is completely observability-free, focusing only on snapshot optimization logic.
//
// # Core Components
//
// ## Main Wrapper
// QueryWrapper[Q, R] - The clean wrapper that transforms any CoreQueryHandler into a
// snapshot-aware handler. It uses Go generics to ensure type safety between queries (Q) and results (R).
//
// ## Interface Architecture
// - SavesAndLoadsSnapshots - Pure snapshot operations (SaveSnapshot, LoadSnapshot)
// - QueriesEventsAndHandlesSnapshots - Composite interface combining shell.QueriesEvents + SavesAndLoadsSnapshots
// - CoreQueryHandler - Clean handlers without observability dependencies
//
// # Usage Pattern
//
// ## Complete Integration Example
//
// For a clean query handler (post-migration architecture):
//
//	// 1. Create clean core query handler
//	coreHandler := bookslentbyreader.NewQueryHandler(eventStore)
//
//	// 2. Wrap with snapshot optimization
//	snapshotHandler, err := snapshot.NewQueryWrapper(
//		coreHandler,
//		snapshotCapableEventStore,
//		bookslentbyreader.Project,
//		func(q bookslentbyreader.Query) eventstore.Filter {
//			return bookslentbyreader.BuildEventFilter(q.ReaderID)
//		},
//	)
//	if err != nil {
//		return err
//	}
//
//	// 3. Optionally wrap with observability
//	observableHandler, err := observable.NewQueryWrapper(
//		snapshotHandler,
//		observable.WithQueryMetrics[...](metricsCollector),
//		observable.WithQueryTracing[...](tracingCollector),
//	)
//
//	// 4. Use as drop-in replacement
//	result, err := observableHandler.Handle(ctx, bookslentbyreader.BuildQuery(readerID))
//
// # Clean Architecture
//
// The new architecture separates concerns cleanly:
// - CoreQueryHandler: Pure business logic, no observability
// - QueryWrapper: Pure snapshot optimization, no observability
// - Observable wrappers: Pure observability instrumentation
//
// # Workflow
//
// The wrapper executes a pure snapshot workflow:
// 1. Load an existing snapshot (if available)
// 2. Deserialize snapshot to base projection
// 3. Query incremental events since snapshot
// 4. Project incremental events onto base projection
// 5. Save the updated snapshot for future queries
// 6. Fail fast on errors with descriptive messages
//
// # Type Safety
//
// The wrapper uses compile-time type checking to ensure compatibility between
// queries, results, and projection functions. EventStore capability is verified
// at wrapper creation time, with clear error messages for incompatible configurations.
package snapshot
