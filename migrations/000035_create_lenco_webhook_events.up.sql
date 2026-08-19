CREATE TABLE lenco_webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_key TEXT NOT NULL UNIQUE,
    event_type TEXT NOT NULL,
    external_reference TEXT,
    payload JSONB NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    processing_error TEXT
);

CREATE INDEX idx_lenco_webhook_events_type_received
    ON lenco_webhook_events (event_type, received_at DESC);
