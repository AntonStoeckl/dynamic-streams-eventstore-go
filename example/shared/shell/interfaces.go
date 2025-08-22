package shell

import (
	"context"

	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
	"github.com/AntonStoeckl/dynamic-streams-eventstore-go/example/shared/core"
)

// QueriesEvents defines the interface needed by query handlers for event store operations.
// This abstraction is shared to support snapshot wrapping while maintaining type safety.
// Note: This represents a pragmatic trade-off in the vertical slice architecture to enable
// cross-cutting snapshot optimization without duplicating wrapper logic in each feature slice.
type QueriesEvents interface {
	Query(ctx context.Context, filter eventstore.Filter) (
		eventstore.StorableEvents,
		eventstore.MaxSequenceNumberUint,
		error,
	)
}

// ExposesSnapshotWrapperDependencies provides access to internal components needed for snapshot wrapping.
// This interface represents a pragmatic trade-off in the vertical slice architecture to enable
// cross-cutting snapshot optimization without duplicating wrapper logic in each feature slice.
type ExposesSnapshotWrapperDependencies interface {
	ExposeEventStore() QueriesEvents
	ExposeMetricsCollector() MetricsCollector
	ExposeTracingCollector() TracingCollector
	ExposeContextualLogger() ContextualLogger
	ExposeLogger() Logger
}

// Query represents the contract for all query types in the event-sourced application.
// Each query encapsulates the intent and parameters needed to retrieve a specific projection.
// The QueryType method enables polymorphic handling and snapshot type generation.
// Queries can range from simple parameter-less requests to complex multi-parameter filters.
type Query interface {
	QueryType() string
}

// QueryResult represents the contract for all query result types (projections).
// Each result encapsulates the current state derived from event history.
// The GetSequenceNumber method returns the highest event sequence number included in the projection,
// enabling incremental updates and consistency checking for snapshot-based optimizations.
// This interface ensures all projections can participate in the snapshot workflow.
type QueryResult interface {
	GetSequenceNumber() uint
}

// QueryHandler defines the contract for components that process queries and return projections.
// Handlers orchestrate the complete query workflow: retrieving events, unmarshaling, and projecting.
// The generic parameters Q and R ensure type safety between queries and their corresponding results.
// Implementations handle infrastructure concerns (EventStore access, observability) while delegating
// business logic to pure projection functions.
type QueryHandler[Q Query, R QueryResult] interface {
	Handle(ctx context.Context, query Q) (R, error)
	ExposesSnapshotWrapperDependencies
}

// ProjectionFunc defines the signature for pure functions that transform events into projections.
// These functions implement the core business logic of event sourcing: deriving the current state from history.
// The optional base parameter enables incremental projection updates from a previous snapshot state.
// Functions must be deterministic - the same events always produce the same projection.
// The query parameter provides context for filtering or customizing the projection logic.
type ProjectionFunc[Q Query, R QueryResult] func(
	events core.DomainEvents,
	query Q,
	maxSeq uint,
	base ...R,
) R

// FilterBuilderFunc constructs event store filters based on query parameters.
// Implementations extract relevant predicates from the query to filter events.
// For parameter-less queries, they return a filter without predicates (only filter by event types).
// The filter determines which events are retrieved for projection building.
type FilterBuilderFunc[Q Query] func(query Q) eventstore.Filter

// SnapshotTypeFunc generates unique snapshot identifiers from query type and parameters.
// The identifier must be deterministic and unique for each query/parameter combination.
// For queries with parameters (e.g., ReaderID), include them in the identifier.
// For parameter-less queries, return just the query type string.
// Examples:
//
//	"BooksInCirculation"
//	"BooksLentByReader:123e4567-e89b-12d3-a456-426614174000".
type SnapshotTypeFunc[Q Query] func(queryType string, query Q) string
