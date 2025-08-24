CREATE TABLE if not exists events (
    sequence_number BIGSERIAL PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    metadata JSONB DEFAULT '{}'::JSONB
);

CREATE INDEX if not exists idx_events_event_type ON events(event_type);
CREATE INDEX if not exists idx_events_occurred_at ON events(occurred_at);
CREATE INDEX if not exists idx_events_payload_gin ON events USING gin(payload jsonb_path_ops);

CREATE TABLE if not exists snapshots (
    projection_type TEXT NOT NULL,
    filter_hash TEXT NOT NULL,
    sequence_number BIGINT NOT NULL,
    snapshot_data JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (projection_type, filter_hash)
);

CREATE INDEX if not exists idx_snapshots_sequence ON snapshots(sequence_number DESC);
CREATE INDEX if not exists idx_snapshots_created_at ON snapshots(created_at DESC);

-- Optimized statistics targets for performance (2025-08-23)
CREATE INDEX if not exists idx_events_metadata_gin ON events USING gin(metadata jsonb_path_ops);
ALTER TABLE events ALTER COLUMN payload SET STATISTICS 500;         -- Balanced for GIN index performance -> 850
ALTER TABLE events ALTER COLUMN event_type SET STATISTICS 100;       -- Reduced for faster ANALYZE
ALTER TABLE events ALTER COLUMN metadata SET STATISTICS 100;         -- Less critical
ALTER TABLE events ALTER COLUMN sequence_number SET STATISTICS 100;  -- Using default
ALTER TABLE events ALTER COLUMN occurred_at SET STATISTICS 100;      -- Less critical

-- Events table: Append-only optimization (minimal vacuum, aggressive analyze)
ALTER TABLE events SET (
    autovacuum_analyze_threshold = 1500,           -- Trigger analyze every 1600 inserts -> 1600
    autovacuum_analyze_scale_factor = 0,           -- Pure threshold, no percentage
    autovacuum_vacuum_threshold = 50000,           -- Very high threshold (append-only)
    autovacuum_vacuum_scale_factor = 0.5,          -- Only vacuum at 50% dead tuples
    autovacuum_vacuum_cost_delay = 0,              -- No throttling (applies to both vacuum and analyze)
    autovacuum_vacuum_cost_limit = 10000,          -- High I/O budget (applies to both)
    fillfactor = 100                               -- No space for updates (append-only)
);

-- Statistics for snapshot table - optimized for mixed workload (2025-08-23)
ALTER TABLE snapshots ALTER COLUMN projection_type SET STATISTICS 200;   -- Used in WHERE, moderate cardinality
ALTER TABLE snapshots ALTER COLUMN filter_hash SET STATISTICS 500;       -- Used in WHERE, high selectivity
ALTER TABLE snapshots ALTER COLUMN sequence_number SET STATISTICS 50;    -- Less critical
ALTER TABLE snapshots ALTER COLUMN snapshot_data SET STATISTICS 10;      -- Large JSONB, minimal stats needed
ALTER TABLE snapshots ALTER COLUMN created_at SET STATISTICS 200;        -- Used for cleanup queries (indexed)

-- Snapshots table: Mixed workload optimization (regular vacuum and analyze)
ALTER TABLE snapshots SET (
    autovacuum_vacuum_threshold = 100,             -- Low threshold for cleanup
    autovacuum_vacuum_scale_factor = 0.1,          -- 10% dead tuples trigger vacuum
    autovacuum_analyze_threshold = 200,            -- Moderate analyze frequency
    autovacuum_analyze_scale_factor = 0.1,         -- 10% changes trigger analyze
    autovacuum_vacuum_cost_delay = 0,              -- No throttling for a small table
    autovacuum_vacuum_cost_limit = -1,             -- Use system default
    fillfactor = 90                                -- 10% space for HOT updates
);
