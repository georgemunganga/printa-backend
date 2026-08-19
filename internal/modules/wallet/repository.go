package wallet

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var ErrAccountNotFound = errors.New("wallet account not found")

type Repository interface {
	GetAccountByVendor(ctx context.Context, vendorID uuid.UUID) (*Account, error)
	GetBalance(ctx context.Context, walletAccountID uuid.UUID) (*Balance, error)
	ListEntries(ctx context.Context, walletAccountID uuid.UUID, limit int) ([]LedgerEntry, error)
	ListWithdrawals(ctx context.Context, vendorID uuid.UUID, limit int) ([]WithdrawalRequest, error)
	Post(ctx context.Context, posting Posting) (uuid.UUID, bool, error)
}

type postgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) GetAccountByVendor(ctx context.Context, vendorID uuid.UUID) (*Account, error) {
	var account Account
	var providerRef, providerStatus sql.NullString
	var activatedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, vendor_id, currency, state, provider_virtual_account_reference,
		       provider_account_status, created_at, updated_at, activated_at
		FROM vendor_wallet_accounts
		WHERE vendor_id = $1`, vendorID).Scan(
		&account.ID, &account.VendorID, &account.Currency, &account.State, &providerRef,
		&providerStatus, &account.CreatedAt, &account.UpdatedAt, &activatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, err
	}
	account.ProviderVirtualAccountRef = providerRef.String
	account.ProviderAccountStatus = providerStatus.String
	if activatedAt.Valid {
		account.ActivatedAt = &activatedAt.Time
	}
	return &account, nil
}

func (r *postgresRepository) GetBalance(ctx context.Context, walletAccountID uuid.UUID) (*Balance, error) {
	var balance Balance
	err := r.db.QueryRowContext(ctx, `
		SELECT wallet_account_id, currency, available_minor, pending_minor, held_minor, calculated_at
		FROM wallet_balance_snapshots WHERE wallet_account_id = $1`, walletAccountID).Scan(
		&balance.WalletAccountID, &balance.Currency, &balance.AvailableMinor,
		&balance.PendingMinor, &balance.HeldMinor, &balance.CalculatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return &Balance{WalletAccountID: walletAccountID, Currency: defaultCurrency, CalculatedAt: time.Now().UTC()}, nil
	}
	if err != nil {
		return nil, err
	}
	return &balance, nil
}

func (r *postgresRepository) ListEntries(ctx context.Context, walletAccountID uuid.UUID, limit int) ([]LedgerEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, journal_id, wallet_account_id, vendor_id, entry_type, ledger_account,
		       amount_minor, currency, provider_reference, created_at
		FROM wallet_ledger_entries
		WHERE wallet_account_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2`, walletAccountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]LedgerEntry, 0)
	for rows.Next() {
		var entry LedgerEntry
		var walletAccountID, vendorID uuid.NullUUID
		var providerRef sql.NullString
		if err := rows.Scan(&entry.ID, &entry.JournalID, &walletAccountID, &vendorID, &entry.EntryType,
			&entry.LedgerAccount, &entry.AmountMinor, &entry.Currency, &providerRef, &entry.CreatedAt); err != nil {
			return nil, err
		}
		if walletAccountID.Valid {
			entry.WalletAccountID = &walletAccountID.UUID
		}
		if vendorID.Valid {
			entry.VendorID = &vendorID.UUID
		}
		entry.ProviderRef = providerRef.String
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (r *postgresRepository) ListWithdrawals(ctx context.Context, vendorID uuid.UUID, limit int) ([]WithdrawalRequest, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, wallet_account_id, vendor_id, amount_minor, currency, status,
		       requested_at, reviewed_at, completed_at, failure_reason
		FROM wallet_withdrawal_requests
		WHERE vendor_id = $1
		ORDER BY requested_at DESC, id DESC
		LIMIT $2`, vendorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	withdrawals := make([]WithdrawalRequest, 0)
	for rows.Next() {
		var request WithdrawalRequest
		var reviewedAt, completedAt sql.NullTime
		var failureReason sql.NullString
		if err := rows.Scan(&request.ID, &request.WalletAccountID, &request.VendorID, &request.AmountMinor,
			&request.Currency, &request.Status, &request.RequestedAt, &reviewedAt, &completedAt, &failureReason); err != nil {
			return nil, err
		}
		if reviewedAt.Valid {
			request.ReviewedAt = &reviewedAt.Time
		}
		if completedAt.Valid {
			request.CompletedAt = &completedAt.Time
		}
		request.FailureReason = failureReason.String
		withdrawals = append(withdrawals, request)
	}
	return withdrawals, rows.Err()
}

func (r *postgresRepository) Post(ctx context.Context, posting Posting) (uuid.UUID, bool, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return uuid.Nil, false, err
	}
	defer func() { _ = tx.Rollback() }()

	metadata := metadataOrEmpty(posting.Metadata)
	var journalID uuid.UUID
	err = tx.QueryRowContext(ctx, `
		INSERT INTO wallet_journals (
			idempotency_key, source_type, source_reference, provider_reference, currency,
			narrative, actor_type, actor_id, occurred_at, metadata
		) VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,$9,$10::jsonb)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id`, posting.IdempotencyKey, posting.SourceType, posting.SourceReference,
		posting.ProviderReference, posting.Currency, posting.Narrative, posting.ActorType,
		posting.ActorID, posting.OccurredAt, metadata).Scan(&journalID)
	if errors.Is(err, sql.ErrNoRows) {
		var sourceType, sourceReference string
		err = tx.QueryRowContext(ctx, `
			SELECT id, source_type, source_reference FROM wallet_journals WHERE idempotency_key = $1`,
			posting.IdempotencyKey).Scan(&journalID, &sourceType, &sourceReference)
		if err != nil {
			return uuid.Nil, false, err
		}
		if sourceType != posting.SourceType || sourceReference != posting.SourceReference {
			return uuid.Nil, false, fmt.Errorf("idempotency key was already used for a different source")
		}
		if err := tx.Commit(); err != nil {
			return uuid.Nil, false, err
		}
		return journalID, true, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}

	walletAccounts := make(map[uuid.UUID]struct{})
	for _, entry := range posting.Entries {
		if entry.WalletAccountID != nil {
			var accountVendorID uuid.UUID
			var accountCurrency string
			err = tx.QueryRowContext(ctx, `
				SELECT vendor_id, currency FROM vendor_wallet_accounts WHERE id = $1 FOR KEY SHARE`,
				*entry.WalletAccountID,
			).Scan(&accountVendorID, &accountCurrency)
			if errors.Is(err, sql.ErrNoRows) {
				return uuid.Nil, false, ErrAccountNotFound
			}
			if err != nil {
				return uuid.Nil, false, err
			}
			if entry.VendorID == nil || accountVendorID != *entry.VendorID {
				return uuid.Nil, false, fmt.Errorf("wallet posting entry vendor does not own the wallet account")
			}
			if accountCurrency != posting.Currency {
				return uuid.Nil, false, fmt.Errorf("wallet posting currency does not match the wallet account")
			}
		}
		entryMetadata := metadataOrEmpty(entry.Metadata)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO wallet_ledger_entries (
				journal_id, wallet_account_id, vendor_id, entry_type, ledger_account,
				amount_minor, currency, fee_policy_id, provider_reference, metadata
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),$10::jsonb)`,
			journalID, entry.WalletAccountID, entry.VendorID, entry.EntryType, entry.LedgerAccount,
			entry.AmountMinor, posting.Currency, entry.FeePolicyID, entry.ProviderReference, entryMetadata)
		if err != nil {
			return uuid.Nil, false, err
		}
		if entry.WalletAccountID != nil {
			walletAccounts[*entry.WalletAccountID] = struct{}{}
		}
	}
	for walletAccountID := range walletAccounts {
		if err := refreshBalanceSnapshot(ctx, tx, walletAccountID, posting.Currency, journalID); err != nil {
			return uuid.Nil, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return uuid.Nil, false, err
	}
	return journalID, false, nil
}

func refreshBalanceSnapshot(ctx context.Context, tx *sql.Tx, walletAccountID uuid.UUID, currency string, journalID uuid.UUID) error {
	var available, pending, held int64
	err := tx.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(amount_minor) FILTER (WHERE ledger_account = 'VENDOR_AVAILABLE'), 0),
			COALESCE(SUM(amount_minor) FILTER (WHERE ledger_account = 'VENDOR_PENDING'), 0),
			COALESCE(SUM(amount_minor) FILTER (WHERE ledger_account = 'VENDOR_HELD'), 0)
		FROM wallet_ledger_entries WHERE wallet_account_id = $1`, walletAccountID).Scan(&available, &pending, &held)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO wallet_balance_snapshots (
			wallet_account_id, currency, available_minor, pending_minor, held_minor,
			calculated_through_journal_id, calculated_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,NOW(),NOW())
		ON CONFLICT (wallet_account_id) DO UPDATE SET
			currency = EXCLUDED.currency,
			available_minor = EXCLUDED.available_minor,
			pending_minor = EXCLUDED.pending_minor,
			held_minor = EXCLUDED.held_minor,
			calculated_through_journal_id = EXCLUDED.calculated_through_journal_id,
			calculated_at = EXCLUDED.calculated_at,
			updated_at = NOW()`, walletAccountID, currency, available, pending, held, journalID)
	return err
}

func metadataOrEmpty(value json.RawMessage) string {
	if len(value) == 0 || !json.Valid(value) {
		return "{}"
	}
	return string(value)
}
