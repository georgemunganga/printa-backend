CREATE TABLE IF NOT EXISTS auth_oauth_states (
    state        VARCHAR(128) PRIMARY KEY,
    redirect_uri TEXT NOT NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    consumed_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_auth_oauth_states_expires_at ON auth_oauth_states(expires_at);

CREATE TABLE IF NOT EXISTS auth_oauth_identities (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider     VARCHAR(50) NOT NULL,
    provider_sub VARCHAR(255) NOT NULL,
    email        VARCHAR(255) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_sub)
);

CREATE INDEX IF NOT EXISTS idx_auth_oauth_identities_user_id ON auth_oauth_identities(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_oauth_identities_email ON auth_oauth_identities(email);
