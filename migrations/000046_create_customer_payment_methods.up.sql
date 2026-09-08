CREATE TABLE IF NOT EXISTS customer_payment_methods (
    id UUID PRIMARY KEY,
    customer_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    phone_number TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT customer_payment_methods_provider_check
        CHECK (provider IN ('MTN_MOMO', 'AIRTEL_MONEY', 'ZAMTEL_MONEY')),
    CONSTRAINT customer_payment_methods_customer_provider_phone_unique
        UNIQUE (customer_id, provider, phone_number)
);

CREATE INDEX IF NOT EXISTS idx_customer_payment_methods_customer_created
    ON customer_payment_methods(customer_id, created_at ASC);

CREATE UNIQUE INDEX IF NOT EXISTS ux_customer_payment_methods_default
    ON customer_payment_methods(customer_id)
    WHERE is_default = TRUE;
