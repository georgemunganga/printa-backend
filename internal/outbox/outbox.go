package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending    Status = "PENDING"
	StatusProcessing Status = "PROCESSING"
	StatusDelivered  Status = "DELIVERED"
	StatusFailed     Status = "FAILED"
	StatusDeadLetter Status = "DEAD_LETTER"
)

type Event struct {
	ID            uuid.UUID       `json:"id"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   uuid.UUID       `json:"aggregate_id"`
	EventType     string          `json:"event_type"`
	Payload       json.RawMessage `json:"payload"`
	Status        Status          `json:"status"`
	Attempts      int             `json:"attempts"`
	AvailableAt   time.Time       `json:"available_at"`
	CreatedAt     time.Time       `json:"created_at"`
}

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// Enqueue records a domain event in the same database. A future worker may safely claim and deliver it.
func (r *Repository) Enqueue(ctx context.Context, event Event) (Event, error) {
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if len(event.Payload) == 0 {
		event.Payload = json.RawMessage(`{}`)
	}
	if event.Status == "" {
		event.Status = StatusPending
	}
	if event.AvailableAt.IsZero() {
		event.AvailableAt = time.Now().UTC()
	}
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, status, available_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING attempts, created_at`, event.ID, event.AggregateType, event.AggregateID, event.EventType, event.Payload, event.Status, event.AvailableAt,
	).Scan(&event.Attempts, &event.CreatedAt)
	return event, err
}

func (r *Repository) ListPending(ctx context.Context, limit int) ([]Event, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, status, attempts, available_at, created_at
		FROM outbox_events
		WHERE status IN ('PENDING', 'FAILED') AND available_at <= NOW()
		ORDER BY created_at ASC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []Event
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.ID, &event.AggregateType, &event.AggregateID, &event.EventType, &event.Payload, &event.Status, &event.Attempts, &event.AvailableAt, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// ClaimPending atomically leases ready events to one worker using SKIP LOCKED.
// A crashed worker lease is recoverable after leaseFor expires.
func (r *Repository) ClaimPending(ctx context.Context, limit int, leaseFor time.Duration) ([]Event, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	if leaseFor <= 0 {
		leaseFor = 5 * time.Minute
	}
	rows, err := r.db.QueryContext(ctx, `
		WITH candidates AS (
			SELECT id FROM outbox_events
			WHERE (status IN ('PENDING','FAILED') AND available_at <= NOW())
			   OR (status = 'PROCESSING' AND locked_at <= NOW() - $2::interval)
			ORDER BY created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE outbox_events e
		SET status='PROCESSING', locked_at=NOW(), attempts=attempts+1, updated_at=NOW()
		FROM candidates c
		WHERE e.id=c.id
		RETURNING e.id,e.aggregate_type,e.aggregate_id,e.event_type,e.payload,e.status,e.attempts,e.available_at,e.created_at`, limit, leaseFor.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []Event
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.ID, &event.AggregateType, &event.AggregateID, &event.EventType, &event.Payload, &event.Status, &event.Attempts, &event.AvailableAt, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// MarkDelivered finalizes a leased event after its side effect succeeds.
func (r *Repository) MarkDelivered(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `UPDATE outbox_events SET status='DELIVERED', processed_at=NOW(), locked_at=NULL, last_error=NULL, updated_at=NOW() WHERE id=$1 AND status='PROCESSING'`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return sql.ErrNoRows
	}
	return nil
}

// MarkFailed releases a leased event for exponential-backoff retry or dead-letters it.
func (r *Repository) MarkFailed(ctx context.Context, id uuid.UUID, cause error, retryAfter time.Duration, maxAttempts int) error {
	if retryAfter < 0 {
		retryAfter = 0
	}
	if maxAttempts < 1 {
		maxAttempts = 5
	}
	message := "unknown delivery failure"
	if cause != nil {
		message = cause.Error()
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE outbox_events
		SET status = CASE WHEN attempts >= $3 THEN 'DEAD_LETTER' ELSE 'FAILED' END,
			available_at = CASE WHEN attempts >= $3 THEN available_at ELSE NOW() + $2::interval END,
			locked_at=NULL,last_error=$4,updated_at=NOW()
		WHERE id=$1 AND status='PROCESSING'`, id, retryAfter.String(), maxAttempts, message)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return sql.ErrNoRows
	}
	return nil
}
