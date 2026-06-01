CREATE TABLE IF NOT EXISTS auth_otp_challenges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    purpose VARCHAR(20) NOT NULL,
    method VARCHAR(20) NOT NULL,
    destination VARCHAR(255) NOT NULL,
    code_hash VARCHAR(128) NOT NULL,
    payload JSONB,
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    consumed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_auth_otp_destination ON auth_otp_challenges(destination);
CREATE INDEX IF NOT EXISTS idx_auth_otp_expires_at ON auth_otp_challenges(expires_at);
