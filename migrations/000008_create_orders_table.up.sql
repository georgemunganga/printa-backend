-- Orders table: represents a customer's print order placed at a store
CREATE TABLE IF NOT EXISTS orders (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id         UUID NOT NULL REFERENCES stores(id) ON DELETE RESTRICT,
    customer_id      UUID REFERENCES users(id) ON DELETE SET NULL,
    order_number     VARCHAR(32) NOT NULL UNIQUE,
    status           VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    channel          VARCHAR(16) NOT NULL DEFAULT 'ONLINE',
    subtotal         NUMERIC(12,2) NOT NULL DEFAULT 0,
    discount         NUMERIC(12,2) NOT NULL DEFAULT 0,
    tax              NUMERIC(12,2) NOT NULL DEFAULT 0,
    total            NUMERIC(12,2) NOT NULL DEFAULT 0,
    currency         VARCHAR(8)  NOT NULL DEFAULT 'ZMW',
    notes            TEXT,
    delivery_address JSONB,
    metadata         JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_status      ON orders(status);
CREATE INDEX idx_orders_store_id    ON orders(store_id);
CREATE INDEX idx_orders_customer_id ON orders(customer_id);
CREATE INDEX idx_orders_created_at  ON orders(created_at DESC);
