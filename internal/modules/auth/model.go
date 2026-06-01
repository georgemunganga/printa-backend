package auth

import "time"

type OTPPurpose string

const (
	OTPPurposeLogin  OTPPurpose = "login"
	OTPPurposeSignup OTPPurpose = "signup"
)

type OTPMethod string

const (
	OTPMethodEmail OTPMethod = "email"
	OTPMethodPhone OTPMethod = "phone"
)

type OTPRequest struct {
	Purpose   OTPPurpose `json:"purpose"`
	Method    OTPMethod  `json:"method"`
	Email     string     `json:"email,omitempty"`
	Phone     string     `json:"phone,omitempty"`
	UserID    string     `json:"user_id,omitempty"`
	Password  string     `json:"password,omitempty"`
	FirstName string     `json:"first_name,omitempty"`
	LastName  string     `json:"last_name,omitempty"`
	Role      string     `json:"role,omitempty"`
}

type OTPVerifyRequest struct {
	ChallengeID string `json:"challenge_id"`
	Code        string `json:"code"`
}

type OTPChallengeResponse struct {
	ChallengeID      string        `json:"challenge_id"`
	Method           OTPMethod     `json:"method"`
	Destination      string        `json:"destination"`
	ExpiresInSeconds int           `json:"expires_in_seconds"`
	DeliveryStatus   string        `json:"delivery_status"`
	Deliveries       []OTPDelivery `json:"deliveries,omitempty"`
}

type OTPDelivery struct {
	Method      OTPMethod `json:"method"`
	Destination string    `json:"destination"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
}

type OTPVerifyResponse struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type"`
}

type OAuthCallbackResponse struct {
	Token       string `json:"token"`
	TokenType   string `json:"token_type"`
	RedirectURI string `json:"redirect_uri"`
}

type OAuthStartRequest struct {
	RedirectURI string `json:"redirect_uri"`
	Role        string `json:"role,omitempty"`
	Mode        string `json:"mode,omitempty"`
}

type oauthState struct {
	RedirectURI string
	Role        string
	Mode        string
}

type otpChallenge struct {
	ID          string
	Purpose     OTPPurpose
	Method      OTPMethod
	Destination string
	CodeHash    string
	Payload     []byte
	Attempts    int
	MaxAttempts int
	ConsumedAt  *time.Time
	ExpiresAt   time.Time
	CreatedAt   time.Time
}
