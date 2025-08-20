CREATE TABLE if not exists events (
    sequence_number BIGSERIAL PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    metadata JSONB DEFAULT '{}'::JSONB
);

-- Optimized indexes for querying
CREATE INDEX if not exists idx_events_event_type ON events(event_type);
CREATE INDEX if not exists idx_events_occurred_at ON events(occurred_at);
CREATE INDEX if not exists idx_events_payload_gin ON events USING gin(payload jsonb_path_ops);
CREATE INDEX if not exists idx_events_metadata_gin ON events USING gin(metadata jsonb_path_ops);

-- Set aggressive statistics targets for all indexed columns
ALTER TABLE events ALTER COLUMN event_type SET STATISTICS 5000;      -- High cardinality
ALTER TABLE events ALTER COLUMN payload SET STATISTICS 5000;         -- JSONB selectivity
ALTER TABLE events ALTER COLUMN metadata SET STATISTICS 2000;        -- Less critical
ALTER TABLE events ALTER COLUMN sequence_number SET STATISTICS 5000; -- Critical for sorts
ALTER TABLE events ALTER COLUMN occurred_at SET STATISTICS 3000;     -- Time-based queries

-- Snapshots table for query projection caching
CREATE TABLE if not exists snapshots (
    projection_type TEXT NOT NULL,
    filter_hash TEXT NOT NULL,
    sequence_number BIGINT NOT NULL,
    snapshot_data JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (projection_type, filter_hash)
);

-- Indexes for snapshot cleanup and monitoring
CREATE INDEX if not exists idx_snapshots_sequence ON snapshots(sequence_number DESC);
CREATE INDEX if not exists idx_snapshots_created_at ON snapshots(created_at DESC);

-- Statistics for snapshot table - only for columns used in WHERE clauses
ALTER TABLE snapshots ALTER COLUMN projection_type SET STATISTICS 200;   -- Used in WHERE, moderate cardinality
ALTER TABLE snapshots ALTER COLUMN filter_hash SET STATISTICS 500;       -- Used in WHERE, high selectivity  
ALTER TABLE snapshots ALTER COLUMN created_at SET STATISTICS 200;        -- Used for cleanup queries (indexed)