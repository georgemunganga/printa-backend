CREATE TABLE IF NOT EXISTS store_operating_hours (
    store_id    UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    day_of_week SMALLINT NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
    is_open     BOOLEAN NOT NULL DEFAULT FALSE,
    opens_at    TIME,
    closes_at   TIME,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (store_id, day_of_week),
    CHECK (
        (NOT is_open AND opens_at IS NULL AND closes_at IS NULL)
        OR
        (is_open AND opens_at IS NOT NULL AND closes_at IS NOT NULL AND opens_at < closes_at)
    )
);

CREATE INDEX IF NOT EXISTS idx_store_operating_hours_store_id
    ON store_operating_hours(store_id);
