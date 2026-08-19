-- Vendor operating controls intentionally derive platform availability from only two
-- conditions: vendor compliance approval and subscription payment standing.

CREATE TABLE IF NOT EXISTS vendor_compliance_reviews (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id       UUID NOT NULL UNIQUE REFERENCES vendors(id) ON DELETE CASCADE,
    status          VARCHAR(16) NOT NULL DEFAULT 'PENDING',
    submitted_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at     TIMESTAMPTZ,
    reviewed_by     UUID REFERENCES users(id) ON DELETE SET NULL,
    decision_reason TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED')),
    CHECK ((status = 'PENDING' AND reviewed_at IS NULL AND reviewed_by IS NULL) OR status <> 'PENDING'),
    CHECK ((status = 'REJECTED' AND length(trim(COALESCE(decision_reason, ''))) > 0) OR status <> 'REJECTED')
);

CREATE INDEX IF NOT EXISTS idx_vendor_compliance_reviews_status
    ON vendor_compliance_reviews(status);

CREATE TABLE IF NOT EXISTS vendor_subscription_grace_periods (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id           UUID NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
    subscription_id     UUID NOT NULL REFERENCES vendor_subscriptions(id) ON DELETE CASCADE,
    requested_by        UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status              VARCHAR(16) NOT NULL DEFAULT 'ACTIVE',
    granted_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ends_at             TIMESTAMPTZ NOT NULL,
    subscription_end_at TIMESTAMPTZ NOT NULL,
    revoked_at          TIMESTAMPTZ,
    revoked_by          UUID REFERENCES users(id) ON DELETE SET NULL,
    revoke_reason       TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('ACTIVE', 'EXPIRED', 'REVOKED')),
    CHECK (ends_at > granted_at),
    CHECK ((status = 'REVOKED') = (revoked_at IS NOT NULL)),
    UNIQUE (vendor_id, subscription_end_at)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_vendor_subscription_grace_active
    ON vendor_subscription_grace_periods(vendor_id)
    WHERE status = 'ACTIVE';

CREATE INDEX IF NOT EXISTS idx_vendor_subscription_grace_ends_at
    ON vendor_subscription_grace_periods(ends_at)
    WHERE status = 'ACTIVE';

CREATE TABLE IF NOT EXISTS vendor_operating_reminders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id       UUID NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
    reminder_type   VARCHAR(32) NOT NULL,
    delivery_day    DATE NOT NULL,
    recipient       VARCHAR(320) NOT NULL,
    notification_id UUID,
    sent_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (reminder_type IN ('SUBSCRIPTION_DUE', 'GRACE_EXPIRING', 'KYC_PENDING', 'KYC_REJECTED')),
    UNIQUE (vendor_id, reminder_type, delivery_day)
);

CREATE INDEX IF NOT EXISTS idx_vendor_operating_reminders_unsent
    ON vendor_operating_reminders(created_at)
    WHERE sent_at IS NULL;

-- Existing vendors are placed in the same pending review state as new vendors.
-- A reviewer must explicitly approve them; there is no implicit compliance approval.
INSERT INTO vendor_compliance_reviews (vendor_id, status, submitted_at)
SELECT v.id, 'PENDING', v.created_at
FROM vendors v
ON CONFLICT (vendor_id) DO NOTHING;

CREATE OR REPLACE FUNCTION create_pending_vendor_compliance_review()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO vendor_compliance_reviews (vendor_id, status, submitted_at)
    VALUES (NEW.id, 'PENDING', NOW())
    ON CONFLICT (vendor_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS vendor_compliance_review_on_create ON vendors;
CREATE TRIGGER vendor_compliance_review_on_create
AFTER INSERT ON vendors
FOR EACH ROW EXECUTE FUNCTION create_pending_vendor_compliance_review();

CREATE OR REPLACE FUNCTION prevent_vendor_operating_reminder_mutation()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'vendor operating reminder records are append-only';
    END IF;
    IF OLD.vendor_id <> NEW.vendor_id
       OR OLD.reminder_type <> NEW.reminder_type
       OR OLD.delivery_day <> NEW.delivery_day
       OR OLD.recipient <> NEW.recipient
       OR OLD.notification_id IS DISTINCT FROM NEW.notification_id
       OR OLD.created_at <> NEW.created_at THEN
        RAISE EXCEPTION 'vendor operating reminder identity is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS vendor_operating_reminder_immutable ON vendor_operating_reminders;
CREATE TRIGGER vendor_operating_reminder_immutable
BEFORE UPDATE OR DELETE ON vendor_operating_reminders
FOR EACH ROW EXECUTE FUNCTION prevent_vendor_operating_reminder_mutation();
