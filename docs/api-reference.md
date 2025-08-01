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
func WithLogger(logger Logger) Option
func WithMetrics(metrics MetricsCollector) Option

// Logger interface for SQL query logging, operational metrics, warnings, and error reporting
type Logger interface {
    Debug(msg string, args ...any)
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
}

// MetricsCollector interface for OpenTelemetry-compatible observability instrumentation
type MetricsCollector interface {
    RecordDuration(metric string, duration time.Duration, labels map[string]string)
    IncrementCounter(metric string, labels map[string]string)
    RecordValue(metric string, value float64, labels map[string]string)
}
```

#### Factory Function Examples

**Using the default table name ("events"):**
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

**Using logger for debugging and monitoring:**

The EventStore supports unified logging through a single logger that handles SQL debugging, operational monitoring, warnings, and error reporting:
- **Debug level**: SQL queries with execution timing (for development and debugging)
- **Info level**: Operational metrics like event counts and durations (production-safe)
- **Warn level**: Non-critical issues like cleanup failures
- **Error level**: Critical failures that cause operation failures

```go
import "log/slog"

// Create logger with appropriate level for your environment
// Development: Debug level to see SQL queries
debugLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

// Production: Info level for operational metrics only
opsLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

// Development setup: See both SQL queries (debug) and operations (info)
eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool, 
    postgresengine.WithLogger(debugLogger))
if err != nil {
    return err
}

// Production setup: See only operational metrics (info level)
eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool,
    postgresengine.WithLogger(opsLogger))
if err != nil {
    return err
}

// Full configuration with all options
eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool,
    postgresengine.WithTableName("my_events"),
    postgresengine.WithLogger(debugLogger),
    postgresengine.WithMetrics(metricsCollector))
if err != nil {
    return err
}
```

**Example logging output:**

With debug-level logger (shows all message levels):
```
time=2024-01-01T12:00:00.000Z level=DEBUG msg="executed sql for: query" duration_ms=0.123 query="SELECT ..."
time=2024-01-01T12:00:00.000Z level=INFO msg="eventstore operation: query completed" event_count=5 duration_ms=0.123
time=2024-01-01T12:00:01.000Z level=DEBUG msg="executed sql for: append" duration_ms=2.456 query="INSERT ..."
time=2024-01-01T12:00:01.000Z level=INFO msg="eventstore operation: events appended" event_count=1 duration_ms=2.456
time=2024-01-01T12:00:02.000Z level=WARN msg="failed to close database rows" error="connection closed"
time=2024-01-01T12:00:03.000Z level=ERROR msg="database query execution failed" error="syntax error" query="SELECT ..."
```

With info-level logger (shows info, warn, and error messages):
```
time=2024-01-01T12:00:00.000Z level=INFO msg="eventstore operation: query completed" event_count=5 duration_ms=0.123
time=2024-01-01T12:00:01.000Z level=INFO msg="eventstore operation: events appended" event_count=1 duration_ms=2.456
time=2024-01-01T12:00:02.000Z level=INFO msg="eventstore operation: concurrency conflict detected" expected_events=1 rows_affected=0 expected_sequence=42
time=2024-01-01T12:00:03.000Z level=WARN msg="failed to close database rows" error="connection closed"
time=2024-01-01T12:00:04.000Z level=ERROR msg="database execution failed during event append" error="constraint violation" query="INSERT ..."
```

**Using OpenTelemetry-compatible metrics for observability:**

The EventStore supports comprehensive metrics instrumentation that follows OpenTelemetry standards, enabling integration with modern observability platforms like Prometheus, DataDog, New Relic, and others.

```go
import (
    "context"
    "time"
    "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
)

// Example metrics collector implementation (OpenTelemetry-compatible)
type MyMetricsCollector struct {
    // Your OpenTelemetry metrics implementation
}

func (m *MyMetricsCollector) RecordDuration(metric string, duration time.Duration, labels map[string]string) {
    // Record duration metrics (histograms/timers)
    // Examples: "eventstore.query.duration", "eventstore.append.duration"
}

func (m *MyMetricsCollector) IncrementCounter(metric string, labels map[string]string) {
    // Increment counter metrics
    // Examples: "eventstore.operations.total", "eventstore.errors.total"
}

func (m *MyMetricsCollector) RecordValue(metric string, value float64, labels map[string]string) {
    // Record gauge/value metrics
    // Examples: "eventstore.events.count", "eventstore.sequence.number"
}

// Enable metrics instrumentation
metricsCollector := &MyMetricsCollector{}
eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool,
    postgresengine.WithMetrics(metricsCollector))
if err != nil {
    return err
}

// Combined observability: logging + metrics
eventStore, err := postgresengine.NewEventStoreFromPGXPool(pgxPool,
    postgresengine.WithLogger(logger),
    postgresengine.WithMetrics(metricsCollector))
if err != nil {
    return err
}
```

**Metrics collected by the EventStore:**

The EventStore automatically instruments the following metrics with OpenTelemetry-compatible labels:

- **Duration Metrics:**
  - `eventstore.query.duration` - Query operation execution time
  - `eventstore.append.duration` - Append operation execution time
  - `eventstore.append_multiple.duration` - Multi-event append execution time

- **Counter Metrics:**
  - `eventstore.operations.total` - Total operations performed (query/append)
  - `eventstore.errors.total` - Total errors encountered
  - `eventstore.concurrency_conflicts.total` - Optimistic concurrency conflicts

- **Value/Gauge Metrics:**
  - `eventstore.events.count` - Number of events processed
  - `eventstore.sequence.number` - Sequence numbers for tracking

**Standard Labels (OpenTelemetry-compatible):**
- `operation`: "query", "append", "append_multiple"
- `status`: "success", "error", "conflict"
- `error_type`: "database_error", "validation_error", etc.
- `conflict_type`: "concurrency_conflict"

**Example metrics output:**
```
# Query operation
eventstore.query.duration{operation="query",status="success"} 0.025
eventstore.operations.total{operation="query",status="success"} 1
eventstore.events.count{operation="query"} 5

# Append operation with concurrency conflict
eventstore.append.duration{operation="append",status="conflict"} 0.012
eventstore.operations.total{operation="append",status="conflict"} 1
eventstore.concurrency_conflicts.total{operation="append",conflict_type="concurrency_conflict"} 1

# Error scenario
eventstore.append.duration{operation="append",status="error"} 0.008
eventstore.operations.total{operation="append",status="error"} 1
eventstore.errors.total{operation="append",error_type="database_error"} 1
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
if errors.Is(err, eventstore.ErrConcurrencyConflict) {
    // Retry the operation
    return retry()
}
```

##### AppendMultiple

```go
func (es EventStore) AppendMultiple(
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
) (StorableEvent, error)
```

Creates a StorableEvent with full metadata. Returns an error if payloadJSON or metadataJSON are not valid JSON.

**Example:**
```go
storableEvent, err := BuildStorableEvent("BookCopyLentToReader", time.Now(), payloadJSON, metadataJSON)
if err != nil {
    // Handle JSON validation error
    return err
}
```

```go
func BuildStorableEventWithEmptyMetadata(
    eventType string, 
    occurredAt time.Time, 
    payloadJSON []byte,
) (StorableEvent, error)
```

Creates a StorableEvent with empty JSON metadata (`{}`). Returns an error if payloadJSON is not valid JSON.

**Example:**
```go
storableEvent, err := BuildStorableEventWithEmptyMetadata("BookCopyLentToReader", time.Now(), payloadJSON)
if err != nil {
    // Handle JSON validation error
    return err
}
```

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
    AnyEventTypeOf("BookCopyLentToReader", "BookCopyReturnedByReader").
    AndAnyPredicateOf(P("BookID", "book-123")).
    Finalize()
```

**Query events affecting multiple entities:**
```go
filter := BuildEventFilter().
    Matching().
    AnyEventTypeOf("BookCopyLentToReader", "BookCopyReturnedByReader").
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
// From eventstore/common.go
var ErrEmptyEventsTableName = errors.New("events table name must not be empty")
var ErrConcurrencyConflict = errors.New("concurrency error, no rows were affected")
var ErrNilDatabaseConnection = errors.New("database connection must not be nil")

// From eventstore/storable_event.go
var ErrInvalidPayloadJSON = errors.New("payload json is not valid")
var ErrInvalidMetadataJSON = errors.New("metadata json is not valid")
```

#### ErrConcurrencyConflict

Returned when an append operation fails due to optimistic concurrency control. This means another process modified the event stream between your Query and Append operations.

#### ErrInvalidPayloadJSON / ErrInvalidMetadataJSON

Returned by `BuildStorableEvent` and `BuildStorableEventWithEmptyMetadata` when the provided JSON data is not valid.

**Handling:**
```go
import "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"

// Concurrency conflict handling
err := eventStore.Append(ctx, filter, maxSeq, event)
if errors.Is(err, eventstore.ErrConcurrencyConflict) {
    // Retry the entire operation: Query -> Business Logic -> Append
    return retryOperation()
}

// JSON validation error handling
storableEvent, err := BuildStorableEvent(eventType, time.Now(), payloadJSON, metadataJSON)
if errors.Is(err, eventstore.ErrInvalidPayloadJSON) {
    // Handle invalid payload JSON
    return fmt.Errorf("invalid payload: %w", err)
}
if errors.Is(err, eventstore.ErrInvalidMetadataJSON) {
    // Handle invalid metadata JSON  
    return fmt.Errorf("invalid metadata: %w", err)
}
```

## Type Aliases

```go
type MaxSequenceNumberUint = uint
type FilterEventTypeString = string
type FilterKeyString = string
type FilterValString = string
```

## Database Schema

The event store requires this PostgreSQL table:

```sql
CREATE TABLE events (
    sequence_number BIGSERIAL PRIMARY KEY,
    event_type TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    payload JSONB NOT NULL,
    metadata JSONB NOT NULL
);

-- Required indexes
CREATE INDEX events_event_type_idx ON events (event_type);
CREATE INDEX events_occurred_at_idx ON events (occurred_at);
CREATE INDEX events_payload_gin_idx ON events USING gin (payload);
```

## Notes

- All operations are thread-safe
- Use `context.Context` for timeouts and cancellation
- The GIN index on payload is critical for performance
- See [Performance](./performance.md) for detailed benchmarks