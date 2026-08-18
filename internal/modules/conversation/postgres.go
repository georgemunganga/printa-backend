package conversation

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, message *Message) error
	ListByOrder(ctx context.Context, orderID string) ([]*Message, error)
	MarkReadByOrder(ctx context.Context, orderID, readerID string) error
}

type postgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, message *Message) error {
	return r.db.QueryRowContext(ctx, `
		INSERT INTO order_messages (id, order_id, sender_id, body)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at, delivered_at, read_at`,
		message.ID, message.OrderID, message.SenderID, message.Body,
	).Scan(&message.CreatedAt, &message.DeliveredAt, &message.ReadAt)
}

func (r *postgresRepository) ListByOrder(ctx context.Context, orderID string) ([]*Message, error) {
	orderUUID, err := uuid.Parse(orderID)
	if err != nil {
		return nil, fmt.Errorf("invalid order ID: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, order_id, sender_id, body, created_at, delivered_at, read_at
		FROM order_messages
		WHERE order_id = $1
		ORDER BY created_at ASC, id ASC`, orderUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		message := &Message{}
		if err := rows.Scan(
			&message.ID, &message.OrderID, &message.SenderID, &message.Body,
			&message.CreatedAt, &message.DeliveredAt, &message.ReadAt,
		); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (r *postgresRepository) MarkReadByOrder(ctx context.Context, orderID, readerID string) error {
	orderUUID, err := uuid.Parse(orderID)
	if err != nil {
		return fmt.Errorf("invalid order ID: %w", err)
	}
	readerUUID, err := uuid.Parse(readerID)
	if err != nil {
		return fmt.Errorf("invalid reader ID: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE order_messages
		SET read_at = COALESCE(read_at, NOW())
		WHERE order_id = $1 AND sender_id <> $2`, orderUUID, readerUUID)
	return err
}
