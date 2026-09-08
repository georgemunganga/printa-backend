DROP INDEX IF EXISTS idx_design_assets_guest_expiry;

ALTER TABLE design_assets
    DROP CONSTRAINT IF EXISTS chk_design_asset_ownership,
    DROP COLUMN IF EXISTS expires_at,
    DROP COLUMN IF EXISTS guest_token_hash,
    ALTER COLUMN owner_id SET NOT NULL;
