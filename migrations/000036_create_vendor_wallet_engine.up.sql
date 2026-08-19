-- Printa vendor virtual-wallet foundation.
-- This migration creates an internal append-only ledger only. It does not activate
-- collection, disbursement, provider re-query, or customer-fund movement.

CREATE TABLE wallet_fee_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version VARCHAR(64) NOT NULL UNIQUE,
    activity VARCHAR(32) NOT NULL,
    -- COLLECTION | DEPOSIT | POS | WITHDRAWAL
    currency CHAR(3) NOT NULL DEFAULT 'ZMW',
    charge_basis VARCHAR(32) NOT NULL,
    -- PERCENT_OF_TRANSACTION | PERCENT_OF_PROVIDER_FEE | FIXED_MINOR
    percentage_bps INTEGER,
    fixed_amount_minor BIGINT,
    effective_from TIMESTAMPTZ NOT NULL,
    effective_to TIMESTAMPTZ,
    status VARCHAR(16) NOT NULL DEFAULT 'DRAFT',
    -- DRAFT | ACTIVE | RETIRED
    description TEXT,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    retired_at TIMESTAMPTZ,
    CHECK (activity IN ('COLLECTION', 'DEPOSIT', 'POS', 'WITHDRAWAL')),
    CHECK (charge_basis IN ('PERCENT_OF_TRANSACTION', 'PERCENT_OF_PROVIDER_FEE', 'FIXED_MINOR')),
    CHECK (status IN ('DRAFT', 'ACTIVE', 'RETIRED')),
    CHECK (percentage_bps IS NULL OR percentage_bps BETWEEN 0 AND 1000000),
    CHECK (fixed_amount_minor IS NULL OR fixed_amount_minor >= 0),
    CHECK (
        (charge_basis = 'FIXED_MINOR' AND fixed_amount_minor IS NOT NULL AND percentage_bps IS NULL)
        OR
        (charge_basis IN ('PERCENT_OF_TRANSACTION', 'PERCENT_OF_PROVIDER_FEE')
            AND percentage_bps IS NOT NULL AND fixed_amount_minor IS NULL)
    ),
    CHECK (effective_to IS NULL OR effective_to > effective_from)
);

CREATE UNIQUE INDEX uq_wallet_fee_policy_active_window
    ON wallet_fee_policies (activity, currency, effective_from)
    WHERE status = 'ACTIVE';

CREATE TABLE vendor_wallet_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id UUID NOT NULL UNIQUE REFERENCES vendors(id) ON DELETE RESTRICT,
    currency CHAR(3) NOT NULL DEFAULT 'ZMW',
    state VARCHAR(16) NOT NULL DEFAULT 'PENDING',
    -- PENDING | ACTIVE | SUSPENDED | CLOSED
    provider_virtual_account_reference VARCHAR(160),
    provider_account_status VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    activated_at TIMESTAMPTZ,
    suspended_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    CHECK (state IN ('PENDING', 'ACTIVE', 'SUSPENDED', 'CLOSED')),
    CHECK ((state = 'ACTIVE') = (activated_at IS NOT NULL))
);

CREATE UNIQUE INDEX uq_vendor_wallet_provider_reference
    ON vendor_wallet_accounts (provider_virtual_account_reference)
    WHERE provider_virtual_account_reference IS NOT NULL;

-- A journal is the idempotent accounting event. All of its entries must net to
-- zero; the posting service validates that invariant before insert.
CREATE TABLE wallet_journals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key VARCHAR(160) NOT NULL UNIQUE,
    source_type VARCHAR(48) NOT NULL,
    source_reference VARCHAR(160) NOT NULL,
    provider_reference VARCHAR(160),
    lenco_webhook_event_id UUID REFERENCES lenco_webhook_events(id) ON DELETE SET NULL,
    currency CHAR(3) NOT NULL DEFAULT 'ZMW',
    narrative TEXT NOT NULL,
    actor_type VARCHAR(24) NOT NULL DEFAULT 'SYSTEM',
    -- SYSTEM | USER | ADMIN | RECONCILIATION
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    CHECK (source_type <> ''),
    CHECK (source_reference <> ''),
    CHECK (narrative <> ''),
    CHECK (actor_type IN ('SYSTEM', 'USER', 'ADMIN', 'RECONCILIATION'))
);

CREATE INDEX idx_wallet_journals_source ON wallet_journals (source_type, source_reference);
CREATE INDEX idx_wallet_journals_provider_reference ON wallet_journals (provider_reference) WHERE provider_reference IS NOT NULL;
CREATE INDEX idx_wallet_journals_lenco_event ON wallet_journals (lenco_webhook_event_id) WHERE lenco_webhook_event_id IS NOT NULL;

CREATE TABLE wallet_ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    journal_id UUID NOT NULL REFERENCES wallet_journals(id) ON DELETE RESTRICT,
    wallet_account_id UUID REFERENCES vendor_wallet_accounts(id) ON DELETE RESTRICT,
    vendor_id UUID REFERENCES vendors(id) ON DELETE RESTRICT,
    entry_type VARCHAR(40) NOT NULL,
    ledger_account VARCHAR(40) NOT NULL,
    -- VENDOR_AVAILABLE | VENDOR_PENDING | VENDOR_HELD | PLATFORM_CLEARING |
    -- PLATFORM_TRANSACTION_CHARGE | PLATFORM_EXPENSE
    amount_minor BIGINT NOT NULL,
    currency CHAR(3) NOT NULL DEFAULT 'ZMW',
    fee_policy_id UUID REFERENCES wallet_fee_policies(id) ON DELETE SET NULL,
    provider_reference VARCHAR(160),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    CHECK (entry_type IN (
        'ORDER_SALE_CREDIT',
        'POS_CASH_RECEIPT',
        'POS_CARD_RECEIPT',
        'MANUAL_DEPOSIT_PENDING_REVIEW',
        'COLLECTION_PENDING',
        'COLLECTION_SETTLED',
        'COLLECTION_REVERSED',
        'TRANSACTION_CHARGE',
        'VENDOR_EXPENSE_DEBIT',
        'REFUND_DEBIT',
        'WITHDRAWAL_HOLD',
        'WITHDRAWAL_PAID',
        'WITHDRAWAL_FAILED_RELEASE',
        'ADJUSTMENT'
    )),
    CHECK (ledger_account IN (
        'VENDOR_AVAILABLE',
        'VENDOR_PENDING',
        'VENDOR_HELD',
        'PLATFORM_CLEARING',
        'PLATFORM_TRANSACTION_CHARGE',
        'PLATFORM_EXPENSE'
    )),
    CHECK (amount_minor <> 0),
    CHECK (
        (ledger_account IN ('VENDOR_AVAILABLE', 'VENDOR_PENDING', 'VENDOR_HELD')
            AND wallet_account_id IS NOT NULL AND vendor_id IS NOT NULL)
        OR
        (ledger_account NOT IN ('VENDOR_AVAILABLE', 'VENDOR_PENDING', 'VENDOR_HELD')
            AND wallet_account_id IS NULL AND vendor_id IS NULL)
    )
);

CREATE INDEX idx_wallet_ledger_entries_wallet_account_created
    ON wallet_ledger_entries (wallet_account_id, created_at DESC)
    WHERE wallet_account_id IS NOT NULL;
CREATE INDEX idx_wallet_ledger_entries_vendor_created
    ON wallet_ledger_entries (vendor_id, created_at DESC)
    WHERE vendor_id IS NOT NULL;
CREATE INDEX idx_wallet_ledger_entries_journal ON wallet_ledger_entries (journal_id);
CREATE INDEX idx_wallet_ledger_entries_entry_type ON wallet_ledger_entries (entry_type, created_at DESC);

CREATE TABLE wallet_balance_snapshots (
    wallet_account_id UUID PRIMARY KEY REFERENCES vendor_wallet_accounts(id) ON DELETE CASCADE,
    currency CHAR(3) NOT NULL DEFAULT 'ZMW',
    available_minor BIGINT NOT NULL DEFAULT 0,
    pending_minor BIGINT NOT NULL DEFAULT 0,
    held_minor BIGINT NOT NULL DEFAULT 0,
    calculated_through_journal_id UUID REFERENCES wallet_journals(id) ON DELETE SET NULL,
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE wallet_withdrawal_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_account_id UUID NOT NULL REFERENCES vendor_wallet_accounts(id) ON DELETE RESTRICT,
    vendor_id UUID NOT NULL REFERENCES vendors(id) ON DELETE RESTRICT,
    amount_minor BIGINT NOT NULL,
    currency CHAR(3) NOT NULL DEFAULT 'ZMW',
    destination_reference VARCHAR(160),
    idempotency_key VARCHAR(160) NOT NULL UNIQUE,
    status VARCHAR(24) NOT NULL DEFAULT 'PENDING_REVIEW',
    -- PENDING_REVIEW | APPROVED | SUBMITTED | PAID | FAILED | CANCELLED | REJECTED
    provider_reference VARCHAR(160),
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    completed_at TIMESTAMPTZ,
    failure_reason TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    CHECK (amount_minor > 0),
    CHECK (status IN ('PENDING_REVIEW', 'APPROVED', 'SUBMITTED', 'PAID', 'FAILED', 'CANCELLED', 'REJECTED'))
);

CREATE INDEX idx_wallet_withdrawal_requests_vendor_created
    ON wallet_withdrawal_requests (vendor_id, requested_at DESC);
CREATE INDEX idx_wallet_withdrawal_requests_status
    ON wallet_withdrawal_requests (status, requested_at DESC);

CREATE TABLE wallet_reconciliation_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_account_id UUID REFERENCES vendor_wallet_accounts(id) ON DELETE SET NULL,
    lenco_webhook_event_id UUID REFERENCES lenco_webhook_events(id) ON DELETE SET NULL,
    wallet_journal_id UUID REFERENCES wallet_journals(id) ON DELETE SET NULL,
    external_reference VARCHAR(160) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'PENDING',
    -- PENDING | MATCHED | UNMATCHED | RETRY_REQUIRED | RESOLVED
    event_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    resolution_note TEXT,
    reconciled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('PENDING', 'MATCHED', 'UNMATCHED', 'RETRY_REQUIRED', 'RESOLVED'))
);

CREATE UNIQUE INDEX uq_wallet_reconciliation_lenco_event
    ON wallet_reconciliation_events (lenco_webhook_event_id)
    WHERE lenco_webhook_event_id IS NOT NULL;
CREATE INDEX idx_wallet_reconciliation_external_reference
    ON wallet_reconciliation_events (external_reference, created_at DESC);

CREATE OR REPLACE FUNCTION prevent_wallet_ledger_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'wallet journals and ledger entries are immutable';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_wallet_journals_immutable
    BEFORE UPDATE OR DELETE ON wallet_journals
    FOR EACH ROW EXECUTE FUNCTION prevent_wallet_ledger_mutation();

CREATE TRIGGER trg_wallet_ledger_entries_immutable
    BEFORE UPDATE OR DELETE ON wallet_ledger_entries
    FOR EACH ROW EXECUTE FUNCTION prevent_wallet_ledger_mutation();

-- Deferred enforcement allows a posting service to insert all sides of a journal
-- in one transaction, but prevents any unbalanced journal from committing.
CREATE OR REPLACE FUNCTION enforce_wallet_journal_balance()
RETURNS TRIGGER AS $$
DECLARE
    balanced_total BIGINT;
BEGIN
    SELECT COALESCE(SUM(amount_minor), 0)
    INTO balanced_total
    FROM wallet_ledger_entries
    WHERE journal_id = NEW.journal_id;

    IF balanced_total <> 0 THEN
        RAISE EXCEPTION 'wallet journal % is not balanced', NEW.journal_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER trg_wallet_journal_balanced
    AFTER INSERT ON wallet_ledger_entries
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION enforce_wallet_journal_balance();
