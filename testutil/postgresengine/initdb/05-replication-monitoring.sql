-- Create an extension for better monitoring
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

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
        COALESCE(client_hostname, client_addr::text)::text as replica_name,
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