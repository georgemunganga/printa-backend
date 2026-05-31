package comms

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Repository persists delivery logs for audit and retry.
type Repository interface {
	Create(ctx context.Context, log *DeliveryLog) error
	GetByID(ctx context.Context, id string) (*DeliveryLog, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*DeliveryLog, error)
	UpdateStatus(ctx context.Context, id string, status DeliveryStatus, providerRef, errMsg string) error
	List(ctx context.Context, filter ListFilter) ([]*DeliveryLog, int, error)
}

type postgresRepository struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, log *DeliveryLog) error {
	log.ID = uuid.New().String()
	log.CreatedAt = time.Now()
	log.UpdatedAt = time.Now()

	var metaJSON []byte
	if log.Body != "" {
		metaJSON = nil
	}
	_ = metaJSON

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO comms_delivery_logs
		  (id, channel, recipient, recipient_id, subject, body, status,
		   provider_ref, error_message, retry_count, idempotency_key, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		log.ID, log.Channel, log.Recipient, nullStr(log.RecipientID),
		nullStr(log.Subject), log.Body, log.Status,
		nullStr(log.ProviderRef), nullStr(log.ErrorMessage),
		log.RetryCount, nullStr(log.IdempotencyKey),
		log.CreatedAt, log.UpdatedAt,
	)
	return err
}

func (r *postgresRepository) GetByID(ctx context.Context, id string) (*DeliveryLog, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, channel, recipient, COALESCE(recipient_id,''), COALESCE(subject,''),
		       body, status, COALESCE(provider_ref,''), COALESCE(error_message,''),
		       retry_count, COALESCE(idempotency_key,''), sent_at, created_at, updated_at
		FROM comms_delivery_logs WHERE id = $1`, id)
	return scanLog(row)
}

func (r *postgresRepository) GetByIdempotencyKey(ctx context.Context, key string) (*DeliveryLog, error) {
	if key == "" {
		return nil, fmt.Errorf("idempotency key is empty")
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, channel, recipient, COALESCE(recipient_id,''), COALESCE(subject,''),
		       body, status, COALESCE(provider_ref,''), COALESCE(error_message,''),
		       retry_count, COALESCE(idempotency_key,''), sent_at, created_at, updated_at
		FROM comms_delivery_logs WHERE idempotency_key = $1 LIMIT 1`, key)
	return scanLog(row)
}

func (r *postgresRepository) UpdateStatus(ctx context.Context, id string, status DeliveryStatus, providerRef, errMsg string) error {
	now := time.Now()
	var sentAt interface{}
	if status == DeliverySent || status == DeliveryDelivered {
		sentAt = now
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE comms_delivery_logs
		SET status=$1, provider_ref=$2, error_message=$3, sent_at=$4, updated_at=$5,
		    retry_count = retry_count + 1
		WHERE id=$6`,
		status, nullStr(providerRef), nullStr(errMsg), sentAt, now, id,
	)
	return err
}

func (r *postgresRepository) List(ctx context.Context, filter ListFilter) ([]*DeliveryLog, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	i := 1
	if filter.Channel != "" {
		where += fmt.Sprintf(" AND channel=$%d", i)
		args = append(args, filter.Channel)
		i++
	}
	if filter.RecipientID != "" {
		where += fmt.Sprintf(" AND recipient_id=$%d", i)
		args = append(args, filter.RecipientID)
		i++
	}
	if filter.Status != "" {
		where += fmt.Sprintf(" AND status=$%d", i)
		args = append(args, filter.Status)
		i++
	}

	var total int
	r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM comms_delivery_logs "+where, args...).Scan(&total)

	if filter.Limit == 0 {
		filter.Limit = 20
	}
	args = append(args, filter.Limit, filter.Offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, channel, recipient, COALESCE(recipient_id,''), COALESCE(subject,''),
		       body, status, COALESCE(provider_ref,''), COALESCE(error_message,''),
		       retry_count, COALESCE(idempotency_key,''), sent_at, created_at, updated_at
		FROM comms_delivery_logs `+where+
		fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", i, i+1),
		args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*DeliveryLog
	for rows.Next() {
		log, err := scanLogRow(rows)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, log)
	}
	return logs, total, nil
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func scanLog(row *sql.Row) (*DeliveryLog, error) {
	var log DeliveryLog
	var sentAt sql.NullTime
	err := row.Scan(
		&log.ID, &log.Channel, &log.Recipient, &log.RecipientID,
		&log.Subject, &log.Body, &log.Status, &log.ProviderRef,
		&log.ErrorMessage, &log.RetryCount, &log.IdempotencyKey,
		&sentAt, &log.CreatedAt, &log.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if sentAt.Valid {
		log.SentAt = &sentAt.Time
	}
	return &log, nil
}

func scanLogRow(rows *sql.Rows) (*DeliveryLog, error) {
	var log DeliveryLog
	var sentAt sql.NullTime
	err := rows.Scan(
		&log.ID, &log.Channel, &log.Recipient, &log.RecipientID,
		&log.Subject, &log.Body, &log.Status, &log.ProviderRef,
		&log.ErrorMessage, &log.RetryCount, &log.IdempotencyKey,
		&sentAt, &log.CreatedAt, &log.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if sentAt.Valid {
		log.SentAt = &sentAt.Time
	}
	return &log, nil
}

// ensure json import is used
var _ = json.Marshal
