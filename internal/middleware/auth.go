package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Role represents a user role in the system.
type Role string

const (
	RoleAdmin    Role = "ADMIN"
	RoleVendor   Role = "VENDOR"
	RoleStaff    Role = "STAFF"
	RoleCashier  Role = "CASHIER"
	RoleCustomer Role = "CUSTOMER"
)

// contextKey is an unexported type for context keys in this package.
type contextKey string

const (
	ContextKeyUserID contextKey = "user_id"
	ContextKeyEmail  contextKey = "email"
	ContextKeyRole   contextKey = "role"
)

// Claims defines the JWT payload structure.
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   Role   `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken creates a signed JWT for the given user.
func GenerateToken(userID, email string, role Role) (string, error) {
	secret, err := jwtSecret()
	if err != nil {
		return "", err
	}
	if userID == "" || email == "" || !isValidRole(role) {
		return "", errors.New("valid user ID, email, and role are required to generate a token")
	}
	expStr := os.Getenv("JWT_EXPIRATION")
	if expStr == "" {
		expStr = "24h"
	}
	dur, err := time.ParseDuration(expStr)
	if err != nil {
		dur = 24 * time.Hour
	}

	claims := &Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(dur)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// ParseToken validates a JWT string and returns the claims.
func ParseToken(tokenStr string) (*Claims, error) {
	secret, err := jwtSecret()
	if err != nil {
		return nil, err
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, jwt.ErrSignatureInvalid
		}
		return secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.UserID == "" || claims.Email == "" || !isValidRole(claims.Role) {
		return nil, errors.New("token has invalid claims")
	}
	return claims, nil
}

func jwtSecret() ([]byte, error) {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		return nil, errors.New("JWT_SECRET must be configured")
	}
	return []byte(secret), nil
}

func isValidRole(role Role) bool {
	switch role {
	case RoleAdmin, RoleVendor, RoleStaff, RoleCashier, RoleCustomer:
		return true
	default:
		return false
	}
}

// Authenticate is a chi-compatible middleware that validates the Bearer token.
func Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondErr(w, http.StatusUnauthorized, "missing Authorization header")
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			respondErr(w, http.StatusUnauthorized, "invalid Authorization header format")
			return
		}
		claims, err := ParseToken(parts[1])
		if err != nil {
			respondErr(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		// Inject claims into context
		ctx := context.WithValue(r.Context(), ContextKeyUserID, claims.UserID)
		ctx = context.WithValue(ctx, ContextKeyEmail, claims.Email)
		ctx = context.WithValue(ctx, ContextKeyRole, string(claims.Role))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole returns a middleware that enforces one of the allowed roles.
func RequireRole(roles ...Role) func(http.Handler) http.Handler {
	allowed := make(map[Role]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roleVal, _ := r.Context().Value(ContextKeyRole).(string)
			if _, ok := allowed[Role(roleVal)]; !ok {
				respondErr(w, http.StatusForbidden, "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GetUserID extracts the user ID from the request context.
func GetUserID(r *http.Request) string {
	v, _ := r.Context().Value(ContextKeyUserID).(string)
	return v
}

// GetRole extracts the role from the request context.
func GetRole(r *http.Request) Role {
	v, _ := r.Context().Value(ContextKeyRole).(string)
	return Role(v)
}

func respondErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
