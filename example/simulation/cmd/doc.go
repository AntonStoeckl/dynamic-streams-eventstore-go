// Package main implements a sophisticated library simulation for the Dynamic Event Streams EventStore.
//
// This high-performance simulation demonstrates the EventStore's key innovation: dynamic consistency boundaries
// that allow querying and appending events across multiple logical entities atomically. Unlike traditional
// event stores with fixed streams per aggregate, operations can span multiple entities using complex predicates
// like "BookID='book-123' OR ReaderID='reader-456'" for cross-aggregate business workflows.
//
// ## Scale and Realism
// The simulation operates at realistic "Münchner Stadtbibliothek" (public library) branch scale:
//   - 60,000-65,000 books in circulation
//   - 14,000-15,000 active readers
//   - Configurable request rates (default: 30 req/s, tested up to 300+ req/s)
//   - Real library operation scenarios with proper business logic
//
// ## Intelligent State-Aware Scenario Generation
// The ScenarioSelector uses SimulationState to generate realistic scenarios:
//   - Smart thresholds: ensures minimum books/readers before complex operations
//   - Lending conflicts: realistic competition for popular titles (not artificial scarcity)
//   - Actor-aware returns: books can only be returned by the reader who borrowed them
//   - Error injection: configurable rates for idempotent operations, conflicts, and edge cases
//
// ## Multi-Adapter Database Support
// Supports all EventStore database adapters with identical functionality:
//   - pgx.Pool: High-performance PostgreSQL driver (default)
//   - sql.DB: Standard library compatibility with lib/pq
//   - sqlx.DB: Extended SQL features with lib/pq
//   - Switchable via DB_ADAPTER environment variable
//
// ## Production-Grade Observability
// Full OpenTelemetry integration with real-time monitoring:
//   - Metrics: Request rates, latencies, error rates, business KPIs
//   - Tracing: Distributed tracing across command-query-append operations
//   - Logging: Structured contextual logging with correlation IDs
//   - Grafana dashboards for performance analysis and bottleneck identification
//
// ## Worker Pool Architecture
// Concurrent request processing with configurable worker pools:
//   - Rate-limited request generation matching target throughput
//   - Thread-safe state management with mutex-protected operations
//   - Graceful shutdown with context cancellation
//   - Performance monitoring and bottleneck detection
//
// ## Domain Features
// Complete library management operations using proper Domain-Driven Design:
//   - Book circulation: add/remove books from circulation
//   - Reader management: register readers and cancel contracts
//   - Lending operations: lend books to readers with business rule validation
//   - Return processing: actor-aware return logic and availability updates
//   - Query projections: real-time state queries for decision making
//
// The simulation serves as both a performance testing tool and a demonstration of the EventStore's
// capabilities for complex event sourcing scenarios with cross-aggregate consistency requirements.
package main
