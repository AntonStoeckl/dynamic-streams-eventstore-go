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