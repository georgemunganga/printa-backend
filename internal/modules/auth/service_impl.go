package auth

import (
	"context"
	"errors"

	appMiddleware "github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/user"
	"golang.org/x/crypto/bcrypt"
)

type service struct {
	userRepo user.Repository
}

// NewService creates a new auth service.
func NewService(userRepo user.Repository) Service {
	return &service{userRepo: userRepo}
}

func (s *service) Login(ctx context.Context, email, password string) (string, error) {
	u, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", errors.New("invalid credentials")
	}
	if !u.IsActive {
		return "", errors.New("account is deactivated")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}
	token, err := appMiddleware.GenerateToken(u.ID.String(), u.Email, appMiddleware.Role(u.Role))
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *service) RefreshToken(ctx context.Context, token string) (string, error) {
	return "", errors.New("not implemented")
}
