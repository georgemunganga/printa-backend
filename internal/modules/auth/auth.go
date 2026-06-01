package auth

import "context"

// Service defines the interface for authentication-related business logic.
type Service interface {
	Login(ctx context.Context, email, password string) (string, error)
	RequestOTP(ctx context.Context, req OTPRequest) (*OTPChallengeResponse, error)
	VerifyOTP(ctx context.Context, req OTPVerifyRequest) (*OTPVerifyResponse, error)
	GoogleAuthURL(ctx context.Context, redirectURI string) (string, error)
	HandleGoogleCallback(ctx context.Context, code, state string) (*OAuthCallbackResponse, error)
}
