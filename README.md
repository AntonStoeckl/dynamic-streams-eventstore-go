# dynamic-streams-eventstore-go

[![Go Report Card](https://goreportcard.com/badge/github.com/AntonStoeckl/dynamic-streams-eventstore-go)](https://goreportcard.com/report/github.com/AntonStoeckl/dynamic-streams-eventstore-go)
[![codecov](https://codecov.io/gh/AntonStoeckl/dynamic-streams-eventstore-go/branch/main/graph/badge.svg)](https://codecov.io/gh/AntonStoeckl/dynamic-streams-eventstore-go)
[![GoDoc](https://godoc.org/github.com/AntonStoeckl/dynamic-streams-eventstore-go?status.svg)](https://godoc.org/github.com/AntonStoeckl/dynamic-streams-eventstore-go)
[![License: GPL v3](https://img.shields.io/badge/License-GPL%20v3-green.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Go Version](https://img.shields.io/github/go-mod/go-version/AntonStoeckl/dynamic-streams-eventstore-go)](https://github.com/AntonStoeckl/dynamic-streams-eventstore-go)
[![Release](https://img.shields.io/github/release-pre/AntonStoeckl/dynamic-streams-eventstore-go.svg)](https://github.com/AntonStoeckl/dynamic-streams-eventstore-go/releases)

A Go-based **Event Store** implementation for **Event Sourcing** with PostgreSQL, operating on **Dynamic Event Streams** (also known as Dynamic Consistency Boundaries).

Unlike traditional event stores with fixed streams tied to specific entities, this approach enables **atomic entity-independent operations** while maintaining strong consistency through PostgreSQL's ACID guarantees.

## ✨ Key Features

- **🔄 Dynamic Event Streams**: Query and modify events across multiple entities atomically
- **⚡ High Performance**: Sub-millisecond queries, ~2.5 ms atomic appends with optimistic locking
- **🛡️ ACID Transactions**: PostgreSQL-backed consistency without distributed transactions
- **🎯 Fluent Filter API**: Type-safe, expressive event filtering with compile-time validation
- **📊 JSON-First**: Efficient JSONB storage with GIN index optimization
- **🔗 Multiple Adapters**: Support for pgx/v5, database/sql, and sqlx database connections
- **📝 Structured Logging**: Configurable SQL query logging and operational monitoring (slog, zerolog, logrus compatible)
- **📝 OpenTelemetry Compatible Contextual Logging**: Context-aware logging with automatic trace correlation
- **📈 OpenTelemetry Compatible Metrics**: Comprehensive observability with duration, counters, error tracking, and context cancellation/timeout detection
- **🔍 OpenTelemetry Compatible Tracing**: Dependency-free tracing interface for OpenTelemetry, Jaeger, and custom backends
- **🔌 OpenTelemetry Ready-to-Use Adapters**: Official plug-and-play adapters for immediate OpenTelemetry integration

## 🚀 Quick Start

```bash
go get github.com/AntonStoeckl/dynamic-streams-eventstore-go
```

```go
// Create event store with pgx adapter (default)
eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool)
if err != nil {
    log.Fatal(err)
}

// Or use alternative adapters:
// eventStore, err := postgresengine.NewEventStoreFromSQLDB(sqlDB)
// eventStore, err := postgresengine.NewEventStoreFromSQLX(sqlxDB)

// Query events spanning multiple entities
filter := BuildEventFilter().
    Matching().
    AnyEventTypeOf(
        "BookCopyAddedToCirculation", "BookCopyRemovedFromCirculation",
        "ReaderRegistered", "BookCopyLentToReader", "BookCopyReturnedByReader").
    AndAnyPredicateOf(
        P("BookID", bookID),
        P("ReaderID", readerID)).
    Finalize()

// Atomic operation: Query → Business Logic → Append
events, maxSeq, _ := eventStore.Query(ctx, filter)
newEvent := applyBusinessLogic(events)
err := eventStore.Append(ctx, filter, maxSeq, newEvent)
```

## 💡 The Dynamic Streams Advantage

**Traditional Event Sourcing:**
```
BookCirculation: [BookCopyAddedToCirculation, BookCopyRemovedFromCirculation, ...]  ← Separate streams
ReaderAccount:   [ReaderRegistered, ReaderContractCanceled, ...]                    ← Separate streams  
BookLending:     [BookCopyLentToReader, BookCopyReturnedByReader, ...]              ← Separate streams
```

**Dynamic Event Streams:**
```
Entity-independent: [BookCopyAddedToCirculation, BookCopyRemovedFromCirculation, ReaderRegistered, 
                     ReaderContractCanceled, BookCopyLentToReader, BookCopyReturnedByReader, ...]  ← Single atomic boundary
```

This eliminates complex synchronization between entities while maintaining strong consistency 
(see [Performance](./docs/performance.md) for detailed benchmarks).  
See **[Core Concepts](./docs/core-concepts.md)** for a more detailed description.

## 🔌 OpenTelemetry Integration

For users with existing OpenTelemetry setups, we provide **ready-to-use adapters** that require zero configuration. The EventStore library uses dependency-free observability interfaces (`Logger`, `ContextualLogger`, `MetricsCollector`, `TracingCollector`) to avoid forcing specific observability dependencies on users. Our OpenTelemetry adapters bridge those interfaces to OpenTelemetry, providing:

- **Zero-config integration** for OpenTelemetry users
- **Automatic trace correlation** in logs
- **Production-ready implementations** using OpenTelemetry best practices
- **Engine-agnostic design** - works with any EventStore engine (PostgreSQL, future MongoDB, etc.)

### Quick Start

```go
import "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/oteladapters"

// Zero-config OpenTelemetry integration
tracer := otel.Tracer("eventstore")
meter := otel.Meter("eventstore")

eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool,
    postgresengine.WithTracing(oteladapters.NewTracingCollector(tracer)),
    postgresengine.WithMetrics(oteladapters.NewMetricsCollector(meter)),
    postgresengine.WithContextualLogger(oteladapters.NewSlogBridgeLogger("eventstore")),
)
```

### Available Adapters

#### 1. Contextual Logger Adapters

**SlogBridgeLogger (Recommended)**
Uses the official OpenTelemetry slog bridge for automatic trace correlation:

```go
// Option 1: Pure OpenTelemetry with automatic trace correlation
logger := oteladapters.NewSlogBridgeLogger("eventstore")

// Option 2: Use your existing slog.Handler (no trace correlation)
slogHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
logger := oteladapters.NewSlogBridgeLoggerWithHandler(slogHandler)
```

**Benefits:**
- ✅ **Automatic trace/span ID injection** when using `NewSlogBridgeLogger()`
- ✅ **Zero configuration** - uses global OpenTelemetry LoggerProvider
- ✅ **Handler compatibility** - `NewSlogBridgeLoggerWithHandler()` for existing setups
- ✅ **Production-ready** with minimal setup

**Trace Correlation Example:**
```json
// Without trace context:
{"level":"INFO","msg":"Query executed","duration":"150ms"}

// With active trace context:
{"level":"INFO","msg":"Query executed","duration":"150ms","trace_id":"abc123","span_id":"def456"}
```

**OTelLogger (Advanced)**
Direct OpenTelemetry logging API integration for when you need direct control over OpenTelemetry log records.

#### 2. Metrics Collector

Maps EventStore metrics to OpenTelemetry instruments:

```go
meter := otel.Meter("eventstore")
collector := oteladapters.NewMetricsCollector(meter)
```

**Instrument Mapping:**
- `RecordDuration(...)` → **Histogram** (for operation durations)
- `IncrementCounter(...)` → **Counter** (for operation counts, errors, cancellations, timeouts)
- `RecordValue(...)` → **Gauge** (for current values, concurrent operations)

**Context Error Detection:**
- **Context Cancellation**: Automatically detects `context.Canceled` errors from user/client cancellations
- **Context Timeout**: Automatically detects `context.DeadlineExceeded` errors from system timeouts
- **Robust Error Handling**: Works with database driver error wrapping (`errors.Join`, custom wrappers)
- **Separate Metrics**: Distinct tracking for cancellations vs timeouts vs regular errors

#### 3. Tracing Collector

Creates OpenTelemetry spans for EventStore operations:

```go
tracer := otel.Tracer("eventstore")
collector := oteladapters.NewTracingCollector(tracer)
```

**Features:**
- Automatic span creation for Query/Append operations
- Context propagation across operations
- Error status mapping and attribute injection
- Proper span lifecycle management

### Production Configuration

For production environments, consider:

```go
// Use OTLP exporters instead of stdout
traceExporter := otlptrace.New(...)
metricExporter := otlpmetric.New(...)
logExporter := otlplog.New(...)

// Configure with environment variables
// OTEL_EXPORTER_OTLP_ENDPOINT=https://your-collector:4317
// OTEL_SERVICE_NAME=your-service
// OTEL_SERVICE_VERSION=1.0.0
// OTEL_ENVIRONMENT=production
```

### Complete Examples

📖 **[Complete OpenTelemetry Setup →](docs/opentelemetry-complete-setup.md)**
- Full OpenTelemetry tracing, metrics, and logging setup
- EventStore integration with all adapters
- Production configuration patterns

📖 **[Slog Integration Examples →](docs/opentelemetry-slog-integration.md)**
- Slog with OpenTelemetry trace correlation
- Slog-only integration (without full OpenTelemetry)
- Custom slog handler integration

## 📚 Documentation

- **[Getting Started](./docs/getting-started.md)** — Installation, setup, and first steps
- **[Core Concepts](./docs/core-concepts.md)** — Understanding Dynamic Event Streams
- **[Usage Examples](./docs/usage-examples.md)** — Real-world implementation patterns
- **[API Reference](./docs/api-reference.md)** — Complete API documentation
- **[Performance](./docs/performance.md)** — Benchmarks and optimization guide
- **[Development](./docs/development.md)** — Contributing and development setup

## 🏗️ Architecture

**Core Components:**
- `eventstore/postgresengine/postgres.go` — PostgreSQL implementation with CTE-based optimistic locking
- `eventstore/postgresengine/internal/adapters/` — Database adapter abstraction (pgx, sql.DB, sqlx)
- `eventstore/filter.go` — Fluent filter builder for entity-independent queries  
- `eventstore/storable_event.go` — Storage-agnostic event DTOs

**Database Adapters:**
The event store supports three PostgreSQL adapters, switchable via factory functions:
- **pgx.Pool** (default): High-performance connection pooling
- **database/sql**: Standard library with lib/pq driver
- **sqlx**: Enhanced database/sql with additional features

**Primary-Replica Support:**
Optional PostgreSQL streaming replication support with context-based query routing:

> **⚠️ CRITICAL RULE**: Always use `WithStrongConsistency()` for command handlers (read-check-write operations) and `WithEventualConsistency()` only for pure query handlers (read-only operations). Mixing these up will cause concurrency conflicts or stale data issues.

```go
import "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"

// Command handlers - require strong consistency (routes to primary)
ctx = eventstore.WithStrongConsistency(ctx)
events, maxSeq, err := eventStore.Query(ctx, filter)
err = eventStore.Append(ctx, filter, maxSeq, newEvent)

// Query handlers - can use eventual consistency (routes to replica)
ctx = eventstore.WithEventualConsistency(ctx)
events, _, err := eventStore.Query(ctx, filter)
```

**Consistency Guarantees:**
- **Strong Consistency** (default): All operations use primary database, ensuring read-after-write consistency
- **Eventual Consistency**: Read operations may use replica database, trading consistency for performance
- **Safe Defaults**: Strong consistency by default prevents subtle bugs in event sourcing scenarios

**Performance Benefits:**
- **Reduced primary load** for read-heavy workloads using replica for queries
- **Optimal load distribution**: Writes to primary, reads from replica
- **Proper consistency guarantees** without performance penalties

**Key Pattern:**
```sql
-- Same WHERE clause used in Query and Append for consistency
WHERE event_type IN ('BookCopyLentToReader', 'ReaderRegistered') 
  AND (payload @> '{"BookID": "123"}' OR payload @> '{"ReaderID": "456"}')
```

## ⚡ Performance

With 10M events in PostgreSQL:
- **Query**: ~0.12 ms average
- **Append**: ~2.55 ms average  
- **Full Workflow**: ~3.56 ms (Query + Business Logic + Append)

See [Performance Documentation](./docs/performance.md) for detailed benchmarks and optimization strategies.

## 🧪 Testing

See [Development Guide](./docs/development.md) for complete testing instructions including adapter switching and benchmarks.


## 🤝 Contributing

See [Development Guide](./docs/development.md) for contribution guidelines, setup instructions, and architecture details.

## 📄 License

This project is licensed under the **GNU GPLv3** — see [LICENSE.txt](LICENSE.txt) for details.

## 🙏 Acknowledgments

Inspired by [Sara Pellegrini](https://sara.event-thinking.io/)'s work on Dynamic Consistency Boundaries and [Rico Fritsche](https://ricofritzsche.me/)'s PostgreSQL CTE implementation patterns.