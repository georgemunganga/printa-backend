DROP TABLE IF EXISTS store_attendance_events;

DROP TRIGGER IF EXISTS trg_ensure_store_owner_staff ON stores;
DROP FUNCTION IF EXISTS ensure_store_owner_staff();

ALTER TABLE store_staff
    DROP COLUMN IF EXISTS pin_updated_at,
    DROP COLUMN IF EXISTS pin_hash;
