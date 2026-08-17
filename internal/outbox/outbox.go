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
