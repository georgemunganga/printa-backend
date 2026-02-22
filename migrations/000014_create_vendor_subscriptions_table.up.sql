-- vendor_subscriptions: tracks each vendor's subscription to a tier with full lifecycle.
CREATE TABLE IF NOT EXISTS vendor_subscriptions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id       UUID NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
    tier_id         UUID NOT NULL REFERENCES vendor_tiers(id) ON DELETE RESTRICT,
    status          VARCHAR(32) NOT NULL DEFAULT 'TRIAL',
    -- TRIAL | ACTIVE | PAST_DUE | SUSPENDED | CANCELLED
    billing_cycle   VARCHAR(16) NOT NULL DEFAULT 'MONTHLY',
    -- MONTHLY | ANNUAL
    current_period_start  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    current_period_end    TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '30 days'),
    trial_ends_at         TIMESTAMPTZ,
    cancelled_at          TIMESTAMPTZ,
    cancel_reason         TEXT,
    auto_renew            BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(vendor_id)  -- one active subscription per vendor
);

CREATE INDEX idx_vendor_subscriptions_vendor_id ON vendor_subscriptions(vendor_id);
CREATE INDEX idx_vendor_subscriptions_status    ON vendor_subscriptions(status);
CREATE INDEX idx_vendor_subscriptions_period_end ON vendor_subscriptions(current_period_end);
