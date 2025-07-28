# dynamic-streams-eventstore-go

[![Go Report Card](https://goreportcard.com/badge/github.com/AntonStoeckl/dynamic-streams-eventstore-go)](https://goreportcard.com/report/github.com/AntonStoeckl/dynamic-streams-eventstore-go)
[![codecov](https://codecov.io/gh/AntonStoeckl/dynamic-streams-eventstore-go/branch/main/graph/badge.svg)](https://codecov.io/gh/AntonStoeckl/dynamic-streams-eventstore-go)
[![GoDoc](https://godoc.org/github.com/AntonStoeckl/dynamic-streams-eventstore-go?status.svg)](https://godoc.org/github.com/AntonStoeckl/dynamic-streams-eventstore-go)
[![License: GPL v3](https://img.shields.io/badge/License-GPL%20v3-green.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Go Version](https://img.shields.io/github/go-mod/go-version/AntonStoeckl/dynamic-streams-eventstore-go)](https://github.com/AntonStoeckl/dynamic-streams-eventstore-go)
[![Release](https://img.shields.io/github/release-pre/AntonStoeckl/dynamic-streams-eventstore-go.svg)](https://github.com/AntonStoeckl/dynamic-streams-eventstore-go/releases)

A Go-based **Event Store** implementation for **Event Sourcing** with PostgreSQL, operating on **Dynamic Event Streams** (also known as Dynamic Consistency Boundaries).

Unlike traditional event stores with fixed streams tied to specific aggregates, this approach enables **atomic cross-entity operations** while maintaining strong consistency through PostgreSQL's ACID guarantees.

## ✨ Key Features

- **🔄 Dynamic Event Streams**: Query and modify events across multiple entities atomically
- **⚡ High Performance**: Sub-millisecond queries, ~2.5 ms atomic appends with optimistic locking
- **🛡️ ACID Transactions**: PostgreSQL-backed consistency without distributed transactions
- **🎯 Fluent Filter API**: Type-safe, expressive event filtering with compile-time validation
- **📊 JSON-First**: Efficient JSONB storage with GIN index optimization

## 🚀 Quick Start

```bash
go get github.com/AntonStoeckl/dynamic-streams-eventstore-go
```

```go
// Query events spanning multiple entities
filter := BuildEventFilter().
    Matching().
    AnyEventTypeOf("BookLent", "BookReturned").
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
BookAggregate: [BookCreated, BookUpdated, ...]     ← Separate streams
UserAggregate: [UserCreated, BookBorrowed, ...]    ← Separate streams
```

**Dynamic Event Streams:**
```
Cross-Entity: [BookCreated, UserCreated, BookBorrowed, ...] ← Single atomic boundary
```

This eliminates the need for complex sagas while maintaining strong consistency for cross-entity business rules.

## 📚 Documentation

- **[Getting Started](./docs/getting-started.md)** - Installation, setup, and first steps
- **[Core Concepts](./docs/core-concepts.md)** - Understanding Dynamic Event Streams
- **[Usage Examples](./docs/usage-examples.md)** - Real-world implementation patterns
- **[API Reference](./docs/api-reference.md)** - Complete API documentation
- **[Performance](./docs/performance.md)** - Benchmarks and optimization guide
- **[Development](./docs/development.md)** - Contributing and development setup

## 🏗️ Architecture

**Core Components:**
- `eventstore/engine/postgres.go` - PostgreSQL implementation with CTE-based optimistic locking
- `eventstore/filter.go` - Fluent filter builder for cross-entity queries  
- `eventstore/storable_event.go` - Storage-agnostic event DTOs

**Key Pattern:**
```postgresql
-- Same WHERE clause used in Query and Append for consistency
WHERE event_type IN ('BookEvent', 'UserEvent') 
  AND (payload @> '{"BookID": "123"}' OR payload @> '{"UserID": "456"}')
```

## ⚡ Performance

With 1M+ events in PostgreSQL:
- **Query**: ~0.12 ms average
- **Append**: ~2.55 ms average  
- **Full Workflow**: ~3.56 ms (Query + Business Logic + Append)

See [Performance Documentation](./docs/performance.md) for detailed benchmarks and optimization strategies.

## 🧪 Testing

```bash
# Start test databases
docker-compose --file test/docker-compose.yml up -d

# Run tests
go test ./eventstore/...

# Run benchmarks  
go test -bench=. ./eventstore/...
```

## 🤝 Contributing

See [Development Guide](./docs/development.md) for contribution guidelines, setup instructions, and architecture details.

## 📄 License

This project is licensed under the **GNU GPLv3** - see [LICENSE.txt](LICENSE.txt) for details.

## 🙏 Acknowledgments

Inspired by [Sara Pellegrini](https://sara.event-thinking.io/)'s work on Dynamic Consistency Boundaries and [Rico Fritsche](https://ricofritzsche.me/)'s PostgreSQL CTE implementation patterns.