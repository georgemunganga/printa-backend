-- billing_invoices: records each billing cycle invoice for a vendor subscription.
CREATE TABLE IF NOT EXISTS billing_invoices (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id     UUID NOT NULL REFERENCES vendor_subscriptions(id) ON DELETE CASCADE,
    vendor_id           UUID NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
    invoice_number      VARCHAR(64) NOT NULL UNIQUE,
    -- format: INV-YYYYMM-XXXX
    amount              NUMERIC(12,2) NOT NULL,
    currency            VARCHAR(3) NOT NULL DEFAULT 'ZMW',
    status              VARCHAR(32) NOT NULL DEFAULT 'DRAFT',
    -- DRAFT | OPEN | PAID | VOID | UNCOLLECTIBLE
    period_start        TIMESTAMPTZ NOT NULL,
    period_end          TIMESTAMPTZ NOT NULL,
    due_date            TIMESTAMPTZ NOT NULL,
    paid_at             TIMESTAMPTZ,
    payment_reference   VARCHAR(128),  -- external payment gateway reference
    line_items          JSONB NOT NULL DEFAULT '[]',
    notes               TEXT,
    idempotency_key     VARCHAR(128) UNIQUE,  -- prevents duplicate invoice generation
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_billing_invoices_vendor_id       ON billing_invoices(vendor_id);
CREATE INDEX idx_billing_invoices_subscription_id ON billing_invoices(subscription_id);
CREATE INDEX idx_billing_invoices_status          ON billing_invoices(status);
CREATE INDEX idx_billing_invoices_due_date        ON billing_invoices(due_date);
