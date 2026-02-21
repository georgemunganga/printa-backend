-- Order items: individual line items within an order
CREATE TABLE IF NOT EXISTS order_items (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id               UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    vendor_store_product_id UUID NOT NULL REFERENCES vendor_store_products(id) ON DELETE RESTRICT,
    quantity               INT NOT NULL DEFAULT 1,
    unit_price             NUMERIC(12,2) NOT NULL,
    line_total             NUMERIC(12,2) NOT NULL,
    customisation          JSONB,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_items_order_id ON order_items(order_id);
