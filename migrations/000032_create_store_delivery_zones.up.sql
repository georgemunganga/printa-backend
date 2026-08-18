CREATE TABLE IF NOT EXISTS store_delivery_zones (
    id UUID PRIMARY KEY,
    store_id UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    city TEXT NOT NULL,
    country TEXT NOT NULL DEFAULT 'Zambia',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_store_delivery_zones_store
    ON store_delivery_zones(store_id, is_active);

CREATE UNIQUE INDEX IF NOT EXISTS ux_store_delivery_zones_city_country
    ON store_delivery_zones(store_id, LOWER(city), LOWER(country));
