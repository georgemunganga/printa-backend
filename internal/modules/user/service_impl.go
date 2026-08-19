package user

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// ErrEmailAlreadyRegistered is safe to expose to account-creation clients.
// It deliberately replaces PostgreSQL’s raw duplicate-key message.
var ErrEmailAlreadyRegistered = errors.New("an account already exists for this email")

type service struct {
	repo Repository
}

// NewService creates a new user service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) RegisterUser(ctx context.Context, email, password, firstName, lastName, role string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return nil, errors.New("email and password are required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if role == "" {
		role = "CUSTOMER"
	}
	if role != "CUSTOMER" && role != "VENDOR" {
		return nil, errors.New("public registration only supports CUSTOMER or VENDOR roles")
	}
	u := &User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(hash),
		FirstName:    firstName,
		LastName:     lastName,
		Role:         role,
		IsActive:     true,
	}
	if err := s.repo.CreateUser(ctx, u); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == "users_email_key" {
			return nil, ErrEmailAlreadyRegistered
		}
		return nil, err
	}
	return u, nil
}

func (s *service) GetUser(ctx context.Context, id string) (*User, error) {
	return s.repo.GetUserByID(ctx, id)
}

func (s *service) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	return s.repo.GetUserByEmail(ctx, email)
}

func (s *service) UpdateProfile(ctx context.Context, id string, req UpdateProfileRequest) (*User, error) {
	if strings.TrimSpace(req.FirstName) == "" || strings.TrimSpace(req.LastName) == "" {
		return nil, errors.New("first_name and last_name are required")
	}
	return s.repo.UpdateProfile(ctx, id, strings.TrimSpace(req.FirstName), strings.TrimSpace(req.LastName), strings.TrimSpace(req.Phone))
}

func (s *service) ListUsers(ctx context.Context) ([]*User, error) {
	return s.repo.ListUsers(ctx)
}
