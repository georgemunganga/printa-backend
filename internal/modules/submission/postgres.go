package submission

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type postgresRepository struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) Repository { return &postgresRepository{db: db} }

func (r *postgresRepository) Create(ctx context.Context, input CreateInput) (*Record, error) {
	requesterID, err := uuid.Parse(input.RequesterUserID)
	if err != nil {
		return nil, fmt.Errorf("invalid requester identity")
	}

	record := &Record{}
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO submission_records (requester_user_id, requester_role, submission_kind, topic, subject, message)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, requester_user_id, requester_role, submission_kind, topic, subject, message, status, created_at, updated_at
	`, requesterID, input.RequesterRole, input.SubmissionKind, input.Topic, input.Subject, input.Message).Scan(
		&record.ID, &record.RequesterUserID, &record.RequesterRole, &record.SubmissionKind,
		&record.Topic, &record.Subject, &record.Message, &record.Status, &record.CreatedAt, &record.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *postgresRepository) ListForRequester(ctx context.Context, requesterUserID string, requesterRole RequesterRole) ([]Record, error) {
	requesterID, err := uuid.Parse(requesterUserID)
	if err != nil {
		return nil, fmt.Errorf("invalid requester identity")
	}
	return r.list(ctx, `WHERE requester_user_id = $1 AND requester_role = $2`, requesterID, requesterRole)
}

func (r *postgresRepository) ListForRole(ctx context.Context, requesterRole RequesterRole) ([]Record, error) {
	return r.list(ctx, `WHERE requester_role = $1`, requesterRole)
}

func (r *postgresRepository) list(ctx context.Context, where string, args ...any) ([]Record, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, requester_user_id, requester_role, submission_kind, topic, subject, message, status, created_at, updated_at
		FROM submission_records `+where+` ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]Record, 0)
	for rows.Next() {
		var record Record
		if err := rows.Scan(&record.ID, &record.RequesterUserID, &record.RequesterRole, &record.SubmissionKind, &record.Topic, &record.Subject, &record.Message, &record.Status, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}
