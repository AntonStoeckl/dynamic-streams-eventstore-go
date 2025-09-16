# Usage Examples

Practical examples demonstrating entity-independent operations with dynamic-streams-eventstore-go.

**Note:** The minimal test fixtures in `/testutil/eventstore/fixtures/` demonstrate the event types used throughout these examples.

## Example: Library Book Lending System

This example demonstrates operations where a single business transaction affects multiple (logical) entities.

### Domain Events

```go
// BookCirculation entity events
type BookCopyAddedToCirculation struct {
    BookID     string   
    Title      string   
    OccurredAt time.Time
}

type BookCopyRemovedFromCirculation struct {
    BookID     string   
    Reason     string   
    OccurredAt time.Time
}

// ReaderAccount entity events
type ReaderRegistered struct {
    ReaderID   string   
    Name       string   
    OccurredAt time.Time
}

type ReaderContractCanceled struct {
    ReaderID   string   
    Reason     string   
    OccurredAt time.Time
}

// BookLending entity events
type BookCopyLentToReader struct {
    BookID     string   
    ReaderID   string   
    OccurredAt time.Time
}

type BookCopyReturnedByReader struct {
    BookID     string   
    ReaderID   string   
    OccurredAt time.Time
}

// Each event implements EventType() and HasOccurredAt() methods
```

### Core Use Case: Lending a Book

```go
import (
    "context"
    "encoding/json"
    "errors"
    "time"
    
    . "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"
)

func LendBookToReader(ctx context.Context, es EventStore, bookID, readerID string) error {
    // 1. Query entity-independent events  
    filter := BuildEventFilter().
        Matching().
        AnyEventTypeOf(
            "BookCopyAddedToCirculation", "BookCopyRemovedFromCirculation",
            "ReaderRegistered", "ReaderContractCanceled",
            "BookCopyLentToReader", "BookCopyReturnedByReader").
        AndAnyPredicateOf(P("BookID", bookID), P("ReaderID", readerID)).
        Finalize()
    
    events, maxSeq, err := es.Query(ctx, filter)
    if err != nil {
        return err
    }
    
    // 2. Build the current state from events
    bookAvailable, readerBorrowCount := buildStateFromEvents(events, bookID, readerID)
    // ... implementation details of buildStateFromEvents omitted for brevity ...
    
    // 3. Apply business rules
    if !bookAvailable {
        return errors.New("book not available")
    }
    if readerBorrowCount >= 5 { // max quota
        return errors.New("reader quota exceeded")
    }
    
    // 4. Create and append new event atomically
    payloadJSON, err := json.Marshal(map[string]string{"BookID": bookID, "ReaderID": readerID})
    if err != nil {
        return err
    }
    
    lendEvent, err := BuildStorableEventWithEmptyMetadata("BookCopyLentToReader", time.Now(), payloadJSON)
    if err != nil {
        return err
    }
    
    return es.Append(ctx, filter, maxSeq, lendEvent)
}
```

## Error Handling and Concurrency

Handle optimistic concurrency conflicts with retries:

```go
func withRetry(operation func() error, maxRetries int) error {
    for i := 0; i < maxRetries; i++ {
        err := operation()
        if err == nil {
            return nil
        }
        
        // Retry on concurrency conflicts
        if errors.Is(err, eventstore.ErrConcurrencyConflict) {
            time.Sleep(time.Duration(1<<i) * 10 * time.Millisecond)
            continue
        }
        
        return err // Non-retryable error
    }
    return errors.New("max retries exceeded")
}

// Usage
err := withRetry(func() error {
    return LendBookToReader(ctx, es, bookID, readerID)
}, 3)
```

## Observability and Monitoring

### Production Observability Setup

Enable comprehensive monitoring with logging and OpenTelemetry-compatible metrics:

```go
import (
    "log/slog"
    "os"
    "time"
    "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
)

// Production metrics collector (OpenTelemetry-compatible)
type ProductionMetricsCollector struct {
    // Your observability platform integration (Prometheus, DataDog, etc.)
}

func (m *ProductionMetricsCollector) RecordDuration(metric string, duration time.Duration, labels map[string]string) {
    // Record operation durations for performance monitoring
    // metric examples: "eventstore.query.duration", "eventstore.append.duration"
}

func (m *ProductionMetricsCollector) IncrementCounter(metric string, labels map[string]string) {
    // Track operations, errors, and conflicts
    // metric examples: "eventstore.operations.total", "eventstore.errors.total"
}

func (m *ProductionMetricsCollector) RecordValue(metric string, value float64, labels map[string]string) {
    // Monitor business metrics
    // metric examples: "eventstore.events.count", "library.books.lent_total"
}

func setupProductionEventStore(dbPool *pgxpool.Pool) postgresengine.EventStore {
    // Production-safe logger (info level - no sensitive SQL queries)
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    
    // Initialize metrics and tracing collectors
    metricsCollector := &ProductionMetricsCollector{}
    tracingCollector := &ProductionTracingCollector{}
    
    // Create EventStore with full observability
    eventStore, err := postgresengine.NewEventStoreFromPGXPool(dbPool,
        postgresengine.WithLogger(logger),
        postgresengine.WithMetrics(metricsCollector),
        postgresengine.WithTracing(tracingCollector),
        postgresengine.WithContextualLogger(contextualLogger))
    if err != nil {
        panic(err)
    }
    
    return eventStore
}

// Enhanced lending function with observability
func LendBookToReaderWithMetrics(ctx context.Context, es EventStore, bookID, readerID string) error {
    start := time.Now()
    
    // Business logic (same as before)
    err := LendBookToReader(ctx, es, bookID, readerID)
    
    // Optional: Record custom business metrics
    if metrics, ok := es.(interface{ GetMetrics() MetricsCollector }); ok {
        collector := metrics.GetMetrics()
        
        labels := map[string]string{
            "operation": "lend_book",
            "book_id":   bookID,
            "reader_id": readerID,
        }
        
        if err != nil {
            labels["status"] = "error"
            collector.IncrementCounter("library.lending.errors", labels)
        } else {
            labels["status"] = "success"
            collector.IncrementCounter("library.books.lent_total", labels)
            collector.RecordDuration("library.lending.duration", time.Since(start), labels)
        }
    }
    
    return err
}
```

**Key observability benefits:**
- **Performance monitoring**: Track query and append durations
- **Error tracking**: Monitor database errors and concurrency conflicts  
- **Business metrics**: Count successful operations and track patterns
- **Operational insights**: Event counts, sequence numbers, conflict rates

### Testing with Different Adapters

```go
func TestLendBookToReader(t *testing.T) {
    ctx := context.Background()
    
    // Test database setup handles ADAPTER_TYPE env var
    db := setupTestDB(t) 
    es := postgresengine.NewEventStoreFromPGXPool(db) // or another adapter
    
    // Test the lending use case
    err := LendBookToReader(ctx, es, "book-123", "reader-456") 
    assert.NoError(t, err)
}

func TestLendBookToReaderWithMetrics(t *testing.T) {
    ctx := context.Background()
    
    // Setup test database and observability collectors
    db := setupTestDB(t)
    metricsCollector := helper.NewTestMetricsCollector(true)
    tracingCollector := helper.NewTestTracingCollector(true)
    
    es, err := postgresengine.NewEventStoreFromPGXPool(db,
        postgresengine.WithMetrics(metricsCollector),
        postgresengine.WithTracing(tracingCollector),
        postgresengine.WithContextualLogger(contextualLogger))
    require.NoError(t, err)
    
    // Execute business operation
    err = LendBookToReader(ctx, es, "book-123", "reader-456")
    assert.NoError(t, err)
    
    // Verify metrics were recorded
    assert.True(t, metricsCollector.HasDurationRecordForMetric("eventstore.query.duration").
        WithOperation("query").WithStatus("success").Assert())
    assert.True(t, metricsCollector.HasDurationRecordForMetric("eventstore.append.duration").
        WithOperation("append").WithStatus("success").Assert())
    assert.True(t, metricsCollector.HasCounterRecordForMetric("eventstore.operations.total").
        WithOperation("query").WithStatus("success").Assert())
}
```

## Primary-Replica Setup with Context-Based Routing

The EventStore supports optional PostgreSQL primary-replica setups with context-based query routing for performance optimization.

> **⚠️ CRITICAL RULE**: Command handlers MUST use `WithStrongConsistency()` and query handlers MUST use `WithEventualConsistency()`. Never mix these up - command handlers need to read their own writes, while query handlers can tolerate eventual consistency.

### Consistency Context Examples

#### Command Handler Pattern (Strong Consistency)

Command handlers perform read-check-write operations and require strong consistency to ensure they see their own writes:

```go
import "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"

func LendBookToReaderCommand(ctx context.Context, es eventstore.EventStore, bookID, readerID string) error {
    // Explicitly request strong consistency for command handlers
    ctx = eventstore.WithStrongConsistency(ctx)

    filter := BuildEventFilter().
        Matching().
        AnyEventTypeOf("BookCopyAddedToCirculation", "BookCopyLentToReader").
        AndAnyPredicateOf(P("BookID", bookID)).
        Finalize()

    // Query operation routes to primary database
    events, maxSeq, err := es.Query(ctx, filter)
    if err != nil {
        return fmt.Errorf("failed to query events: %w", err)
    }

    // Business logic to determine if book can be lent
    if isBookAlreadyLent(events) {
        return fmt.Errorf("book %s is already lent out", bookID)
    }

    newEvent := BookCopyLentToReader{
        BookID:     bookID,
        ReaderID:   readerID,
        OccurredAt: time.Now(),
    }

    // Append operation uses primary database
    return es.Append(ctx, filter, maxSeq, newEvent)
}
```

#### Query Handler Pattern (Eventual Consistency)

Query handlers perform read-only operations and can use eventual consistency for better performance:

```go
func GetBooksInCirculationQuery(ctx context.Context, es eventstore.EventStore) ([]Book, error) {
    // Explicitly request eventual consistency for query handlers
    ctx = eventstore.WithEventualConsistency(ctx)

    filter := BuildEventFilter().
        Matching().
        AnyEventTypeOf("BookCopyAddedToCirculation", "BookCopyRemovedFromCirculation").
        Finalize()

    // Query operation may route to replica database for performance
    events, _, err := es.Query(ctx, filter)
    if err != nil {
        return nil, fmt.Errorf("failed to query events: %w", err)
    }

    // Project current state from events
    books := projectBooksInCirculation(events)
    return books, nil
}
```

### Consistency Guarantees

**Strong Consistency (Default):**
- All operations use the primary database
- Guarantees read-after-write consistency
- Essential for command handlers using optimistic locking
- Prevents concurrency conflicts from replica lag

**Eventual Consistency:**
- Read operations may use replica database
- Accepts potential data staleness for performance gains
- Suitable for read-only queries that don't require immediate consistency
- Reduces load on primary database

### Performance Characteristics

**Command Handlers with Strong Consistency:**
- Consistent primary database performance
- Proper read-after-write consistency for optimistic locking
- No concurrency conflicts from reading stale replica data

**Query Handlers with Eventual Consistency:**
- Offloads read traffic from primary to replica
- Potential minor performance variation on replica
- Optimal load distribution across database cluster

### Setup Requirements

Primary-replica support is optional and requires:
1. PostgreSQL streaming replication configured
2. EventStore factory with replica connection
3. Context-based routing in application handlers

See [Development Guide](../docs/development.md) for Docker Compose replica setup instructions.