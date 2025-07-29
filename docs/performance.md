# Performance

Performance characteristics, benchmarks, and optimization strategies for dynamic-streams-eventstore-go.

## Database Adapter Performance

All three supported adapters (pgx.Pool, database/sql, sqlx) provide equivalent performance characteristics. The choice depends on your existing infrastructure and preferences.

## Benchmark Results

Benchmarks run on Linux with 8-core i7-8565U CPU @ 1.80GHz, 16GB memory, PostgreSQL with 1M+ events.

### Query Performance

```
Benchmark_Query_With_Many_Events_InTheStore
Average: 0.12 ms/query-op
Range: 0.11 - 0.13 ms per query
```

**Key characteristics:**
- Sub-millisecond query times even with 1M+ events
- Consistent performance due to PostgreSQL's efficient JSON indexing
- Performance scales with filter selectivity, not total event count

### Single Event Append

```
Benchmark_SingleAppend_With_Many_Events_InTheStore  
Average: 2.55 ms/append-op
Range: 2.4 - 2.6 ms per append
```

**Key characteristics:**
- ~2.5ms for atomic single event append with optimistic locking
- Time includes CTE evaluation and row insertion
- Performance is consistent regardless of database size

### Multiple Event Append

```
Benchmark_MultipleAppend_With_Many_Events_InTheStore
Average: 3.17 ms/append-op (for 5 events)
Range: 2.8 - 3.7 ms per append operation
```

**Key characteristics:**
- Slightly higher latency for multi-event atomic operations
- All events succeed or fail together
- More efficient than multiple single appends

### Complete Workflow

```
Benchmark_TypicalWorkload_With_Many_Events_InTheStore
Average: 3.56 ms/total-op
Breakdown:
- Query: ~1.0 ms
- Business Logic: ~0.05 ms  
- Append: ~2.5 ms
```

This represents a complete business operation: Query → Apply Business Logic → Append.

## Testing Performance with Different Adapters

```bash
# Benchmark all adapters
go test -bench=. ./eventstore/postgresengine/                    # pgx.Pool
ADAPTER_TYPE=sqldb go test -bench=. ./eventstore/postgresengine/ # database/sql
ADAPTER_TYPE=sqlx go test -bench=. ./eventstore/postgresengine/  # sqlx
```

## Performance Factors

### Database Indexes

The provided schema includes optimized indexes:

```sql
-- Primary key for ordering
CREATE INDEX events_sequence_number_idx ON events (sequence_number);

-- Event type filtering
CREATE INDEX events_event_type_idx ON events (event_type);

-- Time range queries
CREATE INDEX events_occurred_at_idx ON events (occurred_at);

-- JSON payload queries (most important for performance)
CREATE INDEX events_payload_gin_idx ON events USING gin (payload);
```

**The GIN index on payload is critical** - it enables fast JSON containment queries (`@>` operator).

### Filter Design

#### Efficient Filters

**✅ Good - Specific predicates:**
```go
// Fast: Uses GIN index effectively
filter := BuildEventFilter().
    Matching().
    AnyPredicateOf(P("BookID", "specific-id")).
    Finalize()
```

**✅ Good - Event type + predicate:**
```go
// Fast: Combines event type index with JSON index
filter := BuildEventFilter().
    Matching().
    AnyEventTypeOf("BookLent", "BookReturned").
    AndAnyPredicateOf(P("BookID", "specific-id")).
    Finalize()
```

#### Less Efficient Filters

**⚠️ Slower - Too broad:**
```go
// Slower: Returns many events
filter := BuildEventFilter().
    Matching().
    AnyEventTypeOf("BookCopyLentToReader").  // Specific event type
    Finalize()
```

**⚠️ Slower - Multiple OR predicates:**
```go
// Slower: Multiple JSON containment checks
filter := BuildEventFilter().
    Matching().
    AnyPredicateOf(
        P("BookID", "id1"),
        P("BookID", "id2"),  
        P("BookID", "id3"),
        // ... many more
    ).
    Finalize()
```

### Connection Pool Optimization

```go
config, _ := pgxpool.ParseConfig(connString)

// Tune for your workload
config.MaxConns = 30                    // Match your concurrency needs
config.MinConns = 5                     // Keep connections warm
config.MaxConnLifetime = time.Hour      // Rotate connections periodically
config.MaxConnIdleTime = 30 * time.Minute
config.HealthCheckPeriod = 5 * time.Minute

// For high-throughput scenarios
config.MaxConns = 50
config.MinConns = 10
```

## Concurrency Characteristics

### Optimistic Locking Performance

Dynamic streams use optimistic concurrency control, which has specific performance characteristics:

**✅ High Performance Scenarios:**
- Low contention (different entities)
- Read-heavy workloads  
- Independent business operations

**⚠️ Potential Bottlenecks:**
- High contention on same entity
- Many concurrent writers to same stream
- Complex filters returning many events

### Retry Strategy

```go
func withExponentialBackoff(operation func() error, maxRetries int) error {
    for i := 0; i < maxRetries; i++ {
        err := operation()
        if err == nil {
            return nil
        }
        
        if errors.Is(err, engine.ErrConcurrencyConflict) {
            // Exponential backoff: 10ms, 20ms, 40ms, 80ms
            delay := time.Duration(10<<i) * time.Millisecond
            time.Sleep(delay)
            continue
        }
        
        return err // Non-retryable error
    }
    return errors.New("max retries exceeded")
}
```

## Scaling Patterns

### Horizontal Read Scaling

```go
// Use read replicas for queries
readerDB := setupReadReplica()
writerDB := setupPrimaryDB()

readerEventStore := engine.NewPostgresEventStore(readerDB)
writerEventStore := engine.NewPostgresEventStore(writerDB)

// Queries can use read replicas
events, maxSeq, _ := readerEventStore.Query(ctx, filter)

// But appends must use primary
err := writerEventStore.Append(ctx, filter, maxSeq, event)
```

### Partitioning Strategies

For very high scale, consider partitioning:

```sql
-- Partition by event type
CREATE TABLE events_book_events PARTITION OF events
FOR VALUES IN ('BookLent', 'BookReturned', 'BookAdded');

CREATE TABLE events_user_events PARTITION OF events  
FOR VALUES IN ('UserRegistered', 'UserDeactivated');
```

### Caching Strategies

#### Event Caching
```go
type CachedEventStore struct {
    store engine.PostgresEventStore
    cache map[string]CacheEntry
    mu    sync.RWMutex
}

func (c *CachedEventStore) Query(ctx context.Context, filter Filter) (StorableEvents, MaxSequenceNumberUint, error) {
    key := filterKey(filter)
    
    c.mu.RLock()
    if entry, exists := c.cache[key]; exists && !entry.Expired() {
        c.mu.RUnlock()
        return entry.Events, entry.MaxSeq, nil
    }
    c.mu.RUnlock()
    
    // Cache miss - query database
    events, maxSeq, err := c.store.Query(ctx, filter)
    if err != nil {
        return nil, 0, err
    }
    
    // Cache result briefly
    c.mu.Lock()
    c.cache[key] = CacheEntry{
        Events: events,
        MaxSeq: maxSeq,
        Expires: time.Now().Add(100 * time.Millisecond), // Short TTL
    }
    c.mu.Unlock()
    
    return events, maxSeq, nil
}
```

## Monitoring and Observability

### Key Metrics to Track

```go
// Operation latencies
queryDuration := prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "eventstore_query_duration_seconds",
        Help: "Time spent querying events",
    },
    []string{"filter_type"},
)

appendDuration := prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "eventstore_append_duration_seconds", 
        Help: "Time spent appending events",
    },
    []string{"event_count"},
)

// Concurrency conflicts
concurrencyConflicts := prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "eventstore_concurrency_conflicts_total",
        Help: "Number of concurrency conflicts",
    },
    []string{"filter_type"},
)

// Database connections
dbConnections := prometheus.NewGaugeVec(
    prometheus.GaugeOpts{
        Name: "eventstore_db_connections",
        Help: "Current database connections",
    },
    []string{"status"}, // active, idle, etc.
)
```

### Database Monitoring

```sql
-- Monitor query performance
SELECT 
    query,
    mean_exec_time,
    calls,
    total_exec_time
FROM pg_stat_statements 
WHERE query LIKE '%events%'
ORDER BY mean_exec_time DESC;

-- Monitor index usage
SELECT 
    indexrelname,
    idx_tup_read,
    idx_tup_fetch
FROM pg_stat_user_indexes 
WHERE relname = 'events';

-- Monitor table size
SELECT 
    pg_size_pretty(pg_total_relation_size('events')) as table_size,
    pg_size_pretty(pg_relation_size('events')) as data_size;
```

## Optimization Recommendations

### For High-Throughput Applications

1. **Tune Connection Pool**: Match `MaxConns` to your concurrency level
2. **Use Specific Filters**: Avoid broad event type queries
3. **Implement Retry Logic**: Handle concurrency conflicts gracefully
4. **Monitor GIN Index**: Ensure JSON queries use the index
5. **Consider Read Replicas**: For read-heavy workloads

### For Low-Latency Applications

1. **Keep Connections Warm**: Set appropriate `MinConns`
2. **Use Connection Pooling**: Don't create new connections per request
3. **Optimize Business Logic**: Minimize time between Query and Append
4. **Consider Caching**: For frequently accessed, stable event streams

### For High-Scale Applications

1. **Partition Tables**: Split by event type or entity type
2. **Archive Old Events**: Move historical data to separate tables
3. **Use Multiple Databases**: Shard by business domain
4. **Implement CQRS**: Separate read and write models

## Performance Testing

### Load Testing Setup

```go
func BenchmarkConcurrentAppends(b *testing.B) {
    db := setupTestDB()
    es := engine.NewPostgresEventStore(db)
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            // Simulate concurrent appends
            bookID := generateUniqueID()
            filter := buildFilter(bookID)
            
            events, maxSeq, _ := es.Query(ctx, filter)
            event := createTestEvent(bookID)
            
            err := es.Append(ctx, filter, maxSeq, event)
            if errors.Is(err, engine.ErrConcurrencyConflict) {
                // Expected under high concurrency
                continue
            }
            if err != nil {
                b.Error(err)
            }
        }
    })
}
```

The benchmarks in `eventstore/engine/postgres_benchmark_test.go` provide comprehensive performance testing that you can run in your own environment.