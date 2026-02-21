-- pos_transactions: records point-of-sale payment transactions at the counter.
CREATE TABLE IF NOT EXISTS pos_transactions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    store_id        UUID NOT NULL REFERENCES stores(id) ON DELETE RESTRICT,
    cashier_id      UUID REFERENCES users(id) ON DELETE SET NULL,
    amount          NUMERIC(12,2) NOT NULL,
    currency        VARCHAR(3) NOT NULL DEFAULT 'ZMW',
    payment_method  VARCHAR(32) NOT NULL,
    -- CASH | CARD | MOBILE_MONEY | VOUCHER
    reference       VARCHAR(128),    -- external reference (e.g. card terminal receipt no.)
    status          VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    -- PENDING | COMPLETED | REFUNDED | FAILED
    change_given    NUMERIC(12,2) NOT NULL DEFAULT 0,  -- cash change returned to customer
    notes           TEXT,
    transacted_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pos_transactions_order_id  ON pos_transactions(order_id);
CREATE INDEX idx_pos_transactions_store_id  ON pos_transactions(store_id);
CREATE INDEX idx_pos_transactions_status    ON pos_transactions(status);
