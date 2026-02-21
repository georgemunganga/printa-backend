package user

import (
	"context"

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

func (s *service) RegisterUser(ctx context.Context, email, password, firstName, lastName string) (*User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(hashedPassword),
		FirstName:    firstName,
		LastName:     lastName,
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *service) GetUser(ctx context.Context, id string) (*User, error) {
	return s.repo.GetUserByID(ctx, id)
}
