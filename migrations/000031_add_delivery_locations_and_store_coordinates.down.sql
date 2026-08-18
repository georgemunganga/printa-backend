DROP INDEX IF EXISTS ux_customer_delivery_locations_default;
DROP INDEX IF EXISTS idx_customer_delivery_locations_customer_created;
DROP TABLE IF EXISTS customer_delivery_locations;

ALTER TABLE stores
    DROP CONSTRAINT IF EXISTS stores_longitude_range,
    DROP CONSTRAINT IF EXISTS stores_latitude_range,
    DROP CONSTRAINT IF EXISTS stores_coordinates_paired,
    DROP COLUMN IF EXISTS longitude,
    DROP COLUMN IF EXISTS latitude;
