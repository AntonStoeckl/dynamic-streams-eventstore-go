-- PostgreSQL Catalog-Stored Tuning Settings
-- This file contains all settings that are stored in the database catalog
-- and need to be reapplied on every container restart to ensure consistency
-- 
-- These settings override any manual ALTER commands and ensure the single source of truth

-- Reset any ALTER SYSTEM commands that might override our postgresql.conf
ALTER SYSTEM RESET ALL;
SELECT pg_reload_conf();

-- ==============================================================================
-- EVENTS TABLE TUNING
-- ==============================================================================

-- Column Statistics Targets - Optimized for performance
ALTER TABLE events ALTER COLUMN payload SET STATISTICS 200;          -- Reduced to minimize ANALYZE I/O impact
ALTER TABLE events ALTER COLUMN event_type SET STATISTICS 100;       -- Reduced for faster ANALYZE
ALTER TABLE events ALTER COLUMN metadata SET STATISTICS 100;         -- Less critical
ALTER TABLE events ALTER COLUMN sequence_number SET STATISTICS 100;  -- Using default
ALTER TABLE events ALTER COLUMN occurred_at SET STATISTICS 100;      -- Less critical

-- Events table: Append-only optimization (minimal vacuum, reduced analyze frequency)
ALTER TABLE events SET (
    autovacuum_analyze_threshold = 3000,           -- Reduced frequency: every 3000 inserts
    autovacuum_analyze_scale_factor = 0,           -- Pure threshold, no percentage
    autovacuum_vacuum_threshold = 50000,           -- Very high threshold (append-only)
    autovacuum_vacuum_scale_factor = 0.5,          -- Only vacuum at 50% dead tuples
    autovacuum_vacuum_cost_delay = 0,              -- No throttling (applies to both vacuum and analyze)
    autovacuum_vacuum_cost_limit = 10000,          -- High I/O budget (applies to both)
    fillfactor = 98                                -- Minimal space for updates (append-only)
);

-- ==============================================================================
-- SNAPSHOTS TABLE TUNING
-- ==============================================================================

-- Statistics for snapshot table - optimized for mixed workload
ALTER TABLE snapshots ALTER COLUMN projection_type SET STATISTICS 200;   -- Used in WHERE, moderate cardinality
ALTER TABLE snapshots ALTER COLUMN filter_hash SET STATISTICS 300;       -- Used in WHERE, high selectivity
ALTER TABLE snapshots ALTER COLUMN sequence_number SET STATISTICS 30;    -- Less critical
ALTER TABLE snapshots ALTER COLUMN snapshot_data SET STATISTICS 10;      -- Large JSONB, minimal stats needed
ALTER TABLE snapshots ALTER COLUMN created_at SET STATISTICS 100;        -- Used for cleanup queries (indexed)

-- Snapshots table: Mixed workload optimization (regular vacuum and analyze)
ALTER TABLE snapshots SET (
    autovacuum_vacuum_threshold = 100,             -- Low threshold for cleanup
    autovacuum_vacuum_scale_factor = 0.1,          -- 10% dead tuples trigger vacuum
    autovacuum_analyze_threshold = 200,            -- Moderate analyze frequency
    autovacuum_analyze_scale_factor = 0.1,         -- 10% changes trigger analyze
    autovacuum_vacuum_cost_delay = 0,              -- No throttling for a small table
    autovacuum_vacuum_cost_limit = 10000,          -- High I/O budget
    fillfactor = 90                                -- 10% space for HOT updates
);

-- ==============================================================================
-- FORCE STATISTICS UPDATE
-- ==============================================================================

-- Force immediate ANALYZE to apply new statistics targets
-- This ensures the query planner immediately uses the new statistics
ANALYZE events;
ANALYZE snapshots;

-- Log successful application
DO $$
BEGIN
    RAISE NOTICE 'Catalog tuning settings applied successfully - statistics and autovacuum settings updated';
END
$$;