# Getting Started

This guide will help you get up and running with dynamic-streams-eventstore-go quickly.

## Prerequisites

- **Go 1.24+**
- **PostgreSQL 17+** (or Docker for testing)
- Basic understanding of Event Sourcing concepts
- One of the supported database drivers: pgx/v5, lib/pq, or sqlx

## Installation

Add the library to your Go project:

```bash
go get github.com/AntonStoeckl/dynamic-streams-eventstore-go
```

## Quick Start

### 1. Database Setup

Create a PostgreSQL database with the required table structure. You can use the initialization script from this repository:

```sql
-- See testutil/postgresengine/initdb/01-init.sql for the complete schema
CREATE TABLE events (
    sequence_number BIGSERIAL PRIMARY KEY,
    event_type TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    payload JSONB NOT NULL,
    metadata JSONB NOT NULL
);

-- Add indexes for performance
CREATE INDEX events_event_type_idx ON events (event_type);
CREATE INDEX events_occurred_at_idx ON events (occurred_at);
CREATE INDEX events_payload_gin_idx ON events USING gin (payload);
```

### 2. Connect to Database

```go
package main

import (
    "context"
    "database/sql"
    "log"
    
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
    "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
)

func main() {
    ctx := context.Background()
    
    // Option 1: Using pgx.Pool (recommended)
    config, err := pgxpool.ParseConfig("postgres://user:password@localhost/eventstore")
    if err != nil {
        log.Fatal(err)
    }
    
    pgxPool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        log.Fatal(err)
    }
    defer pgxPool.Close()
    
    eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool)
    if err != nil {
        log.Fatal(err)
    }
    
    // For alternative adapters (database/sql, sqlx), see API Reference documentation
}
```

#### Using Custom Table Name

If you need to use a different table name instead of the default "events":

```go
// Using pgx.Pool with custom table name
eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool, 
    postgresengine.WithTableName("my_events"))
if err != nil {
    log.Fatal(err)
}

// Using database/sql with custom table name
eventStore, err := postgresengine.NewEventStoreFromSQLDB(sqlDB,
    postgresengine.WithTableName("my_events"))
if err != nil {
    log.Fatal(err)
}

// Using sqlx with custom table name
eventStore, err := postgresengine.NewEventStoreFromSQLX(sqlxDB,
    postgresengine.WithTableName("my_events"))
if err != nil {
    log.Fatal(err)
}
```

#### Optional Logging

The EventStore supports unified logging through a single logger that handles both SQL debugging and operational monitoring:

##### Development Logging (Debug Level)
For debugging during development, use a debug-level logger to see both SQL queries and operational metrics:

```go
import "log/slog"

// Create a debug logger to see SQL queries and operational metrics
debugLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

// Enable comprehensive logging for development
eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool, 
    postgresengine.WithLogger(debugLogger))
if err != nil {
    log.Fatal(err)
}
```

##### Production Logging (Info Level)
For production monitoring, use an info-level logger to see only operational metrics without exposing sensitive SQL:

```go
import "log/slog"

// Create an info logger for production-safe operational metrics
opsLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

// Enable operational logging for production
eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool, 
    postgresengine.WithLogger(opsLogger))
if err != nil {
    log.Fatal(err)
}
```

##### OpenTelemetry-Compatible Metrics

The EventStore provides comprehensive observability through OpenTelemetry-compatible metrics instrumentation, enabling integration with modern monitoring platforms:

```go
import "time"

// Example metrics collector (implement your OpenTelemetry integration)
type MyMetricsCollector struct {
    // Your metrics implementation (Prometheus, DataDog, New Relic, etc.)
}

func (m *MyMetricsCollector) RecordDuration(metric string, duration time.Duration, labels map[string]string) {
    // Record operation durations
}

func (m *MyMetricsCollector) IncrementCounter(metric string, labels map[string]string) {
    // Count operations, errors, conflicts
}

func (m *MyMetricsCollector) RecordValue(metric string, value float64, labels map[string]string) {
    // Track event counts, sequence numbers
}

// Enable metrics instrumentation
metricsCollector := &MyMetricsCollector{}
eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool,
    postgresengine.WithMetrics(metricsCollector))
if err != nil {
    log.Fatal(err)
}
```

**Metrics automatically collected:**
- Operation durations (`eventstore.query.duration`, `eventstore.append.duration`)
- Operation counters (`eventstore.operations.total`, `eventstore.errors.total`)
- Event counts and sequence numbers (`eventstore.events.count`)
- Concurrency conflicts (`eventstore.concurrency_conflicts.total`)

##### Distributed Tracing Support

The EventStore supports distributed tracing through a dependency-free interface that integrates with any tracing backend:

```go
import "context"

// Example tracing collector (implement your tracing backend integration)
type MyTracingCollector struct {
    // Your tracing implementation (OpenTelemetry, Jaeger, Zipkin, etc.)
}

func (t *MyTracingCollector) StartSpan(ctx context.Context, name string, attrs map[string]string) (context.Context, postgresengine.SpanContext) {
    // Start a new span and return updated context
    // Implementation depends on your tracing backend
}

func (t *MyTracingCollector) FinishSpan(spanCtx postgresengine.SpanContext, status string, attrs map[string]string) {
    // Finish the span with final status and attributes
}

// Enable tracing instrumentation
tracingCollector := &MyTracingCollector{}
eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool,
    postgresengine.WithTracing(tracingCollector))
if err != nil {
    log.Fatal(err)
}
```

**Tracing information automatically captured:**
- Query operations with filter details, event counts, and durations
- Append operations with event types, sequence numbers, and success/error status
- Error scenarios with detailed error classification
- Context propagation through all database operations

##### OpenTelemetry-Compatible Contextual Logging

The EventStore supports context-aware logging that automatically correlates with distributed tracing for unified observability:

```go
import "context"

// Example contextual logger (implement your OpenTelemetry logging integration)
type MyContextualLogger struct {
    // Your OpenTelemetry logging implementation
}

func (l *MyContextualLogger) DebugContext(ctx context.Context, msg string, args ...any) {
    // Log with automatic trace correlation from context
    // OpenTelemetry loggers automatically extract trace_id and span_id
}

func (l *MyContextualLogger) InfoContext(ctx context.Context, msg string, args ...any) {
    // Log operational information with trace correlation
}

func (l *MyContextualLogger) WarnContext(ctx context.Context, msg string, args ...any) {
    // Log warnings with trace correlation
}

func (l *MyContextualLogger) ErrorContext(ctx context.Context, msg string, args ...any) {
    // Log errors with trace correlation
}

// Enable contextual logging
contextualLogger := &MyContextualLogger{}
eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool,
    postgresengine.WithContextualLogger(contextualLogger))
if err != nil {
    log.Fatal(err)
}
```

**Benefits of contextual logging:**
- Automatic trace correlation - logs include trace_id and span_id when tracing is enabled
- Unified observability - logs, metrics, and traces all correlated together
- Better debugging - jump from log entries to distributed traces
- Production observability - complete request flow visibility

##### Full Configuration
Combining custom table name with logging, metrics, tracing, and contextual logging:

```go
// Create logger appropriate for your environment
debugLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

// Create your metrics collector
metricsCollector := &MyMetricsCollector{}

// Create your tracing collector
tracingCollector := &MyTracingCollector{}

// Create your contextual logger
contextualLogger := &MyContextualLogger{}

// Full observability configuration
eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool,
    postgresengine.WithTableName("my_events"),
    postgresengine.WithLogger(debugLogger),
    postgresengine.WithMetrics(metricsCollector),
    postgresengine.WithTracing(tracingCollector),
    postgresengine.WithContextualLogger(contextualLogger))
if err != nil {
    log.Fatal(err)
}
```

**What gets logged:**
- **Debug level**: Full SQL queries with execution timing (sensitive, development only)
- **Info level**: Event counts, sequence numbers, durations, concurrency conflicts (production-safe)
- **Warn level**: Non-critical issues like cleanup failures
- **Error level**: Critical failures that cause operation failures

### 3. Define Your Domain Events

```go
package events

import "time"

type BookCopyAddedToCirculation struct {
    BookID     string   
    Title      string   
    Author     string   
    OccurredAt time.Time
}

func (e BookCopyAddedToCirculation) EventType() string {
    return "BookCopyAddedToCirculation"
}

func (e BookCopyAddedToCirculation) HasOccurredAt() time.Time {
    return e.OccurredAt
}
```

### 4. Create and Store Events

```go
import (
    "encoding/json"
    "time"
    
    . "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

// Create a domain event
event := BookCopyAddedToCirculation{
    BookID:     "book-123",
    Title:      "Domain-Driven Design",
    Author:     "Eric Evans",
    OccurredAt: time.Now(),
}

// Convert to storable event
payloadJSON, err := json.Marshal(event)
if err != nil {
    log.Fatal(err)
}

storableEvent, err := BuildStorableEventWithEmptyMetadata(
    event.EventType(),
    event.HasOccurredAt(),
    payloadJSON,
)
if err != nil {
    log.Fatal(err)
}

// Create filter for this book
filter := BuildEventFilter().
    Matching().
    AnyPredicateOf(P("BookID", "book-123")).
    Finalize()

// Append to event store
err = eventStore.Append(ctx, filter, 0, storableEvent)
if err != nil {
    log.Fatal(err)
}
```

### 5. Query Events

```go
// Query events for a book
storableEvents, maxSeqNum, err := eventStore.Query(ctx, filter)
if err != nil {
    log.Fatal(err)
}

// Process events
for _, storableEvent := range storableEvents {
    fmt.Printf("Event: %s at %v\n", 
        storableEvent.EventType, 
        storableEvent.OccurredAt)
}
```

## Optional: Primary-Replica Setup

For high-performance applications, you can configure PostgreSQL streaming replication with context-based query routing.

### Benefits

- **Load Distribution**: Read queries can use replica, reducing primary load
- **Scalability**: Better resource utilization across database cluster
- **Proper Consistency**: Strong consistency by default prevents subtle bugs

### Setup Requirements

1. **PostgreSQL Streaming Replication**: Primary and replica databases configured
2. **Separate Connections**: Connection pools for both primary and replica
3. **Context-Based Routing**: Application handlers specify consistency requirements

### Basic Configuration

```go
// Create separate connection pools
primaryPool, err := pgxpool.New(ctx, "postgres://user:pass@primary:5432/events")
if err != nil {
    log.Fatal(err)
}

replicaPool, err := pgxpool.New(ctx, "postgres://user:pass@replica:5432/events")
if err != nil {
    log.Fatal(err)
}

// Create EventStore with replica support
eventStore, err := postgresengine.NewEventStoreFromPGXPoolWithReplica(
    primaryPool,
    replicaPool)
if err != nil {
    log.Fatal(err)
}
```

### Usage with Consistency Context

> **⚠️ CRITICAL RULE**: Command handlers MUST use `WithStrongConsistency()` - they need read-after-write consistency for optimistic locking. Query handlers can use `WithEventualConsistency()` - they only read data and can tolerate slight staleness.

```go
import "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"

// Command handlers - use strong consistency (primary)
func HandleLendBookCommand(ctx context.Context, es EventStore, bookID, readerID string) error {
    ctx = eventstore.WithStrongConsistency(ctx)

    // This routes to primary database
    events, maxSeq, err := es.Query(ctx, filter)
    // ... business logic ...
    return es.Append(ctx, filter, maxSeq, newEvent)
}

// Query handlers - use eventual consistency (replica)
func HandleGetBooksQuery(ctx context.Context, es EventStore) ([]Book, error) {
    ctx = eventstore.WithEventualConsistency(ctx)

    // This may route to replica database
    events, _, err := es.Query(ctx, filter)
    // ... projection logic ...
    return books, nil
}
```

### Docker Development Setup

For local development, you can use the provided Docker Compose configuration:

```bash
# Start primary and replica databases
cd testutil/postgresengine
docker-compose up -d postgres_benchmark_master postgres_benchmark_replica

# Primary: localhost:5433
# Replica: localhost:5434
```

See [Development Guide](./development.md) for complete Docker setup instructions.

## Testing

See [Development Guide](./development.md) for testing with different database adapters.

## Next Steps

- Read the [Core Concepts](./core-concepts.md) to understand Dynamic Event Streams
- See [Usage Examples](./usage-examples.md) for more complex scenarios  
- Review [API Reference](./api-reference.md) for detailed documentation
- Check [Development](./development.md) for contribution guidelines

**Note:** The `/example` directory contains test fixtures and simple usage examples that demonstrate the library's capabilities.