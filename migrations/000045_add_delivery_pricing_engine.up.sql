CREATE TABLE IF NOT EXISTS delivery_pricing_rules (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                VARCHAR(100) NOT NULL UNIQUE,
    max_distance_km     NUMERIC(8,2),
    base_fee            NUMERIC(12,2) NOT NULL CHECK (base_fee >= 0),
    per_km_fee          NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (per_km_fee >= 0),
    per_km_above_km     NUMERIC(8,2) NOT NULL DEFAULT 0 CHECK (per_km_above_km >= 0),
    currency            VARCHAR(3) NOT NULL DEFAULT 'ZMW',
    sort_order          INTEGER NOT NULL,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (max_distance_km IS NULL OR max_distance_km > 0)
);

INSERT INTO delivery_pricing_rules (name, max_distance_km, base_fee, per_km_fee, per_km_above_km, currency, sort_order)
VALUES
    ('0–3 km', 3, 30, 0, 0, 'ZMW', 10),
    ('4–5 km', 5, 40, 0, 0, 'ZMW', 20),
    ('6–10 km', 10, 50, 0, 0, 'ZMW', 30),
    ('11–15 km', 15, 75, 0, 0, 'ZMW', 40),
    ('16–20 km', 20, 100, 0, 0, 'ZMW', 50),
    ('20+ km', NULL, 100, 5, 20, 'ZMW', 60)
ON CONFLICT (name) DO UPDATE SET
    max_distance_km=EXCLUDED.max_distance_km,
    base_fee=EXCLUDED.base_fee,
    per_km_fee=EXCLUDED.per_km_fee,
    per_km_above_km=EXCLUDED.per_km_above_km,
    currency=EXCLUDED.currency,
    sort_order=EXCLUDED.sort_order,
    is_active=TRUE,
    updated_at=NOW();

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS delivery_fee NUMERIC(12,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS delivery_distance_km NUMERIC(8,2);
