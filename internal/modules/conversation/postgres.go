package conversation

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type Repository interface {
	Create(ctx context.Context, message *Message, assetIDs []uuid.UUID) error
	ListByOrder(ctx context.Context, orderID string) ([]*Message, error)
	MarkReadByOrder(ctx context.Context, orderID, readerID string) error
	GetAttachment(ctx context.Context, orderID, messageID, assetID string) (*Attachment, error)
}

type postgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, message *Message, assetIDs []uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if len(assetIDs) > 0 {
		var ownedCount int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM design_assets
			WHERE id = ANY($1) AND owner_id = $2 AND deleted_at IS NULL`,
			pq.Array(assetIDs), message.SenderID,
		).Scan(&ownedCount); err != nil {
			return err
		}
		if ownedCount != len(assetIDs) {
			return fmt.Errorf("every attachment must be an available asset owned by the message sender")
		}
	}

	if err := tx.QueryRowContext(ctx, `
		INSERT INTO order_messages (id, order_id, sender_id, body)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at, delivered_at, read_at`,
		message.ID, message.OrderID, message.SenderID, message.Body,
	).Scan(&message.CreatedAt, &message.DeliveredAt, &message.ReadAt); err != nil {
		return err
	}

	for _, assetID := range assetIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO order_message_attachments (message_id, asset_id)
			VALUES ($1, $2)`, message.ID, assetID); err != nil {
			return err
		}
	}
	return tx.Commit()
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
	messageIDs := make([]uuid.UUID, 0)
	byID := make(map[uuid.UUID]*Message)
	for rows.Next() {
		message := &Message{}
		if err := rows.Scan(
			&message.ID, &message.OrderID, &message.SenderID, &message.Body,
			&message.CreatedAt, &message.DeliveredAt, &message.ReadAt,
		); err != nil {
			return nil, err
		}
		messages = append(messages, message)
		messageIDs = append(messageIDs, message.ID)
		byID[message.ID] = message
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(messageIDs) == 0 {
		return messages, nil
	}

	attachments, err := r.db.QueryContext(ctx, `
		SELECT ma.message_id, da.id, da.owner_id, da.original_name, da.content_type, da.size_bytes
		FROM order_message_attachments ma
		JOIN design_assets da ON da.id = ma.asset_id AND da.deleted_at IS NULL
		WHERE ma.message_id = ANY($1)
		ORDER BY ma.created_at ASC`, pq.Array(messageIDs))
	if err != nil {
		return nil, err
	}
	defer attachments.Close()
	for attachments.Next() {
		var messageID uuid.UUID
		attachment := &Attachment{}
		if err := attachments.Scan(&messageID, &attachment.AssetID, &attachment.OwnerID, &attachment.Name, &attachment.ContentType, &attachment.SizeBytes); err != nil {
			return nil, err
		}
		if message, ok := byID[messageID]; ok {
			message.Attachments = append(message.Attachments, attachment)
		}
	}
	return messages, attachments.Err()
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

func (r *postgresRepository) GetAttachment(ctx context.Context, orderID, messageID, assetID string) (*Attachment, error) {
	orderUUID, err := uuid.Parse(orderID)
	if err != nil {
		return nil, fmt.Errorf("invalid order ID: %w", err)
	}
	messageUUID, err := uuid.Parse(messageID)
	if err != nil {
		return nil, fmt.Errorf("invalid message ID: %w", err)
	}
	assetUUID, err := uuid.Parse(assetID)
	if err != nil {
		return nil, fmt.Errorf("invalid asset ID: %w", err)
	}
	attachment := &Attachment{}
	err = r.db.QueryRowContext(ctx, `
		SELECT da.id, da.owner_id, da.original_name, da.content_type, da.size_bytes
		FROM order_message_attachments ma
		JOIN order_messages m ON m.id = ma.message_id
		JOIN design_assets da ON da.id = ma.asset_id AND da.deleted_at IS NULL
		WHERE m.order_id = $1 AND ma.message_id = $2 AND ma.asset_id = $3`,
		orderUUID, messageUUID, assetUUID,
	).Scan(&attachment.AssetID, &attachment.OwnerID, &attachment.Name, &attachment.ContentType, &attachment.SizeBytes)
	if err != nil {
		return nil, err
	}
	return attachment, nil
}
