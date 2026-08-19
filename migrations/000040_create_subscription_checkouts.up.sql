-- subscription_checkouts locks a tier, amount, and vendor before a hosted payment widget is opened.
-- A successful provider collection is reconciled to this record before any subscription is activated.
CREATE TABLE IF NOT EXISTS subscription_checkouts (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id              UUID NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
    tier_id                UUID NOT NULL REFERENCES vendor_tiers(id) ON DELETE RESTRICT,
    amount                 NUMERIC(12,2) NOT NULL CHECK (amount > 0),
    currency               VARCHAR(3) NOT NULL DEFAULT 'ZMW',
    reference              VARCHAR(128) NOT NULL UNIQUE,
    status                 VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    -- PENDING | SUCCESSFUL | FAILED | EXPIRED
    provider_collection_id VARCHAR(128) UNIQUE,
    provider_status        VARCHAR(64),
    subscription_id        UUID REFERENCES vendor_subscriptions(id) ON DELETE SET NULL,
    invoice_id             UUID REFERENCES billing_invoices(id) ON DELETE SET NULL,
    expires_at             TIMESTAMPTZ NOT NULL,
    completed_at           TIMESTAMPTZ,
    failure_reason         TEXT,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscription_checkouts_vendor_status
    ON subscription_checkouts (vendor_id, status, created_at DESC);
CREATE INDEX idx_subscription_checkouts_reference
    ON subscription_checkouts (reference);
