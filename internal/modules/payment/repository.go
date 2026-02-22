package payment

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Repository defines data access for payment transactions.
type Repository interface {
	Create(ctx context.Context, tx *PaymentTransaction) error
	GetByID(ctx context.Context, id string) (*PaymentTransaction, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*PaymentTransaction, error)
	GetByProviderRef(ctx context.Context, provider Provider, ref string) (*PaymentTransaction, error)
	ListByReference(ctx context.Context, refType ReferenceType, refID string) ([]*PaymentTransaction, error)
	ListByVendor(ctx context.Context, vendorID string) ([]*PaymentTransaction, error)
	UpdateStatus(ctx context.Context, id string, status TxStatus, providerStatus string, lastError string) error
	UpdateProviderRef(ctx context.Context, id string, ref string, status string) error
	RecordWebhook(ctx context.Context, id string, payload interface{}) error
	IncrementRetry(ctx context.Context, id string, lastError string) error
}

type postgresRepo struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) Repository { return &postgresRepo{db: db} }

func (r *postgresRepo) Create(ctx context.Context, tx *PaymentTransaction) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO payment_transactions
		  (id, reference_type, reference_id, vendor_id, provider, provider_ref,
		   provider_status, status, amount, currency, phone_number, description,
		   idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		tx.ID, tx.ReferenceType, tx.ReferenceID, tx.VendorID,
		tx.Provider, nilIfEmpty(tx.ProviderRef), nilIfEmpty(tx.ProviderStatus),
		tx.Status, tx.Amount, tx.Currency,
		nilIfEmpty(tx.PhoneNumber), nilIfEmpty(tx.Description),
		nilIfEmpty(tx.IdempotencyKey))
	return err
}

func (r *postgresRepo) GetByID(ctx context.Context, id string) (*PaymentTransaction, error) {
	return r.scan(r.db.QueryRowContext(ctx, selectSQL+" WHERE id=$1", id))
}

func (r *postgresRepo) GetByIdempotencyKey(ctx context.Context, key string) (*PaymentTransaction, error) {
	return r.scan(r.db.QueryRowContext(ctx, selectSQL+" WHERE idempotency_key=$1", key))
}

func (r *postgresRepo) GetByProviderRef(ctx context.Context, provider Provider, ref string) (*PaymentTransaction, error) {
	return r.scan(r.db.QueryRowContext(ctx, selectSQL+" WHERE provider=$1 AND provider_ref=$2", provider, ref))
}

func (r *postgresRepo) ListByReference(ctx context.Context, refType ReferenceType, refID string) ([]*PaymentTransaction, error) {
	rows, err := r.db.QueryContext(ctx, selectSQL+" WHERE reference_type=$1 AND reference_id=$2 ORDER BY created_at DESC", refType, refID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanRows(rows)
}

func (r *postgresRepo) ListByVendor(ctx context.Context, vendorID string) ([]*PaymentTransaction, error) {
	rows, err := r.db.QueryContext(ctx, selectSQL+" WHERE vendor_id=$1 ORDER BY created_at DESC", vendorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanRows(rows)
}

func (r *postgresRepo) UpdateStatus(ctx context.Context, id string, status TxStatus, providerStatus string, lastError string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE payment_transactions
		SET status=$1, provider_status=COALESCE(NULLIF($2,''), provider_status),
		    last_error=COALESCE(NULLIF($3,''), last_error), updated_at=$4
		WHERE id=$5`,
		status, providerStatus, lastError, time.Now(), id)
	return err
}

func (r *postgresRepo) UpdateProviderRef(ctx context.Context, id string, ref string, status string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE payment_transactions SET provider_ref=$1, provider_status=$2, updated_at=$3 WHERE id=$4`,
		ref, status, time.Now(), id)
	return err
}

func (r *postgresRepo) RecordWebhook(ctx context.Context, id string, payload interface{}) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	now := time.Now()
	_, err = r.db.ExecContext(ctx, `
		UPDATE payment_transactions SET webhook_received_at=$1, webhook_payload=$2, updated_at=$3 WHERE id=$4`,
		now, b, now, id)
	return err
}

func (r *postgresRepo) IncrementRetry(ctx context.Context, id string, lastError string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE payment_transactions SET retry_count=retry_count+1, last_error=$1, updated_at=$2 WHERE id=$3`,
		lastError, time.Now(), id)
	return err
}

// ── Scanner ───────────────────────────────────────────────────────────────────

const selectSQL = `
	SELECT id, reference_type, reference_id, vendor_id, provider, provider_ref,
	       provider_status, status, amount, currency, phone_number, description,
	       webhook_received_at, webhook_payload, idempotency_key, retry_count,
	       last_error, metadata, created_at, updated_at
	FROM payment_transactions`

type rowScanner interface{ Scan(dest ...interface{}) error }

func (r *postgresRepo) scan(row rowScanner) (*PaymentTransaction, error) {
	tx := &PaymentTransaction{}
	var vendorID sql.NullString
	var providerRef, providerStatus, phone, desc, iKey, lastErr sql.NullString
	var webhookAt sql.NullTime
	var webhookPayload, metadata []byte

	err := row.Scan(
		&tx.ID, &tx.ReferenceType, &tx.ReferenceID, &vendorID,
		&tx.Provider, &providerRef, &providerStatus,
		&tx.Status, &tx.Amount, &tx.Currency,
		&phone, &desc, &webhookAt, &webhookPayload,
		&iKey, &tx.RetryCount, &lastErr, &metadata,
		&tx.CreatedAt, &tx.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if vendorID.Valid {
		id := uuid.MustParse(vendorID.String)
		tx.VendorID = &id
	}
	if providerRef.Valid {
		tx.ProviderRef = providerRef.String
	}
	if providerStatus.Valid {
		tx.ProviderStatus = providerStatus.String
	}
	if phone.Valid {
		tx.PhoneNumber = phone.String
	}
	if desc.Valid {
		tx.Description = desc.String
	}
	if iKey.Valid {
		tx.IdempotencyKey = iKey.String
	}
	if lastErr.Valid {
		tx.LastError = lastErr.String
	}
	if webhookAt.Valid {
		tx.WebhookReceivedAt = &webhookAt.Time
	}
	if len(webhookPayload) > 0 {
		var wp interface{}
		_ = json.Unmarshal(webhookPayload, &wp)
		tx.WebhookPayload = wp
	}
	if len(metadata) > 0 {
		var m interface{}
		_ = json.Unmarshal(metadata, &m)
		tx.Metadata = m
	}
	return tx, nil
}

func (r *postgresRepo) scanRows(rows *sql.Rows) ([]*PaymentTransaction, error) {
	var txs []*PaymentTransaction
	for rows.Next() {
		tx, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	if txs == nil {
		txs = []*PaymentTransaction{}
	}
	return txs, nil
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
