package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/georgemunganga/printa-backend/internal/modules/user"
)

type authServiceStub struct {
	requestOTPErr error
}

func (s authServiceStub) Login(context.Context, string, string) (string, error) {
	return "", errors.New("not implemented")
}
func (s authServiceStub) RequestOTP(context.Context, OTPRequest) (*OTPChallengeResponse, error) {
	return nil, s.requestOTPErr
}
func (s authServiceStub) VerifyOTP(context.Context, OTPVerifyRequest) (*OTPVerifyResponse, error) {
	return nil, errors.New("not implemented")
}
func (s authServiceStub) GoogleAuthURL(context.Context, OAuthStartRequest) (string, error) {
	return "", errors.New("not implemented")
}
func (s authServiceStub) HandleGoogleCallback(context.Context, string, string) (*OAuthCallbackResponse, error) {
	return nil, errors.New("not implemented")
}

func TestRequestOTPReturnsConflictForExistingAccount(t *testing.T) {
	h := NewHandler(authServiceStub{requestOTPErr: user.ErrEmailAlreadyRegistered})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/request", strings.NewReader(`{"purpose":"signup","method":"email","email":"existing@example.com"}`))
	rec := httptest.NewRecorder()

	h.requestOTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != "ACCOUNT_EXISTS" {
		t.Fatalf("code = %q, want ACCOUNT_EXISTS", body["code"])
	}
	if strings.Contains(strings.ToLower(body["error"]), "pq:") {
		t.Fatalf("response leaked database detail: %q", body["error"])
	}
}
