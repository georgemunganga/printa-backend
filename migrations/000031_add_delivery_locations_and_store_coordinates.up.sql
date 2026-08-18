ALTER TABLE stores
    ADD COLUMN IF NOT EXISTS latitude NUMERIC(9,6),
    ADD COLUMN IF NOT EXISTS longitude NUMERIC(9,6);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'stores_coordinates_paired'
    ) THEN
        ALTER TABLE stores
            ADD CONSTRAINT stores_coordinates_paired
            CHECK ((latitude IS NULL) = (longitude IS NULL));
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'stores_latitude_range'
    ) THEN
        ALTER TABLE stores
            ADD CONSTRAINT stores_latitude_range
            CHECK (latitude IS NULL OR latitude BETWEEN -90 AND 90);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'stores_longitude_range'
    ) THEN
        ALTER TABLE stores
            ADD CONSTRAINT stores_longitude_range
            CHECK (longitude IS NULL OR longitude BETWEEN -180 AND 180);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS customer_delivery_locations (
    id UUID PRIMARY KEY,
    customer_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    recipient_name TEXT NOT NULL,
    recipient_phone TEXT NOT NULL,
    address_line1 TEXT NOT NULL,
    address_line2 TEXT,
    city TEXT NOT NULL,
    country TEXT NOT NULL DEFAULT 'Zambia',
    latitude NUMERIC(9,6),
    longitude NUMERIC(9,6),
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT customer_delivery_locations_coordinates_paired
        CHECK ((latitude IS NULL) = (longitude IS NULL)),
    CONSTRAINT customer_delivery_locations_latitude_range
        CHECK (latitude IS NULL OR latitude BETWEEN -90 AND 90),
    CONSTRAINT customer_delivery_locations_longitude_range
        CHECK (longitude IS NULL OR longitude BETWEEN -180 AND 180)
);

CREATE INDEX IF NOT EXISTS idx_customer_delivery_locations_customer_created
    ON customer_delivery_locations(customer_id, created_at ASC);

CREATE UNIQUE INDEX IF NOT EXISTS ux_customer_delivery_locations_default
    ON customer_delivery_locations(customer_id)
    WHERE is_default = TRUE;
