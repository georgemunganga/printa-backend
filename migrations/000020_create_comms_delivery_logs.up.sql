CREATE TABLE IF NOT EXISTS comms_delivery_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel         VARCHAR(20) NOT NULL,
    recipient       VARCHAR(255) NOT NULL,
    recipient_id    UUID,
    subject         TEXT,
    body            TEXT NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    provider_ref    VARCHAR(255),
    error_message   TEXT,
    retry_count     INT NOT NULL DEFAULT 0,
    idempotency_key VARCHAR(255) UNIQUE,
    sent_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_comms_logs_recipient_id ON comms_delivery_logs(recipient_id);
CREATE INDEX IF NOT EXISTS idx_comms_logs_channel ON comms_delivery_logs(channel);
CREATE INDEX IF NOT EXISTS idx_comms_logs_status ON comms_delivery_logs(status);
CREATE INDEX IF NOT EXISTS idx_comms_logs_created_at ON comms_delivery_logs(created_at DESC);
