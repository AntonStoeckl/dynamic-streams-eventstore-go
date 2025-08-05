-- EventStore Events Table Performance Tuning for High-Throughput Load Testing
-- This script optimizes the events table for aggressive autovacuum and better query planning

-- Set per-table autovacuum parameters for the events table
-- These override the global settings for this specific high-insert table
ALTER TABLE events SET (
  -- Vacuum more aggressively for high-insert tables
  autovacuum_vacuum_threshold = 500,        -- Vacuum after 500 dead tuples
  autovacuum_vacuum_scale_factor = 0.05,    -- Vacuum when 5% of the table is dead
  
  -- Analyze very frequently to keep statistics fresh for query planner
  autovacuum_analyze_threshold = 100,       -- Analyze after 100 inserts/updates
  autovacuum_analyze_scale_factor = 0.02,   -- Analyze when 2% of table changed
  
  -- Faster vacuum to reduce blocking
  autovacuum_vacuum_cost_delay = 5,         -- 5 ms delay (faster than global 10 ms)
  autovacuum_vacuum_cost_limit = 3000       -- Higher limit for faster completion
);

-- Set aggressive statistics targets for all indexed columns
-- Critical for accurate cost estimation at 2M+ event scale
ALTER TABLE events ALTER COLUMN event_type SET STATISTICS 5000;    -- High cardinality
ALTER TABLE events ALTER COLUMN payload SET STATISTICS 5000;       -- JSONB selectivity
ALTER TABLE events ALTER COLUMN metadata SET STATISTICS 2000;      -- Less critical
ALTER TABLE events ALTER COLUMN sequence_number SET STATISTICS 5000; -- Critical for sorts
ALTER TABLE events ALTER COLUMN occurred_at SET STATISTICS 3000;   -- Time-based queries

-- Force immediate statistics collection with higher sampling
ANALYZE events;

-- Ensure GIN indexes have optimal settings
-- (These would need to be dropped and recreated with new settings)
-- Note: This will be applied when indexes are created in the main schema

-- Create an extension for better monitoring
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Set up a view for monitoring autovacuum activity
CREATE OR REPLACE VIEW v_events_autovacuum_stats AS
SELECT 
    schemaname,
    relname,
    n_tup_ins as inserts,
    n_tup_upd as updates,
    n_tup_del as deletes,
    n_live_tup as live_tuples,
    n_dead_tup as dead_tuples,
    ROUND(100.0 * n_dead_tup / GREATEST(n_live_tup, 1), 2) as dead_tuple_percent,
    last_vacuum,
    last_autovacuum,
    last_analyze,
    last_autoanalyze,
    vacuum_count,
    autovacuum_count,
    analyze_count,
    autoanalyze_count
FROM pg_stat_user_tables 
WHERE relname = 'events';

-- Function to check if the events table needs immediate maintenance
CREATE OR REPLACE FUNCTION check_events_table_health() 
RETURNS TABLE(
    metric text,
    value numeric,
    threshold numeric,
    status text,
    recommendation text
) AS $$
BEGIN
    -- Check dead tuple percentage
    RETURN QUERY 
    SELECT 
        'dead_tuple_percent'::text,
        ROUND(100.0 * n_dead_tup / GREATEST(n_live_tup, 1), 2),
        5.0::numeric,
        CASE 
            WHEN ROUND(100.0 * n_dead_tup / GREATEST(n_live_tup, 1), 2) > 5.0 
            THEN 'WARNING'::text
            ELSE 'OK'::text
        END,
        CASE 
            WHEN ROUND(100.0 * n_dead_tup / GREATEST(n_live_tup, 1), 2) > 5.0 
            THEN 'Consider manual VACUUM ANALYZE events;'::text
            ELSE 'Table health is good'::text
        END
    FROM pg_stat_user_tables 
    WHERE relname = 'events';
    
    -- Check time since last analyze
    RETURN QUERY 
    SELECT 
        'minutes_since_analyze'::text,
        COALESCE(EXTRACT(EPOCH FROM (NOW() - GREATEST(last_analyze, last_autoanalyze)))/60, 999999),
        15.0::numeric,
        CASE 
            WHEN COALESCE(EXTRACT(EPOCH FROM (NOW() - GREATEST(last_analyze, last_autoanalyze)))/60, 999999) > 15.0 
            THEN 'WARNING'::text
            ELSE 'OK'::text
        END,
        CASE 
            WHEN COALESCE(EXTRACT(EPOCH FROM (NOW() - GREATEST(last_analyze, last_autoanalyze)))/60, 999999) > 15.0 
            THEN 'Statistics may be stale, consider ANALYZE events;'::text
            ELSE 'Statistics are fresh'::text
        END
    FROM pg_stat_user_tables 
    WHERE relname = 'events';
END;
$$ LANGUAGE plpgsql;

-- Helpful comment for troubleshooting
COMMENT ON FUNCTION check_events_table_health() IS 
'Check events table health and get recommendations. Usage: SELECT * FROM check_events_table_health();';