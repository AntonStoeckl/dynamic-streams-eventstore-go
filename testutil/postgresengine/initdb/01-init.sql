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