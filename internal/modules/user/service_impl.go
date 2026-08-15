package user

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type service struct {
	repo Repository
}

// NewService creates a new user service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) RegisterUser(ctx context.Context, email, password, firstName, lastName, role string) (*User, error) {
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
		return nil, err
	}
	return u, nil
}

func (s *service) GetUser(ctx context.Context, id string) (*User, error) {
	return s.repo.GetUserByID(ctx, id)
}

func (s *service) ListUsers(ctx context.Context) ([]*User, error) {
	return s.repo.ListUsers(ctx)
}
