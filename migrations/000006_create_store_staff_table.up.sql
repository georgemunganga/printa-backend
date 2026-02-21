-- Store staff links users to stores with a specific role.
CREATE TABLE IF NOT EXISTS store_staff (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id   UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       VARCHAR(50) NOT NULL DEFAULT 'STAFF', -- MANAGER, STAFF, CASHIER
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(store_id, user_id)
);
CREATE INDEX idx_store_staff_store_id ON store_staff(store_id);
CREATE INDEX idx_store_staff_user_id  ON store_staff(user_id);
