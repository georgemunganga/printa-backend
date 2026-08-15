package middleware

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateAndParseTokenAcceptsValidHS256Claims(t *testing.T) {
	t.Setenv("JWT_SECRET", "m2-test-secret")
	t.Setenv("JWT_EXPIRATION", "1h")

	token, err := GenerateToken("user-1", "user@example.com", RoleVendor)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if claims.UserID != "user-1" || claims.Email != "user@example.com" || claims.Role != RoleVendor {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestParseTokenRejectsUnexpectedSigningAlgorithm(t *testing.T) {
	t.Setenv("JWT_SECRET", "m2-test-secret")

	token := jwt.NewWithClaims(jwt.SigningMethodHS512, &Claims{
		UserID: "user-1",
		Email:  "user@example.com",
		Role:   RoleVendor,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	encoded, err := token.SignedString([]byte("m2-test-secret"))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	if _, err := ParseToken(encoded); err == nil {
		t.Fatal("ParseToken() accepted an HS512 token")
	}
}

func TestParseTokenRejectsInvalidRoleClaims(t *testing.T) {
	t.Setenv("JWT_SECRET", "m2-test-secret")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		UserID: "user-1",
		Email:  "user@example.com",
		Role:   Role("UNTRUSTED"),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	encoded, err := token.SignedString([]byte("m2-test-secret"))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	if _, err := ParseToken(encoded); err == nil {
		t.Fatal("ParseToken() accepted an invalid role claim")
	}
}

func TestGenerateTokenRequiresConfiguredSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")

	if _, err := GenerateToken("user-1", "user@example.com", RoleVendor); err == nil {
		t.Fatal("GenerateToken() succeeded without JWT_SECRET")
	}
}
