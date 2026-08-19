-- Single-use reset links for the vendor owner's first-store staff PIN.
-- Raw email-link tokens are never persisted; only SHA-256 digests are stored.
CREATE TABLE IF NOT EXISTS store_staff_pin_resets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash CHAR(64) NOT NULL UNIQUE,
    requested_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_store_staff_pin_resets_active
    ON store_staff_pin_resets (store_id, owner_id, expires_at DESC)
    WHERE used_at IS NULL;
