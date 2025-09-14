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
	SnapshotType() string
}

// QueryResult represents the contract for all query result types (projections).
// Each result encapsulates the current state derived from event history.
// The GetSequenceNumber method returns the highest event sequence number included in the projection,
// enabling incremental updates and consistency checking for snapshot-based optimizations.
// This interface ensures all projections can participate in the snapshot workflow.
type QueryResult interface {
	GetSequenceNumber() uint
}

// CoreQueryHandler defines the contract for components that process queries with pure business logic.
// Handlers orchestrate the complete query workflow: retrieving events, unmarshaling, and projecting.
// The generic parameters Q and R ensure type safety between queries and their corresponding results.
// Implementations should focus purely on business logic without observability or infrastructure concerns.
// This interface is designed to be wrapped with observability decorators for complete functionality.
type CoreQueryHandler[Q Query, R QueryResult] interface {
	Handle(ctx context.Context, query Q) (R, error)
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

// Command represents the contract for all command types in the event-sourced application.
// Each command encapsulates the intent and parameters needed to execute a specific business operation.
// The CommandType method enables polymorphic handling and observability instrumentation.
// Commands can range from simple parameter-less requests to complex multi-parameter operations.
type Command interface {
	CommandType() string
}

// CoreCommandHandler defines the contract for components that process commands with pure business logic.
// Handlers orchestrate the complete command workflow: retrieving events, unmarshaling, business logic, and appending.
// The generic parameter C ensures type safety between commands and their corresponding handlers.
// Implementations should focus purely on business logic without observability or infrastructure concerns.
// This interface is designed to be wrapped with observability decorators for complete functionality.
// Handlers return HandlerResult containing business outcomes (idempotency) and execution metadata (retry info).
type CoreCommandHandler[C Command] interface {
	Handle(ctx context.Context, command C) (HandlerResult, error)
}

// CommandHandler defines the contract for command handlers that return only errors (compatibility interface).
// This interface is used for backward compatibility with existing code that expects error-only return values.
// Typically implemented by wrapper types that convert (HandlerResult, error) to just error.
type CommandHandler[C Command] interface {
	Handle(ctx context.Context, command C) error
}
