package attendance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrPINNotConfigured = errors.New("staff PIN has not been configured")
	ErrNotAssigned      = errors.New("staff member is not assigned to this store")
)

type Repository interface {
	SetPIN(ctx context.Context, storeID, userID string, pinHash string) error
	GetStoreOwnerContact(ctx context.Context, storeID string) (*ownerContact, error)
	CreatePINReset(ctx context.Context, reset *PINResetRecord) error
	ConsumePINResetAndSetOwnerPIN(ctx context.Context, tokenHash, pinHash string, now time.Time) error
	GetPINHash(ctx context.Context, storeID, userID string) (string, error)
	GetLastEventType(ctx context.Context, storeID, userID string) (*EventType, error)
	CreateEvent(ctx context.Context, event *AttendanceEvent) error
	ListRecent(ctx context.Context, storeID string, limit int) ([]*AttendanceEvent, error)
}

type postgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func parseIDs(storeID, userID string) (uuid.UUID, uuid.UUID, error) {
	storeUUID, err := uuid.Parse(storeID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid store ID: %w", err)
	}
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid user ID: %w", err)
	}
	return storeUUID, userUUID, nil
}

func (r *postgresRepository) SetPIN(ctx context.Context, storeID, userID string, pinHash string) error {
	storeUUID, userUUID, err := parseIDs(storeID, userID)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE store_staff
		SET pin_hash = $3, pin_updated_at = NOW(), updated_at = NOW()
		WHERE store_id = $1 AND user_id = $2`, storeUUID, userUUID, pinHash)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return ErrNotAssigned
	}
	return nil
}

func (r *postgresRepository) GetStoreOwnerContact(ctx context.Context, storeID string) (*ownerContact, error) {
	storeUUID, err := uuid.Parse(storeID)
	if err != nil {
		return nil, fmt.Errorf("invalid store ID: %w", err)
	}
	contact := &ownerContact{StoreID: storeUUID}
	err = r.db.QueryRowContext(ctx, `
		SELECT v.owner_id, u.email
		FROM stores s
		JOIN vendors v ON v.id = s.vendor_id
		JOIN users u ON u.id = v.owner_id
		WHERE s.id = $1`, storeUUID).Scan(&contact.OwnerID, &contact.Email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("store not found")
	}
	if err != nil {
		return nil, err
	}
	return contact, nil
}

func (r *postgresRepository) CreatePINReset(ctx context.Context, reset *PINResetRecord) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE store_staff_pin_resets
		SET used_at = NOW()
		WHERE store_id = $1 AND owner_id = $2 AND used_at IS NULL`, reset.StoreID, reset.OwnerID)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO store_staff_pin_resets (id, store_id, owner_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5)`, reset.ID, reset.StoreID, reset.OwnerID, reset.TokenHash, reset.ExpiresAt)
	return err
}

func (r *postgresRepository) ConsumePINResetAndSetOwnerPIN(ctx context.Context, tokenHash, pinHash string, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var reset PINResetRecord
	err = tx.QueryRowContext(ctx, `
		SELECT id, store_id, owner_id, token_hash, expires_at
		FROM store_staff_pin_resets
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2
		FOR UPDATE`, tokenHash, now).Scan(&reset.ID, &reset.StoreID, &reset.OwnerID, &reset.TokenHash, &reset.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrPINResetInvalid
	}
	if err != nil {
		return err
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE store_staff
		SET pin_hash = $3, pin_updated_at = NOW(), updated_at = NOW()
		WHERE store_id = $1 AND user_id = $2`, reset.StoreID, reset.OwnerID, pinHash)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return ErrNotAssigned
	}
	if _, err := tx.ExecContext(ctx, `UPDATE store_staff_pin_resets SET used_at = $2 WHERE id = $1`, reset.ID, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *postgresRepository) GetPINHash(ctx context.Context, storeID, userID string) (string, error) {
	storeUUID, userUUID, err := parseIDs(storeID, userID)
	if err != nil {
		return "", err
	}
	var pinHash sql.NullString
	err = r.db.QueryRowContext(ctx, `
		SELECT pin_hash
		FROM store_staff
		WHERE store_id = $1 AND user_id = $2`, storeUUID, userUUID).Scan(&pinHash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotAssigned
	}
	if err != nil {
		return "", err
	}
	if !pinHash.Valid || pinHash.String == "" {
		return "", ErrPINNotConfigured
	}
	return pinHash.String, nil
}

func (r *postgresRepository) GetLastEventType(ctx context.Context, storeID, userID string) (*EventType, error) {
	storeUUID, userUUID, err := parseIDs(storeID, userID)
	if err != nil {
		return nil, err
	}
	var eventType EventType
	err = r.db.QueryRowContext(ctx, `
		SELECT event_type
		FROM store_attendance_events
		WHERE store_id = $1 AND user_id = $2
		ORDER BY occurred_at DESC, created_at DESC
		LIMIT 1`, storeUUID, userUUID).Scan(&eventType)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &eventType, nil
}

func (r *postgresRepository) CreateEvent(ctx context.Context, event *AttendanceEvent) error {
	return r.db.QueryRowContext(ctx, `
		INSERT INTO store_attendance_events (id, store_id, user_id, event_type, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING occurred_at, created_at`,
		event.ID, event.StoreID, event.UserID, event.EventType, event.CreatedBy,
	).Scan(&event.OccurredAt, &event.CreatedAt)
}

func (r *postgresRepository) ListRecent(ctx context.Context, storeID string, limit int) ([]*AttendanceEvent, error) {
	storeUUID, err := uuid.Parse(storeID)
	if err != nil {
		return nil, fmt.Errorf("invalid store ID: %w", err)
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, store_id, user_id, event_type, occurred_at, created_by, created_at
		FROM store_attendance_events
		WHERE store_id = $1
		ORDER BY occurred_at DESC, created_at DESC
		LIMIT $2`, storeUUID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*AttendanceEvent
	for rows.Next() {
		event := &AttendanceEvent{}
		if err := rows.Scan(
			&event.ID, &event.StoreID, &event.UserID, &event.EventType,
			&event.OccurredAt, &event.CreatedBy, &event.CreatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
