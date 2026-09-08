ALTER TABLE orders
    DROP COLUMN IF EXISTS delivery_distance_km,
    DROP COLUMN IF EXISTS delivery_fee;

DROP TABLE IF EXISTS delivery_pricing_rules;
