package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type postgresRepository struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, n *Notification) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	if n.Priority == "" {
		n.Priority = PriorityNormal
	}
	n.Status = StatusUnread
	n.CreatedAt = time.Now()
	n.UpdatedAt = time.Now()

	meta, _ := json.Marshal(n.Metadata)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO notifications
			(id, recipient_id, type, title, body, status, priority, metadata, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		n.ID, n.RecipientID, string(n.Type), n.Title, n.Body,
		string(n.Status), string(n.Priority), string(meta),
		n.CreatedAt, n.UpdatedAt,
	)
	return err
}

func (r *postgresRepository) BulkCreate(ctx context.Context, notifications []*Notification) error {
	if len(notifications) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO notifications
			(id, recipient_id, type, title, body, status, priority, metadata, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, n := range notifications {
		if n.ID == "" {
			n.ID = uuid.New().String()
		}
		if n.Priority == "" {
			n.Priority = PriorityNormal
		}
		n.Status = StatusUnread
		n.CreatedAt = now
		n.UpdatedAt = now
		meta, _ := json.Marshal(n.Metadata)
		if _, err := stmt.ExecContext(ctx,
			n.ID, n.RecipientID, string(n.Type), n.Title, n.Body,
			string(n.Status), string(n.Priority), string(meta),
			n.CreatedAt, n.UpdatedAt,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *postgresRepository) GetByID(ctx context.Context, id string) (*Notification, error) {
	n := &Notification{}
	var metaBytes []byte
	var readAt sql.NullTime

	err := r.db.QueryRowContext(ctx, `
		SELECT id, recipient_id, type, title, body, status, priority, metadata, read_at, created_at, updated_at
		FROM notifications WHERE id = $1`, id).
		Scan(&n.ID, &n.RecipientID, &n.Type, &n.Title, &n.Body,
			&n.Status, &n.Priority, &metaBytes, &readAt, &n.CreatedAt, &n.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("notification not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	if readAt.Valid {
		n.ReadAt = &readAt.Time
	}
	if len(metaBytes) > 0 {
		json.Unmarshal(metaBytes, &n.Metadata)
	}
	return n, nil
}

func (r *postgresRepository) List(ctx context.Context, filter ListFilter) ([]*Notification, int, error) {
	where := []string{"recipient_id = $1"}
	args := []interface{}{filter.RecipientID}
	idx := 2

	if filter.Status != "" {
		where = append(where, fmt.Sprintf("status = $%d", idx))
		args = append(args, string(filter.Status))
		idx++
	}
	if filter.Type != "" {
		where = append(where, fmt.Sprintf("type = $%d", idx))
		args = append(args, string(filter.Type))
		idx++
	}

	whereClause := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM notifications WHERE %s", whereClause),
		args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	args = append(args, filter.Limit, filter.Offset)
	query := fmt.Sprintf(`
		SELECT id, recipient_id, type, title, body, status, priority, metadata, read_at, created_at, updated_at
		FROM notifications WHERE %s
		ORDER BY
			CASE priority WHEN 'URGENT' THEN 1 WHEN 'HIGH' THEN 2 WHEN 'NORMAL' THEN 3 ELSE 4 END,
			created_at DESC
		LIMIT $%d OFFSET $%d`, whereClause, idx, idx+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var notifications []*Notification
	for rows.Next() {
		n := &Notification{}
		var metaBytes []byte
		var readAt sql.NullTime
		if err := rows.Scan(&n.ID, &n.RecipientID, &n.Type, &n.Title, &n.Body,
			&n.Status, &n.Priority, &metaBytes, &readAt, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if readAt.Valid {
			n.ReadAt = &readAt.Time
		}
		if len(metaBytes) > 0 {
			json.Unmarshal(metaBytes, &n.Metadata)
		}
		notifications = append(notifications, n)
	}
	return notifications, total, nil
}

func (r *postgresRepository) MarkRead(ctx context.Context, id, recipientID string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE notifications
		SET status = 'READ', read_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND recipient_id = $2 AND status = 'UNREAD'`,
		id, recipientID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("notification not found or already read")
	}
	return nil
}

func (r *postgresRepository) MarkAllRead(ctx context.Context, recipientID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE notifications
		SET status = 'READ', read_at = NOW(), updated_at = NOW()
		WHERE recipient_id = $1 AND status = 'UNREAD'`,
		recipientID)
	return err
}

func (r *postgresRepository) Dismiss(ctx context.Context, id, recipientID string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE notifications
		SET status = 'DISMISSED', updated_at = NOW()
		WHERE id = $1 AND recipient_id = $2`,
		id, recipientID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("notification not found")
	}
	return nil
}

func (r *postgresRepository) Delete(ctx context.Context, id, recipientID string) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM notifications WHERE id = $1 AND recipient_id = $2", id, recipientID)
	return err
}

func (r *postgresRepository) GetUnreadCount(ctx context.Context, recipientID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM notifications WHERE recipient_id = $1 AND status = 'UNREAD'",
		recipientID).Scan(&count)
	return count, err
}
