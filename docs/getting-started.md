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
-- See testutil/postgresengine/initdb/init.sql for the complete schema
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

##### Full Configuration
Combining custom table name with logging:

```go
// Create logger appropriate for your environment
debugLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

// Use logger with other options
eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool,
    postgresengine.WithTableName("my_events"),
    postgresengine.WithLogger(debugLogger))
if err != nil {
    log.Fatal(err)
}
```

**What gets logged:**
- **Debug level**: Full SQL queries with execution timing (sensitive, development only)
- **Info level**: Event counts, sequence numbers, durations, concurrency conflicts (production-safe)

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

## Testing

See [Development Guide](./development.md) for testing with different database adapters.

## Next Steps

- Read the [Core Concepts](./core-concepts.md) to understand Dynamic Event Streams
- See [Usage Examples](./usage-examples.md) for more complex scenarios  
- Review [API Reference](./api-reference.md) for detailed documentation
- Check [Development](./development.md) for contribution guidelines

**Note:** The `/example` directory contains test fixtures and simple usage examples that demonstrate the library's capabilities.