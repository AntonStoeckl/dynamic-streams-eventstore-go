// Package snapshot provides generic snapshot-based optimization for query handlers in event sourcing systems.
//
// This package implements a unified, type-safe wrapper that adds incremental projection capabilities
// to any query handler, significantly improving performance by avoiding full event history replay.
// The wrapper automatically extracts dependencies from base handlers to maintain clean architecture.
//
// # Core Components
//
// ## Main Wrapper
// GenericSnapshotWrapper[Q, R] - The main wrapper that transforms any query handler into a
// snapshot-aware handler. It uses Go generics to ensure type safety between queries (Q) and results (R).
//
// ## Interface Architecture
// - SavesAndLoadsSnapshots - Pure snapshot operations (SaveSnapshot, LoadSnapshot)
// - QueriesEventsAndHandlesSnapshots - Composite interface combining shell.QueriesEvents + SavesAndLoadsSnapshots
// - Dependency extraction via shell.ExposesSnapshotWrapperDependencies embedded in base handlers
//
// # Usage Pattern
//
// ## Complete Integration Example
//
// For a query handler with parameters (like BooksLentByReader):
//
//	// 1. Create base query handler
//	baseHandler, err := bookslentbyreader.NewQueryHandler(snapshotCapableEventStore)
//	if err != nil {
//		return err
//	}
//
//	// 2. Wrap with snapshot optimization
//	wrapper, err := snapshot.NewGenericSnapshotWrapper[
//		bookslentbyreader.Query,
//		bookslentbyreader.BooksCurrentlyLent,
//	](
//		baseHandler,
//		bookslentbyreader.Project,
//		func(q bookslentbyreader.Query) eventstore.Filter {
//			return bookslentbyreader.BuildEventFilter(q.ReaderID)
//		},
//		func(queryType string, q bookslentbyreader.Query) string {
//			return queryType + ":" + q.ReaderID.String()
//		},
//	)
//	if err != nil {
//		return err
//	}
//
//	// 3. Use wrapper as drop-in replacement
//	result, err := wrapper.Handle(ctx, bookslentbyreader.BuildQuery(readerID))
//
// ## Parameter-less Query Handler Example
//
// For handlers without parameters (like BooksInCirculation):
//
//	wrapper, err := snapshot.NewGenericSnapshotWrapper[
//		booksincirculation.Query,
//		booksincirculation.BooksInCirculation,
//	](
//		baseHandler,
//		booksincirculation.Project,
//		func(_ booksincirculation.Query) eventstore.Filter {
//			return booksincirculation.BuildEventFilter()
//		},
//		func(queryType string, _ booksincirculation.Query) string {
//			return queryType
//		},
//	)
//
// # Dependency Extraction Architecture
//
// The wrapper automatically extracts all required dependencies from the base handler:
// - EventStore via ExposeEventStore() with type assertion to QueriesEventsAndHandlesSnapshots
// - Observability components (metrics, tracing, logging) via Expose*() methods
// - Returns ErrEventStoreNotSnapshotCapable if EventStore doesn't support snapshots
//
// # Workflow
//
// The wrapper executes a sophisticated snapshot workflow:
// 1. Load an existing snapshot (if available)
// 2. Deserialize snapshot to base projection
// 3. Query incremental events since snapshot
// 4. Project incremental events onto base projection
// 5. Save the updated snapshot for future queries
// 6. Fallback to base handler on any error (non-fatal)
//
// # Observability
//
// All observability components (metrics, tracing, logging) are automatically extracted
// from the base handler via the ExposesSnapshotWrapperDependencies interface.
// The wrapper adds detailed component-level timing metrics for each phase:
// ComponentSnapshotLoad, ComponentIncrementalQuery, ComponentUnmarshal,
// ComponentSnapshotDeserialize, ComponentIncrementalProjection, ComponentSnapshotSave.
//
// # Type Safety
//
// The wrapper uses compile-time type checking to ensure compatibility between
// queries, results, and projection functions. EventStore capability is verified
// via type assertion at wrapper creation time, with clear error messages for
// incompatible configurations.
package snapshot
