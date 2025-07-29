# API Reference

## Event Store Interface

### EventStore

The event store supports multiple PostgreSQL database adapters: pgx.Pool, database/sql, and sqlx.DB.

```go
type EventStore struct {
    // Private fields
}

// Factory functions for different database adapters
func NewEventStoreFromPGXPool(pool *pgxpool.Pool, options ...Option) (EventStore, error)
func NewEventStoreFromSQLDB(db *sql.DB, options ...Option) (EventStore, error)
func NewEventStoreFromSQLX(db *sqlx.DB, options ...Option) (EventStore, error)

// Functional options
type Option func(*EventStore) error

func WithTableName(tableName string) Option
```

#### Factory Function Examples

**Using default table name ("events"):**
```go
// Using pgx.Pool
eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool)
if err != nil {
    return err
}

// Using database/sql
eventStore, err := postgresengine.NewEventStoreFromSQLDB(sqlDB)
if err != nil {
    return err
}

// Using sqlx
eventStore, err := postgresengine.NewEventStoreFromSQLX(sqlxDB)
if err != nil {
    return err
}
```

**Using custom table name:**
```go
// Using pgx.Pool with custom table name
eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool, 
    postgresengine.WithTableName("my_events"))
if err != nil {
    return err
}

// Using database/sql with custom table name
eventStore, err := postgresengine.NewEventStoreFromSQLDB(sqlDB,
    postgresengine.WithTableName("my_events"))
if err != nil {
    return err
}

// Using sqlx with custom table name  
eventStore, err := postgresengine.NewEventStoreFromSQLX(sqlxDB,
    postgresengine.WithTableName("my_events"))
if err != nil {
    return err
}
```

#### Methods

##### Query

```go
func (es EventStore) Query(
    ctx context.Context, 
    filter Filter,
) (StorableEvents, MaxSequenceNumberUint, error)
```

Retrieves events matching the filter criteria.

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `filter`: Filter defining which events to retrieve

**Returns:**
- `StorableEvents`: Slice of matching events ordered by sequence number
- `MaxSequenceNumberUint`: Highest sequence number in the filtered stream
- `error`: Any error that occurred during the query

**Example:**
```go
events, maxSeq, err := eventStore.Query(ctx, filter)
if err != nil {
    return err
}
fmt.Printf("Found %d events, max sequence: %d\n", len(events), maxSeq)
```

##### Append

```go
func (es EventStore) Append(
    ctx context.Context,
    filter Filter,
    expectedMaxSequenceNumber MaxSequenceNumberUint,
    event StorableEvent,
) error
```

Appends a single event to the store with optimistic concurrency control.

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `filter`: Same filter used in the Query operation
- `expectedMaxSequenceNumber`: The max sequence number from Query
- `event`: Event to append

**Returns:**
- `error`: `ErrConcurrencyConflict` if stream changed, other errors for failures

**Example:**
```go
err := eventStore.Append(ctx, filter, maxSeq, storableEvent)
if errors.Is(err, engine.ErrConcurrencyConflict) {
    // Retry the operation
    return retry()
}
```

##### AppendMultiple

```go
func (es PostgresEventStore) AppendMultiple(
    ctx context.Context,
    filter Filter,
    expectedMaxSequenceNumber MaxSequenceNumberUint,
    events StorableEvents,
) error
```

Appends multiple events atomically.

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `filter`: Same filter used in the Query operation
- `expectedMaxSequenceNumber`: The max sequence number from Query
- `events`: Events to append (all succeed or all fail)

## Event Types

### StorableEvent

```go
type StorableEvent struct {
    EventType    string
    OccurredAt   time.Time
    PayloadJSON  []byte
    MetadataJSON []byte
}
```

Data transfer object for events stored in the event store.

#### Factory Functions

```go
func BuildStorableEvent(
    eventType string, 
    occurredAt time.Time, 
    payloadJSON []byte, 
    metadataJSON []byte,
) StorableEvent
```

Creates a StorableEvent with full metadata.

```go
func BuildStorableEventWithEmptyMetadata(
    eventType string, 
    occurredAt time.Time, 
    payloadJSON []byte,
) StorableEvent
```

Creates a StorableEvent with empty JSON metadata (`{}`).

### StorableEvents

```go
type StorableEvents = []StorableEvent
```

Type alias for a slice of StorableEvent.

## Filter System

### Filter

```go
type Filter struct {
    // Private fields
}
```

Represents the criteria for querying events.

#### Methods

```go
func (f Filter) Items() []FilterItem
func (f Filter) OccurredFrom() time.Time
func (f Filter) OccurredUntil() time.Time
```

### Filter Builder

The filter system uses a fluent builder pattern to construct type-safe filters.

#### BuildEventFilter

```go
func BuildEventFilter() FilterBuilderStart
```

Starts building a new event filter.

#### Builder Chain

```go
type FilterBuilderStart interface {
    Matching() FilterBuilderMatching
}

type FilterBuilderMatching interface {
    AnyEventTypeOf(eventTypes ...FilterEventTypeString) FilterBuilderWithEventTypes
    AnyPredicateOf(predicates ...FilterPredicate) FilterBuilderWithPredicates
}

type FilterBuilderWithEventTypes interface {
    AndAnyPredicateOf(predicates ...FilterPredicate) FilterBuilderComplete
    AndAllPredicatesOf(predicates ...FilterPredicate) FilterBuilderComplete
}

type FilterBuilderWithPredicates interface {
    Finalize() Filter
}

type FilterBuilderComplete interface {
    Finalize() Filter
}
```

#### Filter Examples

**Query events by entity ID:**
```go
filter := BuildEventFilter().
    Matching().
    AnyPredicateOf(P("BookID", "book-123")).
    Finalize()
```

**Query specific event types for an entity:**
```go
filter := BuildEventFilter().
    Matching().
    AnyEventTypeOf("BookLent", "BookReturned").
    AndAnyPredicateOf(P("BookID", "book-123")).
    Finalize()
```

**Query events affecting multiple entities:**
```go
filter := BuildEventFilter().
    Matching().
    AnyEventTypeOf("BookLent", "BookReturned").
    AndAnyPredicateOf(
        P("BookID", "book-123"),
        P("ReaderID", "reader-456")).
    Finalize()
```

### FilterPredicate

```go
type FilterPredicate struct {
    Key   FilterKeyString
    Value FilterValString
}

func P(key FilterKeyString, value FilterValString) FilterPredicate
```

Helper function to create predicates for JSON payload matching.

**Example:**
```go
P("BookID", "book-123")  // Matches: payload @> '{"BookID": "book-123"}'
```

## Error Types

### Predefined Errors

```go
var ErrEmptyTableNameSupplied = errors.New("empty eventTableName supplied")
var ErrConcurrencyConflict = errors.New("concurrency error, no rows were affected")
```

#### ErrConcurrencyConflict

Returned when an append operation fails due to optimistic concurrency control. This means another process modified the event stream between your Query and Append operations.

**Handling:**
```go
err := eventStore.Append(ctx, filter, maxSeq, event)
if errors.Is(err, engine.ErrConcurrencyConflict) {
    // Retry the entire operation: Query -> Business Logic -> Append
    return retryOperation()
}
```

## Type Aliases

```go
type MaxSequenceNumberUint = uint
type FilterEventTypeString = string
type FilterKeyString = string
type FilterValString = string
```

## Database Configuration

### Connection Setup

The event store requires a PostgreSQL connection pool:

```go
import (
    "github.com/jackc/pgx/v5/pgxpool"
)

config, err := pgxpool.ParseConfig("postgres://user:pass@localhost/db")
if err != nil {
    return err
}

// Optional: Configure pool settings
config.MaxConns = 30
config.MinConns = 5

db, err := pgxpool.NewWithConfig(ctx, config)
if err != nil {
    return err
}
defer db.Close()

eventStore := engine.NewPostgresEventStore(db)
```

### Required Database Schema

```sql
CREATE TABLE events (
    sequence_number BIGSERIAL PRIMARY KEY,
    event_type TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    payload JSONB NOT NULL,
    metadata JSONB NOT NULL
);

-- Performance indexes
CREATE INDEX events_event_type_idx ON events (event_type);
CREATE INDEX events_occurred_at_idx ON events (occurred_at);
CREATE INDEX events_payload_gin_idx ON events USING gin (payload);
```

## Context Support

All operations accept a `context.Context` for:

- **Cancellation**: Cancel long-running operations
- **Timeouts**: Set operation deadlines
- **Tracing**: Propagate trace information

```go
// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

events, maxSeq, err := eventStore.Query(ctx, filter)
```

## Thread Safety

The `PostgresEventStore` is thread-safe and can be used concurrently from multiple goroutines. The underlying pgx connection pool handles concurrent access safely.

## Performance Considerations

### Query Performance

- Use specific event type filters when possible
- JSON predicates use GIN indexes for fast lookup
- Consider the number of events returned (pagination may be needed for large result sets)

### Append Performance

- Single event appends are fastest
- Multiple event appends are atomic but slower
- High contention on the same filter may cause retries

### Connection Pool Tuning

```go
config.MaxConns = 30        // Max concurrent connections
config.MinConns = 5         // Keep minimum connections open
config.MaxConnLifetime = time.Hour
config.MaxConnIdleTime = time.Minute * 30
```