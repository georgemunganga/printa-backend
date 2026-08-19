-- Versioned policy registry and vendor-consent evidence.
-- Only PUBLISHED required policies can trigger the vendor operating gate. Draft
-- policies remain visible to administrators but cannot be accepted as final terms.

CREATE TABLE platform_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(64) NOT NULL,
    version VARCHAR(32) NOT NULL,
    title VARCHAR(255) NOT NULL,
    summary TEXT NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'DRAFT',
    -- DRAFT | PUBLISHED | RETIRED
    required_for_vendor BOOLEAN NOT NULL DEFAULT FALSE,
    document_url TEXT,
    content_sha256 CHAR(64),
    effective_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    retired_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (slug IN ('vendor-terms', 'vendor-privacy-notice', 'vendor-acceptable-use')),
    CHECK (status IN ('DRAFT', 'PUBLISHED', 'RETIRED')),
    CHECK ((status = 'PUBLISHED') = (effective_at IS NOT NULL AND published_at IS NOT NULL)),
    CHECK (status <> 'PUBLISHED' OR (document_url IS NOT NULL AND length(trim(document_url)) > 0 AND content_sha256 IS NOT NULL)),
    CHECK (status <> 'RETIRED' OR retired_at IS NOT NULL),
    UNIQUE (slug, version)
);

CREATE UNIQUE INDEX uq_platform_policies_one_published_per_slug
    ON platform_policies (slug)
    WHERE status = 'PUBLISHED';

CREATE TABLE vendor_policy_consents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    vendor_id UUID REFERENCES vendors(id) ON DELETE SET NULL,
    policy_id UUID NOT NULL REFERENCES platform_policies(id) ON DELETE RESTRICT,
    policy_version VARCHAR(32) NOT NULL,
    accepted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_ip INET,
    user_agent TEXT,
    source VARCHAR(48) NOT NULL DEFAULT 'VENDOR_ONBOARDING',
    withdrawn_at TIMESTAMPTZ,
    withdrawal_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (source IN ('VENDOR_ONBOARDING', 'VENDOR_REACCEPTANCE', 'ADMIN_RECORDED')),
    UNIQUE (user_id, policy_id)
);

CREATE INDEX idx_vendor_policy_consents_user_current
    ON vendor_policy_consents (user_id, policy_id)
    WHERE withdrawn_at IS NULL;
CREATE INDEX idx_vendor_policy_consents_vendor_current
    ON vendor_policy_consents (vendor_id, policy_id)
    WHERE withdrawn_at IS NULL;

CREATE TABLE vendor_policy_consent_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    consent_id UUID NOT NULL REFERENCES vendor_policy_consents(id) ON DELETE RESTRICT,
    event_type VARCHAR(16) NOT NULL,
    -- ACCEPTED | WITHDRAWN | ATTACHED_TO_VENDOR
    event_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_ip INET,
    user_agent TEXT,
    detail JSONB NOT NULL DEFAULT '{}'::jsonb,
    CHECK (event_type IN ('ACCEPTED', 'WITHDRAWN', 'ATTACHED_TO_VENDOR'))
);

CREATE INDEX idx_vendor_policy_consent_events_consent
    ON vendor_policy_consent_events (consent_id, event_at ASC);

-- These rows deliberately remain DRAFT. They become publishable only after the
-- legal entity, contact, retention, hosting, and counsel-approval details are
-- completed through the controlled publication process.
INSERT INTO platform_policies (
    slug, version, title, summary, status, required_for_vendor
) VALUES
(
    'vendor-terms', 'v0.1-draft', 'Printa Vendor Platform Terms',
    'Working terms draft pending completion of legal-entity details and professional review.',
    'DRAFT', TRUE
),
(
    'vendor-privacy-notice', 'v0.1-draft', 'Printa Vendor Privacy Notice',
    'Working privacy-notice draft pending completion of controller, contact, processor, location, and retention details.',
    'DRAFT', TRUE
),
(
    'vendor-acceptable-use', 'v0.1-draft', 'Printa Acceptable Use and Print Content Policy',
    'Working acceptable-use and print-content draft pending professional review.',
    'DRAFT', FALSE
);
