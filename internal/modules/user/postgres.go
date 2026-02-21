package user

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type postgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository creates a new PostgreSQL user repository.
func NewPostgresRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) CreateUser(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (id, email, password_hash, first_name, last_name)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.ExecContext(ctx, query, user.ID, user.Email, user.PasswordHash, user.FirstName, user.LastName)
	return err
}

func (r *postgresRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	user := &User{}
	query := `
		SELECT id, email, password_hash, first_name, last_name, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *postgresRepository) GetUserByID(ctx context.Context, id string) (*User, error) {
	user := &User{}
	query := `
		SELECT id, email, password_hash, first_name, last_name, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	err = r.db.QueryRowContext(ctx, query, parsedID).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return user, nil
}
