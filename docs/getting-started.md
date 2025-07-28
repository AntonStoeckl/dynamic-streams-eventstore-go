# Getting Started

This guide will help you get up and running with dynamic-streams-eventstore-go quickly.

## Prerequisites

- **Go 1.24+**
- **PostgreSQL 17+** (or Docker for testing)
- Basic understanding of Event Sourcing concepts

## Installation

Add the library to your Go project:

```bash
go get github.com/AntonStoeckl/dynamic-streams-eventstore-go
```

## Quick Start

### 1. Database Setup

Create a PostgreSQL database with the required table structure. You can use the initialization script from this repository:

```sql
-- See test/initdb/init.sql for the complete schema
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
    "log"
    
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/engine"
)

func main() {
    ctx := context.Background()
    
    // Configure your database connection
    config, err := pgxpool.ParseConfig("postgres://user:password@localhost/eventstore")
    if err != nil {
        log.Fatal(err)
    }
    
    db, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // Create event store
    eventStore := engine.NewPostgresEventStore(db)
}
```

### 3. Define Your Domain Events

```go
package events

import "time"

type BookAddedEvent struct {
    BookID    string    `json:"BookID"`
    Title     string    `json:"Title"`
    Author    string    `json:"Author"`
    OccurredAt time.Time `json:"OccurredAt"`
}

func (e BookAddedEvent) EventType() string {
    return "BookAdded"
}

func (e BookAddedEvent) HasOccurredAt() time.Time {
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
event := BookAddedEvent{
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

storableEvent := BuildStorableEventWithEmptyMetadata(
    event.EventType(),
    event.HasOccurredAt(),
    payloadJSON,
)

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

## Next Steps

- Read the [Core Concepts](./core-concepts.md) to understand Dynamic Event Streams
- See [Usage Examples](./usage-examples.md) for more complex scenarios
- Check [Configuration](./configuration.md) for production setup
- Review [API Reference](./api-reference.md) for detailed documentation