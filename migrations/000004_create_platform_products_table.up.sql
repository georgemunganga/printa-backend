-- Platform products are the master catalog managed by the Printa platform admin.
-- Vendors list these products in their stores with custom pricing.
CREATE TABLE IF NOT EXISTS platform_products (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    category    VARCHAR(100) NOT NULL,
    base_price  DECIMAL(12, 2) NOT NULL DEFAULT 0.00,
    currency    VARCHAR(3) NOT NULL DEFAULT 'ZMW',
    sku         VARCHAR(100) UNIQUE,
    image_url   TEXT,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    attributes  JSONB,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_platform_products_category ON platform_products(category);
CREATE INDEX idx_platform_products_is_active ON platform_products(is_active);
