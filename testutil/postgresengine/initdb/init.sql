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

-- I experimented with compound indexes, but it does not seem to make queries faster ...
-- CREATE EXTENSION if not exists btree_gin;
-- CREATE INDEX if not exists idx_events_event_type_payload_gin ON events USING gin(event_type, payload jsonb_path_ops);
-- CREATE INDEX if not exists idx_events_event_type_payload_occurred_at_gin ON events USING gin(event_type, payload jsonb_path_ops, occurred_at);