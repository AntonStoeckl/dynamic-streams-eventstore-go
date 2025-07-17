CREATE TABLE if not exists events (
    sequence_number BIGSERIAL PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL
);

-- Optimized indexes for querying
CREATE INDEX if not exists idx_events_event_type ON events(event_type);
CREATE INDEX if not exists idx_events_occurred_at ON events(occurred_at);
CREATE INDEX if not exists idx_events_payload_gin ON events USING gin(payload);
-- CREATE EXTENSION if not exists btree_gin;
-- CREATE INDEX if not exists idx_events_event_type_payload_gin ON events USING gin(event_type, payload);
-- CREATE INDEX if not exists idx_events_event_type_payload_occurred_at_gin ON events USING gin(event_type, payload, occurred_at);