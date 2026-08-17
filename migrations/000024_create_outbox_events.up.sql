CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(16) NOT NULL DEFAULT 'PENDING',
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_at TIMESTAMPTZ,
    processed_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_outbox_event_status CHECK (status IN ('PENDING', 'PROCESSING', 'DELIVERED', 'FAILED', 'DEAD_LETTER'))
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_pending
    ON outbox_events (status, available_at, created_at)
    WHERE status IN ('PENDING', 'FAILED');

CREATE INDEX IF NOT EXISTS idx_outbox_events_aggregate
    ON outbox_events (aggregate_type, aggregate_id, created_at);
