-- payment_transactions: provider-agnostic record of every payment attempt.
-- Supports MTN Mobile Money, Airtel Money, and future providers.
CREATE TABLE IF NOT EXISTS payment_transactions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Reference to what is being paid (polymorphic)
    reference_type      VARCHAR(32) NOT NULL,
    -- ORDER | INVOICE | SUBSCRIPTION
    reference_id        UUID NOT NULL,
    vendor_id           UUID REFERENCES vendors(id) ON DELETE SET NULL,
    -- Provider details
    provider            VARCHAR(32) NOT NULL,
    -- MTN_MOMO | AIRTEL_MONEY | CASH | CARD
    provider_ref        VARCHAR(128),
    -- External transaction ID from the provider
    provider_status     VARCHAR(64),
    -- Raw status string from provider
    -- Internal lifecycle
    status              VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    -- PENDING | PROCESSING | COMPLETED | FAILED | CANCELLED | REFUNDED
    amount              NUMERIC(12,2) NOT NULL,
    currency            VARCHAR(3) NOT NULL DEFAULT 'ZMW',
    phone_number        VARCHAR(20),
    -- MSISDN for mobile money
    description         TEXT,
    -- Webhook & callback tracking
    webhook_received_at TIMESTAMPTZ,
    webhook_payload     JSONB,
    -- Full raw payload from provider webhook
    -- Retry & idempotency
    idempotency_key     VARCHAR(128) UNIQUE,
    retry_count         INTEGER NOT NULL DEFAULT 0,
    last_error          TEXT,
    -- Metadata
    metadata            JSONB,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payment_transactions_reference  ON payment_transactions(reference_type, reference_id);
CREATE INDEX idx_payment_transactions_provider   ON payment_transactions(provider, provider_ref);
CREATE INDEX idx_payment_transactions_status     ON payment_transactions(status);
CREATE INDEX idx_payment_transactions_vendor     ON payment_transactions(vendor_id);
