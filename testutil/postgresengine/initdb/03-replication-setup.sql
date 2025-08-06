-- PostgreSQL 17.5 Replication Setup for EventStore Benchmark Master
-- This script creates the replication user and sets up the event store schema with replication support

-- Create a replication user with enhanced PostgreSQL 17.5 privileges
CREATE USER replicator REPLICATION LOGIN ENCRYPTED PASSWORD 'replicator_password';

-- Grant additional privileges for PostgreSQL 17.5 replication features
GRANT CONNECT ON DATABASE eventstore TO replicator;
GRANT USAGE ON SCHEMA public TO replicator;

-- Grant permissions to application user
GRANT SELECT, INSERT, UPDATE, DELETE ON events TO test;
GRANT USAGE, SELECT ON SEQUENCE events_sequence_number_seq TO test;

-- Grant permissions to replicator for monitoring
GRANT SELECT ON events TO replicator;
GRANT USAGE ON SCHEMA public TO replicator;

-- Create a replication slot for the replica (PostgreSQL 17.5 enhanced slots)
-- This will be used by the replica for guaranteed WAL retention
SELECT pg_create_physical_replication_slot('replica_slot');

-- Create monitoring views for replication health
CREATE OR REPLACE VIEW v_replication_status AS
SELECT 
    client_addr,
    client_hostname,
    state,
    sent_lsn,
    write_lsn,
    flush_lsn,
    replay_lsn,
    write_lag,
    flush_lag,
    replay_lag,
    sync_state,
    sync_priority
FROM pg_stat_replication;

-- Create a view for monitoring WAL generation
CREATE OR REPLACE VIEW v_wal_status AS
SELECT 
    slot_name,
    plugin,
    slot_type,
    database,
    temporary,
    active,
    active_pid,
    xmin,
    catalog_xmin,
    restart_lsn,
    confirmed_flush_lsn,
    wal_status,
    safe_wal_size
FROM pg_replication_slots;

-- Create a view for events table health monitoring (from 02-events-table-tuning.sql)
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

-- Function to check replication lag and health
CREATE OR REPLACE FUNCTION check_replication_health() 
RETURNS TABLE(
    replica_name text,
    client_addr inet,
    state text,
    write_lag_ms numeric,
    flush_lag_ms numeric,
    replay_lag_ms numeric,
    status text,
    recommendation text
) AS $$
BEGIN
    RETURN QUERY 
    SELECT 
        COALESCE(sr.client_hostname, sr.client_addr::text)::text as replica_name,
        sr.client_addr,
        sr.state,
        EXTRACT(EPOCH FROM sr.write_lag) * 1000 as write_lag_ms,
        EXTRACT(EPOCH FROM sr.flush_lag) * 1000 as flush_lag_ms,
        EXTRACT(EPOCH FROM sr.replay_lag) * 1000 as replay_lag_ms,
        CASE 
            WHEN sr.state = 'streaming' AND EXTRACT(EPOCH FROM COALESCE(sr.replay_lag, '0s'::interval)) < 0.1 
            THEN 'HEALTHY'::text
            WHEN sr.state = 'streaming' AND EXTRACT(EPOCH FROM COALESCE(sr.replay_lag, '0s'::interval)) < 1.0
            THEN 'LAGGING'::text
            WHEN sr.state = 'streaming'
            THEN 'SLOW'::text
            ELSE 'DISCONNECTED'::text
        END,
        CASE 
            WHEN sr.state = 'streaming' AND EXTRACT(EPOCH FROM COALESCE(sr.replay_lag, '0s'::interval)) < 0.1 
            THEN 'Replica is healthy and up-to-date'::text
            WHEN sr.state = 'streaming' AND EXTRACT(EPOCH FROM COALESCE(sr.replay_lag, '0s'::interval)) < 1.0
            THEN 'Replica lag is acceptable for read queries'::text
            WHEN sr.state = 'streaming'
            THEN 'Replica lag is high - check network and load'::text
            ELSE 'Replica is not connected - check configuration'::text
        END
    FROM pg_stat_replication sr;
END;
$$ LANGUAGE plpgsql;

-- Add helpful comments for monitoring and troubleshooting
COMMENT ON VIEW v_replication_status IS 'Monitor replication status and lag for all replicas';
COMMENT ON VIEW v_wal_status IS 'Monitor WAL generation and replication slot status';
COMMENT ON FUNCTION check_replication_health() IS 'Check replication health and get recommendations. Usage: SELECT * FROM check_replication_health();';