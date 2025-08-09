# PostgreSQL Performance Tuning for EventStore

This directory contains PostgreSQL performance optimizations specifically designed for high-throughput EventStore operations with JSONB GIN indexes.

## The Problem

With high insert rates (250+ ops/sec), PostgreSQL faces several challenges:

1. **Stale Statistics**: Table statistics become outdated quickly, causing the query planner to make poor decisions
2. **GIN Index Avoidance**: Query planner falls back to sequential scans or primary key scans instead of using GIN indexes
3. **Autovacuum Lag**: Default autovacuum settings are too conservative for high-insert workloads
4. **Performance Degradation**: Performance drops significantly as the planner avoids optimal indexes

## The Solution

### 1. Aggressive Autovacuum Configuration
**File**: `postgresql-performance.conf`

Key settings for EventStore workloads:
```sql
autovacuum_naptime = 15s                 # Check every 15 seconds (vs 60s default)
autovacuum_analyze_threshold = 500       # Analyze after 500 inserts (vs 50 default)  
autovacuum_analyze_scale_factor = 0.05   # Analyze when 5% changed (vs 10% default)
default_statistics_target = 1000         # Better statistics (vs 100 default)
```

### 2. Per-Table Tuning for Events Table
**File**: `initdb/02-events-table-tuning.sql`

Events table gets even more aggressive settings:
```sql
ALTER TABLE events SET (
  autovacuum_analyze_threshold = 100,       -- Analyze after 100 inserts
  autovacuum_analyze_scale_factor = 0.02    -- Analyze when 2% changed
);
```

### 3. Monitoring and Health Checks
Built-in function to check table health:
```sql
SELECT * FROM check_events_table_health();
```

Shows:
- Dead tuple percentage (should be < 5%)
- Time since last analyze (should be < 15 minutes)
- Recommendations for manual maintenance

### 4. Statistics Monitoring View
```sql
SELECT * FROM v_events_autovacuum_stats;
```

Shows autovacuum activity, dead tuples, and last maintenance times.

## Usage

### Start Optimized PostgreSQL (First Time or After Config Changes)
```bash
cd testutil/postgresengine

# Option 1: Use the restart script (recommended)
./restart-postgres-benchmark.sh

# Option 2: Manual restart (clears existing data volume)
docker compose stop postgres_benchmark
docker compose rm -f postgres_benchmark  
docker volume rm des_pgdata
docker compose up -d postgres_benchmark
```

### Start PostgreSQL (Normal Usage)
```bash
cd testutil/postgresengine
docker compose up -d postgres_benchmark
```

### Monitor During Load Testing
```sql
-- Check table health during load testing
SELECT * FROM check_events_table_health();

-- Monitor autovacuum activity
SELECT * FROM v_events_autovacuum_stats;

-- Check query plans are using GIN indexes
EXPLAIN (ANALYZE, BUFFERS) 
SELECT * FROM events 
WHERE payload @> '{"BookID": "some-book-id"}';
```

### Manual Maintenance (if needed)
```sql
-- If table health shows warnings:
VACUUM ANALYZE events;

-- Or just update statistics:
ANALYZE events;
```

## Expected Results

With these optimizations:

✅ **Consistent Performance**: Query planner consistently uses GIN indexes  
✅ **Auto-Maintenance**: Statistics updated every ~2-5 minutes automatically  
✅ **No Manual Intervention**: Eliminates need for manual `VACUUM ANALYZE`  
✅ **Sustained Throughput**: Maintains 250+ ops/sec without degradation  
✅ **Query Plan Stability**: GIN indexes used consistently, not avoided  

## Configuration Details

### Memory Settings
- **shared_buffers**: 256MB (adjust based on available RAM)
- **effective_cache_size**: 1GB (set to ~75% of available RAM)
- **work_mem**: 4MB (per-operation memory for sorts/joins)

### WAL Settings  
- **max_wal_size**: 2GB (larger WAL for high insert rates)
- **wal_compression**: on (reduces I/O)
- **checkpoint_completion_target**: 0.9 (spread checkpoints)

### Cost Settings
- **random_page_cost**: 1.1 (optimized for SSD)
- **default_statistics_target**: 1000 (detailed statistics for better planning)

## Troubleshooting

### Query Planner Not Using GIN Indexes?
```sql
-- Check if statistics are stale
SELECT * FROM check_events_table_health();

-- Check actual query plan
EXPLAIN (ANALYZE, BUFFERS) SELECT ...;

-- Force statistics update if needed
ANALYZE events;
```

### High Dead Tuple Percentage?
```sql
-- Check autovacuum settings are active
SHOW autovacuum;

-- Check recent autovacuum activity  
SELECT * FROM v_events_autovacuum_stats;

-- Force vacuum if needed
VACUUM events;
```

### Performance Still Degrading?
1. Check if PostgreSQL container has enough memory allocated
2. Monitor `pg_stat_statements` for slow queries
3. Consider adjusting `autovacuum_analyze_scale_factor` even lower (0.01)
4. Check disk I/O isn't the bottleneck

This configuration eliminates the need for manual `VACUUM ANALYZE` operations and maintains consistent EventStore performance under high load.

## 2025-08-08 Deep Performance Investigation Session

### Context
Master-replica PostgreSQL 17.5 setup with read/write splitting:
- **Master** (port 5433): 4GB RAM, handles only Append() operations (INSERTs)  
- **Replica** (port 5434): 6GB RAM, handles Query() operations (SELECTs)
- **Load Generator**: Worker pool architecture, 50 workers, targeting 150+ req/s

### Problem Statement
Load generator achieving 250 req/s stable previously, but master became bottleneck for Append() operations. Context timeouts occurring at 150 req/s load despite optimizations.

### Investigation Process

#### 1. Root Cause Analysis - Query Performance Under Load

**Without Load (Baseline)**:
```
Execution Time: 5.035ms
Planning Time: 23.700ms
Total: ~29ms
```

**With Load (150 req/s)**:
```
Execution Time: 42.640ms  (8.5x slower)
Planning Time: 135.841ms  (5.7x slower)
Total: ~178ms             (6.2x slower)
```

**Key Finding**: Individual index scan times varied wildly under load:
- BookCopyAddedToCirculation: **12.622ms** (should be <1ms)
- BookCopyRemovedFromCirculation: **14.549ms** 
- Other event types: 0.710ms - 1.707ms

This indicated **lock contention and I/O wait** on event_type index scans under concurrent load.

#### 2. Database Health Analysis

**Index Usage Statistics**:
- `idx_events_event_type`: 288,084 scans reading **1.7 BILLION tuples** (terrible selectivity)
- `idx_events_payload_gin`: 144,699 scans reading **10.3 million tuples** (much better)

**System Health**: All green
- Connections: 26/200 used (13% utilization)
- Cache hit ratios: 99.15% heap, 99.75% index
- No blocking locks, no temp files, no WAL pressure

#### 3. Configuration Optimization Attempt

**Changes Made** (`benchmarkdb-master-postgresql.conf`):
- **Memory**: shared_buffers 1024MB → 1536MB, work_mem 64MB → 128MB
- **WAL**: wal_buffers 32MB → 64MB, added wal_writer optimizations
- **Planner**: random_page_cost 1.1 → 1.0, cpu_index_tuple_cost reduced

**Results**: 
- Execution time improved: 42.640ms → **13.902ms** (67% improvement)
- Planning time improved: 135.841ms → **111.980ms** (18% improvement)
- Index scan consistency improved (no more 12-14ms outliers)

#### 4. Persistent Context Timeouts

Despite query performance improvements, context timeouts persisted at 150 req/s. 

**Analysis of Updated Performance**:
- Current execution: ~14ms + ~112ms planning = **126ms total**
- At 126ms per operation: ~8 ops/second per worker theoretical max
- 50 workers × 8 ops/sec = ~400 ops/sec theoretical max
- But **111ms planning time** still the major bottleneck

**Real Issue Identified**: Planning time explosion under concurrent load (111ms vs 24ms baseline) due to:
- Catalog lock contention during planning
- Statistics lookup pressure with many concurrent queries
- Query plan cache misses

#### 5. Configuration Rollback Decision

**User Decision**: Roll back configuration changes
**Reasoning**: Configuration changes helped individual query performance but didn't solve the fundamental planning time bottleneck causing context timeouts.

**Current Status**: Back to baseline configuration with understanding that the real bottleneck is **planning overhead under concurrent load**, not individual query execution time.

### Key Learnings

1. **Query Structure is Optimal**: JSONB payload index scanned first (most selective), event_type second
2. **Database Health is Good**: No connection, lock, memory, or I/O bottlenecks
3. **Real Bottleneck**: Planning time explosion under concurrent load (24ms → 111ms)
4. **Index Performance**: Current indexes are well-optimized for the query patterns
5. **Statistics are Effective**: Setting payload column statistics to 10,000 helps query planner

### Failed Approaches (Documented for Future Reference)

❌ **Composite Indexes**: Considered BookID/ReaderID specific indexes, but these are example-specific, not generic for EventStore library
❌ **jsonb_ops vs jsonb_path_ops**: jsonb_path_ops is correct for @> containment queries  
❌ **Connection Pool Issues**: 26/200 connections used, not a bottleneck
❌ **Memory Configuration**: Helped individual queries but didn't solve concurrency planning overhead

### Remaining Bottleneck

**Planning Time Under Concurrent Load**: 111ms planning time makes total operation time ~126ms, limiting sustainable throughput despite good individual query performance (14ms execution).

**Potential Solutions Not Yet Attempted**:
- Prepared statements to avoid re-planning
- Connection pooling optimizations  
- Query pattern restructuring
- PostgreSQL 17.5 specific concurrent planning optimizations

### Composite Index Experiment (August 2025)

#### Attempt: Composite GIN Index with btree_gin
**Hypothesis**: Create `(payload, event_type)` composite GIN index to eliminate BitmapAnd operations

**Implementation**:
```sql
CREATE EXTENSION IF NOT EXISTS btree_gin;
CREATE INDEX idx_events_payload_eventtype_gin 
ON events USING gin(payload, event_type) 
WITH (fastupdate = off);
```

**Results**: **FAILED - Major Performance Regression**
- **Planning Time**: 24ms → **330ms** (13.8x slower)
- **Execution Time**: ~14ms → **97ms** (7x slower)  
- **Total Time**: ~40ms → **427ms** (10.7x worse overall)

**Root Cause**: 
1. **Query Structure Mismatch**: Cross-product OR conditions `(payload @> A OR payload @> B) AND (event_type = C OR D OR E)` don't benefit from composite indexes
2. **Individual Scan Overhead**: Composite index scans were 10-28ms vs <1ms with individual indexes
3. **Planning Complexity**: Optimizer struggled with composite index statistics/costs

**Conclusion**: **Individual indexes with BitmapAnd are optimal** for this query pattern. The original index design is already the best approach.

❌ **Composite indexes don't help cross-product OR queries**
✅ **Original BitmapAnd approach is actually optimal** 
✅ **Confirms real bottleneck is planning time, not index structure**

This investigation confirms that the EventStore query patterns and indexes are well-optimized, but PostgreSQL query planning becomes the bottleneck under high concurrent load.