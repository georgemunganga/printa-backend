-- Store-scoped attendance. PINs are stored only as bcrypt hashes and never returned by the API.
ALTER TABLE store_staff
    ADD COLUMN IF NOT EXISTS pin_hash VARCHAR(255),
    ADD COLUMN IF NOT EXISTS pin_updated_at TIMESTAMP WITH TIME ZONE;

-- Every vendor-owned store must include the vendor owner as its initial manager so
-- attendance PIN enrolment is possible without exposing a user directory endpoint.
INSERT INTO store_staff (id, store_id, user_id, role)
SELECT gen_random_uuid(), s.id, v.owner_id, 'MANAGER'
FROM stores s
JOIN vendors v ON v.id = s.vendor_id
ON CONFLICT (store_id, user_id) DO NOTHING;

CREATE OR REPLACE FUNCTION ensure_store_owner_staff() RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO store_staff (id, store_id, user_id, role)
    SELECT gen_random_uuid(), NEW.id, v.owner_id, 'MANAGER'
    FROM vendors v
    WHERE v.id = NEW.vendor_id
    ON CONFLICT (store_id, user_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_ensure_store_owner_staff ON stores;
CREATE TRIGGER trg_ensure_store_owner_staff
AFTER INSERT ON stores
FOR EACH ROW EXECUTE FUNCTION ensure_store_owner_staff();

CREATE TABLE IF NOT EXISTS store_attendance_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type VARCHAR(16) NOT NULL CHECK (event_type IN ('CLOCK_IN', 'CLOCK_OUT')),
    occurred_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_store_attendance_events_store_user_time
    ON store_attendance_events(store_id, user_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_store_attendance_events_store_time
    ON store_attendance_events(store_id, occurred_at DESC);
