package auth

import (
	"context"
	"database/sql"
	"time"
)

type otpRepository interface {
	Create(ctx context.Context, challenge *otpChallenge) error
	Get(ctx context.Context, id string) (*otpChallenge, error)
	IncrementAttempts(ctx context.Context, id string) error
	Consume(ctx context.Context, id string) error
}

type postgresOTPRepository struct{ db *sql.DB }

func NewPostgresOTPRepository(db *sql.DB) otpRepository {
	return &postgresOTPRepository{db: db}
}

func (r *postgresOTPRepository) Create(ctx context.Context, c *otpChallenge) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO auth_otp_challenges
		  (id, purpose, method, destination, code_hash, payload, max_attempts, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		c.ID, c.Purpose, c.Method, c.Destination, c.CodeHash, c.Payload, c.MaxAttempts, c.ExpiresAt)
	return err
}

func (r *postgresOTPRepository) Get(ctx context.Context, id string) (*otpChallenge, error) {
	var c otpChallenge
	var consumed sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, purpose, method, destination, code_hash, COALESCE(payload, '{}'::jsonb),
		       attempts, max_attempts, consumed_at, expires_at, created_at
		FROM auth_otp_challenges
		WHERE id = $1`, id).Scan(
		&c.ID, &c.Purpose, &c.Method, &c.Destination, &c.CodeHash, &c.Payload,
		&c.Attempts, &c.MaxAttempts, &consumed, &c.ExpiresAt, &c.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if consumed.Valid {
		c.ConsumedAt = &consumed.Time
	}
	return &c, nil
}

func (r *postgresOTPRepository) IncrementAttempts(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE auth_otp_challenges SET attempts = attempts + 1 WHERE id = $1`, id)
	return err
}

func (r *postgresOTPRepository) Consume(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE auth_otp_challenges SET consumed_at = $1 WHERE id = $2`, time.Now(), id)
	return err
}
