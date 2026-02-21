-- Vendor store products are platform products listed by a vendor in a specific store,
-- with vendor-specific pricing and stock management.
CREATE TABLE IF NOT EXISTS vendor_store_products (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id            UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    platform_product_id UUID NOT NULL REFERENCES platform_products(id),
    vendor_price        DECIMAL(12, 2) NOT NULL,
    currency            VARCHAR(3) NOT NULL DEFAULT 'ZMW',
    stock_quantity      INT NOT NULL DEFAULT 0,
    is_available        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(store_id, platform_product_id)
);
CREATE INDEX idx_vsp_store_id            ON vendor_store_products(store_id);
CREATE INDEX idx_vsp_platform_product_id ON vendor_store_products(platform_product_id);
