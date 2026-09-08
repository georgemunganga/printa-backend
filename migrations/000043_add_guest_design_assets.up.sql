ALTER TABLE design_assets
    ALTER COLUMN owner_id DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS guest_token_hash CHAR(64),
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;

ALTER TABLE design_assets
    ADD CONSTRAINT chk_design_asset_ownership
    CHECK (
        (owner_id IS NOT NULL AND guest_token_hash IS NULL AND expires_at IS NULL)
        OR
        (owner_id IS NULL AND guest_token_hash IS NOT NULL AND expires_at IS NOT NULL)
    );

CREATE INDEX IF NOT EXISTS idx_design_assets_guest_expiry
    ON design_assets(expires_at)
    WHERE owner_id IS NULL AND deleted_at IS NULL;
