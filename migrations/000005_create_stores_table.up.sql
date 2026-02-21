-- Stores belong to a vendor. A vendor can have multiple stores depending on their tier.
CREATE TABLE IF NOT EXISTS stores (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id   UUID NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    address     TEXT,
    city        VARCHAR(100),
    country     VARCHAR(100) NOT NULL DEFAULT 'Zambia',
    phone       VARCHAR(30),
    email       VARCHAR(255),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_stores_vendor_id ON stores(vendor_id);
