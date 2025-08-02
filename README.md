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
- **📈 OpenTelemetry Compatible Metrics**: Comprehensive observability with duration, counters, and error tracking
- **🔍 OpenTelemetry Compatible Tracing**: Dependency-free tracing interface for OpenTelemetry, Jaeger, and custom backends
- **🔌 OpenTelemetry Ready-to-Use Adapters**: Official plug-and-play adapters for immediate OpenTelemetry integration

## 🔌 OpenTelemetry Integration

For users with existing OpenTelemetry setups, we provide **ready-to-use adapters** that require zero configuration:

```bash
# Optional: Add OpenTelemetry adapters
go get github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/oteladapters
```

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

**Features:**
- **Automatic trace correlation** in logs (trace/span IDs included)
- **Engine-agnostic design** - works with any database engine (current and future)
- **Separate module** - no OpenTelemetry dependencies in core library
- **Production-ready** using OpenTelemetry best practices

[📖 View OpenTelemetry Adapters Documentation →](eventstore/oteladapters/README.md)

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

Note: The `example/` directory contains shared domain components and feature examples used by the test suite.

## 🤝 Contributing

See [Development Guide](./docs/development.md) for contribution guidelines, setup instructions, and architecture details.

## 📄 License

This project is licensed under the **GNU GPLv3** — see [LICENSE.txt](LICENSE.txt) for details.

## 🙏 Acknowledgments

Inspired by [Sara Pellegrini](https://sara.event-thinking.io/)'s work on Dynamic Consistency Boundaries and [Rico Fritsche](https://ricofritzsche.me/)'s PostgreSQL CTE implementation patterns.