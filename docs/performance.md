# Performance

Performance characteristics, benchmarks, and optimization strategies for dynamic-streams-eventstore-go.

## Database Adapter Performance

All three supported adapters (pgx.Pool, database/sql, sqlx) provide equivalent performance characteristics. The choice depends on your existing infrastructure and preferences.

## Benchmark Results

Benchmarks run on Linux with 8-core i7-8565U CPU @ 1.80 GHz, 16GB memory, PostgreSQL with 2.5M events.

### Query Performance

```
Benchmark_Query_With_Many_Events_InTheStore
Average: 0.36 ms/query-op
Range: 0.34 - 0.38 ms per query
```

**Key characteristics:**
- Sub-millisecond query times even with millions of events
- Consistent performance due to PostgreSQL's efficient JSON indexing
- Performance scales with filter selectivity, not total event count

### Single Event Appends

```
Benchmark_SingleAppend_With_Many_Events_InTheStore
Average: 3.1 ms/append-op
Range: 2.8 - 3.4 ms per append
```

**Key characteristics:**
- ~3.1 ms for atomic single event append with optimistic locking
- Time includes CTE evaluation and row insertion
- Performance is consistent regardless of database size

### Multiple Event Appends

```
Benchmark_MultipleAppend_With_Many_Events_InTheStore
Average: 4.8 ms/append-op (for 5 events)
Range: 3.9 - 5.9 ms per append operation
```

**Key characteristics:**
- Slightly higher latency for multi-event atomic operations
- All events succeed or fail together
- More efficient than multiple single append operations


## Testing Performance

See [Development Guide](./development.md) for benchmarking with different database adapters.

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

**The GIN index on payload is critical** — it enables fast JSON containment queries (`@>` operator).

### Filter Design

#### Efficient Filters

**✅ Good — Specific predicates:**
```go
// Fast: Uses GIN index effectively
filter := BuildEventFilter().
    Matching().
    AnyPredicateOf(P("BookID", "specific-id")).
    Finalize()
```

**✅ Good — Event type + predicate:**
```go
// Fast: Combines event type index with JSON index
filter := BuildEventFilter().
    Matching().
    AnyEventTypeOf("BookLent", "BookReturned").
    AndAnyPredicateOf(P("BookID", "specific-id")).
    Finalize()
```

#### Less Efficient Filters

**⚠️ Slower — Too broad:**
```go
// Slower: Returns many events
filter := BuildEventFilter().
    Matching().
    AnyEventTypeOf("BookCopyLentToReader").  // Specific event type
    Finalize()
```

**⚠️ Slower — Multiple OR predicates:**
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
- High contention on the same entity
- Many concurrent writers to the same stream
- Complex filters returning many events

### Retry Strategy

```go
func withExponentialBackoff(operation func() error, maxRetries int) error {
    for i := 0; i < maxRetries; i++ {
        err := operation()
        if err == nil {
            return nil
        }
        
        if errors.Is(err, eventstore.ErrConcurrencyConflict) {
            // Exponential backoff: 10 ms, 20 ms, 40 ms, 80 ms
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

### Scaling Strategies

- **Read Replicas**: Use read replicas for queries, primary for appending
- **Partitioning**: Partition events table by event type for a very high scale  
- **Caching**: Cache query results briefly (100 ms TTL) for high-frequency reads

## Monitoring

**Key Metrics:**
- Query/append latencies 
- Concurrency conflict rates
- Database connection usage
- Table/index sizes

**Important Queries:**
- Monitor `pg_stat_statements` for slow queries
- Check `pg_stat_user_indexes` for index usage  
- Track table size with `pg_total_relation_size('events')`

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

## Primary-Replica Performance Benefits

For applications with mixed read/write workloads, primary-replica setups with context-based routing provide significant performance improvements.

### Performance Characteristics

**Command Handlers (Strong Consistency):**
- **Consistent primary performance** for read-check-write operations
- **Proper read-after-write consistency** for optimistic locking
- **No concurrency conflicts** from reading stale replica data
- **Optimal resource utilization** for write-heavy workloads

**Query Handlers (Eventual Consistency):**
- **Offloaded read traffic** from primary to replica
- **Improved primary write capacity** due to reduced read load
- **Horizontal read scaling** across replica nodes
- **Minor performance variation** acceptable for read-only operations

### Load Distribution

**Optimal Routing Pattern:**
```
Primary Database:
- All command handler operations (read + write)
- All append operations
- All operations requiring strong consistency

Replica Database:
- Pure query operations from query handlers
- Read-only projections and reporting
- Operations tolerating eventual consistency
```

**Performance Impact:**
- **Primary Load**: Reduced by query handler traffic (typically 30-70% of total reads)
- **Replica Load**: Handles read-only traffic efficiently
- **Overall System**: Better resource utilization across database cluster

### Consistency Trade-offs

**Strong Consistency (Default):**
- Guarantees read-after-write consistency
- Essential for optimistic concurrency control
- Required for command handlers using read-check-write patterns
- Performance: ~3.1ms average append time

**Eventual Consistency:**
- Accepts potential staleness for performance gains
- Suitable for pure query operations
- Reduces primary database load
- Performance: Replica lag typically <1ms

### Setup Requirements

For optimal performance benefits:
1. **PostgreSQL Streaming Replication**: Sub-millisecond lag preferred
2. **Separate Connection Pools**: Independent primary/replica connections
3. **Explicit Context Usage**: Application handlers must specify consistency requirements
4. **Workload Profiling**: Measure read/write ratio to assess benefits

See [Getting Started](./getting-started.md) for replica setup instructions.

## Performance Testing

The benchmarks in `eventstore/postgresengine/postgres_benchmark_test.go` provide comprehensive performance testing. Run with different adapters using the `ADAPTER_TYPE` environment variable.