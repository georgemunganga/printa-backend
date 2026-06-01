package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/georgemunganga/printa-backend/internal/modules/user"
)

type oauthRepository interface {
	CreateState(ctx context.Context, state, redirectURI string, expiresAt time.Time) error
	ConsumeState(ctx context.Context, state string) (string, error)
	GetUserByIdentity(ctx context.Context, provider, providerSub string) (*user.User, error)
	LinkIdentity(ctx context.Context, provider, providerSub, email, userID string) error
}

type postgresOAuthRepository struct{ db *sql.DB }

func NewPostgresOAuthRepository(db *sql.DB) oauthRepository {
	return &postgresOAuthRepository{db: db}
}

func (r *postgresOAuthRepository) CreateState(ctx context.Context, state, redirectURI string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO auth_oauth_states (state, redirect_uri, expires_at)
		VALUES ($1, $2, $3)`, state, redirectURI, expiresAt)
	return err
}

func (r *postgresOAuthRepository) ConsumeState(ctx context.Context, state string) (string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var redirectURI string
	var expiresAt time.Time
	var consumed sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT redirect_uri, expires_at, consumed_at
		FROM auth_oauth_states
		WHERE state = $1
		FOR UPDATE`, state).Scan(&redirectURI, &expiresAt, &consumed)
	if err != nil {
		return "", err
	}
	if consumed.Valid {
		return "", errors.New("OAuth state already used")
	}
	if time.Now().After(expiresAt) {
		return "", errors.New("OAuth state expired")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE auth_oauth_states SET consumed_at = NOW() WHERE state = $1`, state); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return redirectURI, nil
}

func (r *postgresOAuthRepository) GetUserByIdentity(ctx context.Context, provider, providerSub string) (*user.User, error) {
	u := &user.User{}
	var phone sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.password_hash, u.first_name, u.last_name,
		       COALESCE(u.role::text, 'CUSTOMER'), u.is_active, u.phone, u.created_at, u.updated_at
		FROM auth_oauth_identities oi
		JOIN users u ON u.id = oi.user_id
		WHERE oi.provider = $1 AND oi.provider_sub = $2`, provider, providerSub).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName,
		&u.Role, &u.IsActive, &phone, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if phone.Valid {
		u.Phone = phone.String
	}
	return u, nil
}

func (r *postgresOAuthRepository) LinkIdentity(ctx context.Context, provider, providerSub, email, userID string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO auth_oauth_identities (provider, provider_sub, email, user_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (provider, provider_sub)
		DO UPDATE SET email = EXCLUDED.email, updated_at = NOW()`,
		provider, providerSub, email, userID)
	return err
}
