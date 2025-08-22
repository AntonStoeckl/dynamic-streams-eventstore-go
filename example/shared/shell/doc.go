// Package shell provides the infrastructure layer for event sourcing applications,
// implementing the "imperative shell" pattern that bridges the functional core and external systems.
//
// This package serves as the shared abstraction layer for the event sourcing system,
// handling event conversion, observability patterns, and core interface definitions
// while supporting vertical slice architecture through minimal coupling.
//
// # Core Components
//
// ## Event Conversion
// - DomainEventsFrom() - Converts storable events to domain events
// - StorableEventsFrom() - Converts domain events to storable events
// - Event metadata handling and serialization management
//
// ## Query/Command Abstractions
// - Query - Contract for all query types with QueryType() method
// - QueryResult - Contract for projections with GetSequenceNumber() method
// - QueryHandler[Q, R] - Generic interface for query processing components
// - ProjectionFunc[Q, R] - Signature for pure projection functions
//
// ## Infrastructure Interfaces
// - QueriesEvents - Shared abstraction for event store query operations
// - ExposesSnapshotWrapperDependencies - Dependency exposure for snapshot optimization
// - FilterBuilderFunc[Q], SnapshotTypeFunc[Q] - Function type signatures
//
// ## Observability Patterns
// - MetricsCollector, TracingCollector, ContextualLogger interfaces
// - StartQuerySpan(), FinishQuerySpan() - Distributed tracing helpers
// - RecordQueryMetrics(), RecordQueryComponentDuration() - Metrics recording
// - LogQueryStart(), LogQuerySuccess(), LogQueryError() - Structured logging
//
// # Architecture Philosophy
//
// This package represents a pragmatic trade-off in the vertical slice architecture.
// While feature slices remain independent for business logic, they share these
// infrastructure concerns to enable cross-cutting optimizations like snapshot-based
// query acceleration without duplicating complex wrapper logic.
//
// In Domain-Driven Design terminology, this serves as the 'infrastructure' layer,
// providing technical capabilities that support but don't contain business logic.
package shell
