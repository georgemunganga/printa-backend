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

func (r *postgresRepository) CreateUser(ctx context.Context, u *User) error {
	query := `
		INSERT INTO users (id, email, password_hash, first_name, last_name, role, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	role := u.Role
	if role == "" {
		role = "CUSTOMER"
	}
	_, err := r.db.ExecContext(ctx, query,
		u.ID, u.Email, u.PasswordHash, u.FirstName, u.LastName, role, true)
	return err
}

func (r *postgresRepository) CreateOAuthUser(ctx context.Context, u *User) error {
	query := `
		INSERT INTO users (id, email, password_hash, first_name, last_name, role, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	role := u.Role
	if role == "" {
		role = "CUSTOMER"
	}
	passwordHash := u.PasswordHash
	if passwordHash == "" {
		passwordHash = "oauth:google"
	}
	_, err := r.db.ExecContext(ctx, query,
		u.ID, u.Email, passwordHash, u.FirstName, u.LastName, role, true)
	return err
}

func (r *postgresRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	u := &User{}
	var phone sql.NullString
	query := `
		SELECT id, email, password_hash, first_name, last_name,
		       COALESCE(role::text, 'CUSTOMER'), is_active, phone, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	err := r.db.QueryRowContext(ctx, query, email).Scan(
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

func (r *postgresRepository) GetUserByPhone(ctx context.Context, phoneNumber string) (*User, error) {
	u := &User{}
	var phone sql.NullString
	query := `
		SELECT id, email, password_hash, first_name, last_name,
		       COALESCE(role::text, 'CUSTOMER'), is_active, phone, created_at, updated_at
		FROM users
		WHERE phone = $1
	`
	err := r.db.QueryRowContext(ctx, query, phoneNumber).Scan(
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

func (r *postgresRepository) GetUserByID(ctx context.Context, id string) (*User, error) {
	u := &User{}
	var phone sql.NullString
	query := `
		SELECT id, email, password_hash, first_name, last_name,
		       COALESCE(role::text, 'CUSTOMER'), is_active, phone, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	err = r.db.QueryRowContext(ctx, query, parsedID).Scan(
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

func (r *postgresRepository) UpdateProfile(ctx context.Context, id string, firstName, lastName, phone string) (*User, error) {
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE users
		SET first_name = $2, last_name = $3, phone = NULLIF($4, ''), updated_at = NOW()
		WHERE id = $1`, parsedID, firstName, lastName, phone)
	if err != nil {
		return nil, err
	}
	return r.GetUserByID(ctx, id)
}

func (r *postgresRepository) ListUsers(ctx context.Context) ([]*User, error) {
	query := `
		SELECT id, email, first_name, last_name,
		       COALESCE(role::text, 'CUSTOMER'), is_active, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(
			&u.ID, &u.Email, &u.FirstName, &u.LastName,
			&u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *postgresRepository) UpdateUserRole(ctx context.Context, id, role string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET role = $1::user_role, updated_at = NOW() WHERE id = $2`,
		role, id)
	return err
}

func (r *postgresRepository) PromoteUserRole(ctx context.Context, id, role string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET role = $1::user_role, updated_at = NOW()
		WHERE id = $2 AND role = 'CUSTOMER'::user_role`,
		role, id)
	return err
}

func (r *postgresRepository) DeactivateUser(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET is_active = FALSE, updated_at = NOW() WHERE id = $1`, id)
	return err
}
