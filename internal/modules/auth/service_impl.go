package auth

import (
	"context"
	"errors"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/georgemunganga/printa-backend/internal/modules/user"
	"golang.org/x/crypto/bcrypt"
)

var jwtKey = []byte("your-secret-key")

type service struct {
	userRepo user.Repository
}

// NewService creates a new auth service.
func NewService(userRepo user.Repository) Service {
	return &service{userRepo: userRepo}
}

func (s *service) Login(ctx context.Context, email, password string) (string, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &jwt.StandardClaims{
		Subject:   user.ID.String(),
		ExpiresAt: expirationTime.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
