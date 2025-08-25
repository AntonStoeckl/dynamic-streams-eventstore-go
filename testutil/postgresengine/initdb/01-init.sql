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

-- Additional indexes for performance
CREATE INDEX if not exists idx_events_metadata_gin ON events USING gin(metadata jsonb_path_ops);

-- Note: All tuning settings (column statistics, autovacuum parameters) have been moved to
-- tuning/apply-catalog-tuning.sql to ensure they are applied on every container startup
-- and not just on initial database creation.
