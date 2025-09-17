# Primary-Replica Support

PostgreSQL Primary-Replica support enables horizontal read scaling by routing queries between primary and replica databases based on consistency requirements.

## Overview

The EventStore supports PostgreSQL streaming replication setups with intelligent query routing:

- **Primary Database**: Handles all writes and strong consistency reads
- **Replica Database**: Handles eventual consistency reads for better performance
- **Context-Based Routing**: Use context to specify consistency requirements
- **Safe Defaults**: Strong consistency by default prevents subtle event sourcing bugs

## Benefits

- **Load Distribution**: Read queries can use replica, reducing primary load
- **Better Performance**: Replica queries don't compete with writes on primary
- **Scalability**: Better resource utilization across database cluster
- **Production Ready**: Built for high-traffic event-sourced applications

## Configuration

### pgx.Pool (Recommended)

```go
import (
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
)

// Create primary and replica pools
primaryPool, err := pgxpool.New(ctx, "postgres://user:pass@primary-host:5432/eventstore")
if err != nil {
    log.Fatal(err)
}

replicaPool, err := pgxpool.New(ctx, "postgres://user:pass@replica-host:5432/eventstore")
if err != nil {
    log.Fatal(err)
}

// Create EventStore with replica support
eventStore, err := postgresengine.NewEventStoreFromPGXPoolAndReplica(
    primaryPool,
    replicaPool,
    // Optional: Add observability, custom table names, etc.
)
if err != nil {
    log.Fatal(err)
}
```

### database/sql

```go
import (
    "database/sql"
    _ "github.com/lib/pq"
    "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
)

// Create primary and replica connections
primaryDB, err := sql.Open("postgres", "postgres://user:pass@primary-host:5432/eventstore")
if err != nil {
    log.Fatal(err)
}

replicaDB, err := sql.Open("postgres", "postgres://user:pass@replica-host:5432/eventstore")
if err != nil {
    log.Fatal(err)
}

// Configure connection pools
primaryDB.SetMaxOpenConns(60)
primaryDB.SetMaxIdleConns(2)
replicaDB.SetMaxOpenConns(60)
replicaDB.SetMaxIdleConns(2)

// Create EventStore with replica support
eventStore, err := postgresengine.NewEventStoreFromSQLDBAndReplica(
    primaryDB,
    replicaDB,
)
if err != nil {
    log.Fatal(err)
}
```

### sqlx

```go
import (
    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
    "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore/postgresengine"
)

// Create primary and replica connections
primaryDB, err := sqlx.Connect("postgres", "postgres://user:pass@primary-host:5432/eventstore")
if err != nil {
    log.Fatal(err)
}

replicaDB, err := sqlx.Connect("postgres", "postgres://user:pass@replica-host:5432/eventstore")
if err != nil {
    log.Fatal(err)
}

// Configure connection pools
primaryDB.SetMaxOpenConns(60)
primaryDB.SetMaxIdleConns(2)
replicaDB.SetMaxOpenConns(60)
replicaDB.SetMaxIdleConns(2)

// Create EventStore with replica support
eventStore, err := postgresengine.NewEventStoreFromSQLXAndReplica(
    primaryDB,
    replicaDB,
)
if err != nil {
    log.Fatal(err)
}
```

## Context-Based Routing

### Strong Consistency (Primary)

Use `WithStrongConsistency()` for command handlers and any operation requiring read-after-write consistency:

```go
import "github.com/AntonStoeckl/dynamic-streams-eventstore-go/eventstore"

// Command handlers - ALWAYS use strong consistency
func HandleAddBookCommand(ctx context.Context, cmd AddBookCommand) error {
    // Route to primary database
    ctx = eventstore.WithStrongConsistency(ctx)

    // Query existing events (from primary)
    events, maxSeq, err := eventStore.Query(ctx, filter)
    if err != nil {
        return err
    }

    // Apply business logic
    newEvent := applyBusinessLogic(events, cmd)

    // Append to primary database
    return eventStore.Append(ctx, filter, maxSeq, newEvent)
}
```

### Eventual Consistency (Replica)

Use `WithEventualConsistency()` for read-only query handlers where slight staleness is acceptable:

```go
// Query handlers - can use eventual consistency for better performance
func GetBooksInCirculation(ctx context.Context, query BooksQuery) (*BooksResult, error) {
    // Route to replica database (if available)
    ctx = eventstore.WithEventualConsistency(ctx)

    // Query events (from replica)
    events, _, err := eventStore.Query(ctx, filter)
    if err != nil {
        return nil, err
    }

    // Build read-only projection
    return buildProjection(events), nil
}
```

## Critical Usage Rules

> **⚠️ CRITICAL**: Always use `WithStrongConsistency()` for command handlers (read-check-write operations) and `WithEventualConsistency()` only for pure query handlers (read-only operations). Mixing these up will cause concurrency conflicts or stale data issues.

### Correct Patterns

✅ **Command Handler (Write Operations)**
```go
ctx = eventstore.WithStrongConsistency(ctx) // Always primary
events, maxSeq, err := eventStore.Query(ctx, filter)
err = eventStore.Append(ctx, filter, maxSeq, newEvent)
```

✅ **Query Handler (Read-Only)**
```go
ctx = eventstore.WithEventualConsistency(ctx) // Can use replica
events, _, err := eventStore.Query(ctx, filter)
// No append operations
```

### Incorrect Patterns

❌ **Never use eventual consistency for commands**
```go
ctx = eventstore.WithEventualConsistency(ctx) // WRONG! Could read stale data
events, maxSeq, err := eventStore.Query(ctx, filter)
err = eventStore.Append(ctx, filter, maxSeq, newEvent) // Race condition!
```

❌ **Don't mix consistency levels**
```go
ctx1 = eventstore.WithEventualConsistency(ctx)
events, _, err := eventStore.Query(ctx1, filter) // From replica

ctx2 = eventstore.WithStrongConsistency(ctx)
err = eventStore.Append(ctx2, filter, maxSeq, newEvent) // Wrong maxSeq!
```

## Default Behavior

- **Strong Consistency**: Default behavior when no context is set
- **Primary Database**: All operations use primary by default
- **Safe for Event Sourcing**: Default behavior prevents concurrency issues

```go
// These are equivalent:
events, maxSeq, err := eventStore.Query(ctx, filter)
events, maxSeq, err := eventStore.Query(eventstore.WithStrongConsistency(ctx), filter)
```

## PostgreSQL Streaming Replication Setup

### Basic Replication Configuration

On the **primary server**, add to `postgresql.conf`:
```conf
# Replication settings
wal_level = replica
max_wal_senders = 3
wal_keep_size = 64MB
```

Create a replication user:
```sql
CREATE USER replicator WITH REPLICATION;
```

Add to `pg_hba.conf`:
```conf
host replication replicator replica-host/32 md5
```

On the **replica server**, create `recovery.conf`:
```conf
standby_mode = 'on'
primary_conninfo = 'host=primary-host port=5432 user=replicator'
```

### Docker Compose Example

For local development and testing:

```yaml
version: '3.8'
services:
  postgres-primary:
    image: postgres:15
    environment:
      POSTGRES_DB: eventstore
      POSTGRES_USER: eventstore
      POSTGRES_PASSWORD: eventstore
    volumes:
      - ./docker/postgresql.primary.conf:/etc/postgresql/postgresql.conf
    command: postgres -c config_file=/etc/postgresql/postgresql.conf
    ports:
      - "5432:5432"

  postgres-replica:
    image: postgres:15
    environment:
      POSTGRES_DB: eventstore
      POSTGRES_USER: eventstore
      POSTGRES_PASSWORD: eventstore
      PGUSER: postgres
    volumes:
      - ./docker/setup-replica.sh:/docker-entrypoint-initdb.d/setup-replica.sh
    command: |
      bash -c "
        pg_basebackup -h postgres-primary -D /var/lib/postgresql/data -U replicator -W &&
        echo 'standby_mode = on' >> /var/lib/postgresql/data/recovery.conf &&
        echo 'primary_conninfo = host=postgres-primary port=5432 user=replicator' >> /var/lib/postgresql/data/recovery.conf &&
        postgres
      "
    ports:
      - "5433:5432"
    depends_on:
      - postgres-primary
```

## Performance Characteristics

### Load Distribution

- **Write Load**: All writes go to primary (no change)
- **Read Load**: Distributed between primary and replica
- **Query Performance**: Replica queries don't compete with writes

### Replication Lag

- **Typical Lag**: 1–50 ms for streaming replication
- **Network Impact**: Higher latency = higher lag
- **Monitoring**: Use `pg_stat_replication` for lag metrics

## Monitoring and Troubleshooting

### Check Replication Status

On primary:
```sql
SELECT client_addr, state, sent_lsn, write_lsn, flush_lsn, replay_lsn,
       EXTRACT(EPOCH FROM (now() - replay_lag)) AS replay_lag_seconds
FROM pg_stat_replication;
```

On replica:
```sql
SELECT now() - pg_last_xact_replay_timestamp() AS replication_lag;
```

### Common Issues

**High Replication Lag**:
- Check network connectivity between primary and replica
- Monitor primary write load
- Verify replica hardware capacity

**Connection Failures**:
- Test connectivity to both primary and replica
- Verify credentials and permissions
- Check PostgreSQL logs for connection errors

**Stale Read Issues**:
- Ensure `WithStrongConsistency()` is used for command handlers
- Consider increasing replica resources if the lag is consistently high

## Best Practices

### When to Use Primary-Replica

✅ **Good Candidates**:
- Read-heavy workloads with frequent queries
- Applications with clear command/query separation
- Production systems with predictable traffic patterns

❌ **Avoid When**:
- Write-heavy workloads (no benefit)
- Immediate-consistency requirements (replication lag sensitive)
- Simple applications with low traffic

### Testing Strategies

1. **Unit Tests**: Test with a single database (primary only)
2. **Integration Tests**: Test with both primary and replica
3. **Load Tests**: Verify performance benefits under a realistic load
4. **Failover Tests**: Test behavior when replica becomes unavailable

### Failover Considerations

- **Replica Failure**: Queries requesting eventual consistency will fail; applications must handle fallback
- **Primary Failure**: Requires manual intervention (promote replica)
- **Split Brain**: Use proper PostgreSQL failover tools (pg_auto_failover, Patroni)

## Error Handling

**Important**: The EventStore does **NOT** automatically fall back to primary when a replica fails.

```go
// If replica is unavailable, this will return an error
ctx = eventstore.WithEventualConsistency(ctx)
events, _, err := eventStore.Query(ctx, filter)
if err != nil {
    // Handle replica failure - could retry with strong consistency
    ctx = eventstore.WithStrongConsistency(ctx)
    events, _, err = eventStore.Query(ctx, filter)
}
```

Applications must handle replica failures explicitly if they want fallback behavior.

## Migration from Single Database

Existing single-database EventStores can be upgraded to Primary-Replica:

```go
// Before: Single database
eventStore, err := postgresengine.NewEventStoreFromPGXPool(pool)

// After: Primary + Replica (no breaking changes)
eventStore, err := postgresengine.NewEventStoreFromPGXPoolAndReplica(primaryPool, replicaPool)
```

All existing code continues to work with strong consistency by default.