CREATE TABLE IF NOT EXISTS design_assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    original_name TEXT NOT NULL,
    content_type VARCHAR(255) NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 20971520),
    storage_provider VARCHAR(16) NOT NULL DEFAULT 'DATABASE',
    storage_key TEXT,
    content BYTEA,
    checksum_sha256 CHAR(64) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'READY',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT chk_design_asset_provider CHECK (storage_provider IN ('DATABASE', 'S3')),
    CONSTRAINT chk_design_asset_content CHECK ((storage_provider = 'DATABASE' AND content IS NOT NULL) OR (storage_provider = 'S3' AND storage_key IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS idx_design_assets_owner_created ON design_assets(owner_id, created_at DESC) WHERE deleted_at IS NULL;
