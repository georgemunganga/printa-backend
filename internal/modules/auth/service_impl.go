package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	appMiddleware "github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/comms"
	"github.com/georgemunganga/printa-backend/internal/modules/user"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type service struct {
	userRepo    user.Repository
	userService user.Service
	otpRepo     otpRepository
	oauthRepo   oauthRepository
	comms       comms.Service
}

// NewService creates a new auth service.
func NewService(userRepo user.Repository, userService user.Service, otpRepo otpRepository, oauthRepo oauthRepository, commsService comms.Service) Service {
	return &service{
		userRepo:    userRepo,
		userService: userService,
		otpRepo:     otpRepo,
		oauthRepo:   oauthRepo,
		comms:       commsService,
	}
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

func (s *service) GoogleAuthURL(ctx context.Context, req OAuthStartRequest) (string, error) {
	if s.oauthRepo == nil {
		return "", errors.New("OAuth repository is not configured")
	}
	clientID := strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_ID"))
	if clientID == "" {
		return "", errors.New("Google OAuth is not configured")
	}
	callbackURL := googleCallbackURL()
	frontendRedirect, err := resolveOAuthRedirectURI(req.RedirectURI)
	if err != nil {
		return "", err
	}
	role, err := normalizeOAuthRole(req.Role, frontendRedirect)
	if err != nil {
		return "", err
	}
	state, err := randomToken(32)
	if err != nil {
		return "", err
	}
	if err := s.oauthRepo.CreateState(ctx, state, oauthState{RedirectURI: frontendRedirect, Role: role}, time.Now().Add(10*time.Minute)); err != nil {
		return "", err
	}

	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("redirect_uri", callbackURL)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	q.Set("access_type", "offline")
	q.Set("prompt", "select_account")
	return "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode(), nil
}

func (s *service) HandleGoogleCallback(ctx context.Context, code, state string) (*OAuthCallbackResponse, error) {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(state) == "" {
		return nil, errors.New("code and state are required")
	}
	oauthState, err := s.oauthRepo.ConsumeState(ctx, state)
	if err != nil {
		return nil, err
	}

	accessToken, err := exchangeGoogleCode(ctx, code)
	if err != nil {
		return nil, err
	}
	profile, err := fetchGoogleProfile(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	if profile.Sub == "" || profile.Email == "" {
		return nil, errors.New("Google profile is missing required identity fields")
	}
	if !profile.EmailVerified {
		return nil, errors.New("Google email is not verified")
	}

	u, err := s.oauthRepo.GetUserByIdentity(ctx, "google", profile.Sub)
	if errors.Is(err, sql.ErrNoRows) {
		u, err = s.userRepo.GetUserByEmail(ctx, profile.Email)
		if errors.Is(err, sql.ErrNoRows) {
			u = &user.User{
				ID:        uuid.New(),
				Email:     profile.Email,
				FirstName: profile.GivenName,
				LastName:  profile.FamilyName,
				Role:      oauthState.Role,
				IsActive:  true,
			}
			err = s.userRepo.CreateOAuthUser(ctx, u)
		}
		if err == nil {
			u, err = s.applyOAuthRoleIntent(ctx, u, oauthState.Role)
		}
		if err == nil {
			err = s.oauthRepo.LinkIdentity(ctx, "google", profile.Sub, profile.Email, u.ID.String())
		}
	}
	if err == nil {
		u, err = s.applyOAuthRoleIntent(ctx, u, oauthState.Role)
	}
	if err != nil {
		return nil, err
	}
	if !u.IsActive {
		return nil, errors.New("account is deactivated")
	}

	token, err := appMiddleware.GenerateToken(u.ID.String(), u.Email, appMiddleware.Role(u.Role))
	if err != nil {
		return nil, err
	}
	return &OAuthCallbackResponse{Token: token, TokenType: "Bearer", RedirectURI: oauthState.RedirectURI}, nil
}

func (s *service) applyOAuthRoleIntent(ctx context.Context, u *user.User, role string) (*user.User, error) {
	if role == "" || strings.EqualFold(u.Role, role) {
		return u, nil
	}
	if role != string(appMiddleware.RoleVendor) {
		return u, nil
	}
	if u.Role != string(appMiddleware.RoleCustomer) {
		return u, nil
	}
	if err := s.userRepo.PromoteUserRole(ctx, u.ID.String(), role); err != nil {
		return nil, err
	}
	u.Role = role
	return u, nil
}

func (s *service) RequestOTP(ctx context.Context, req OTPRequest) (*OTPChallengeResponse, error) {
	req.Method = OTPMethod(strings.ToLower(string(req.Method)))
	req.Purpose = OTPPurpose(strings.ToLower(string(req.Purpose)))
	if req.Purpose == "" {
		req.Purpose = OTPPurposeLogin
	}
	if req.Purpose != OTPPurposeLogin && req.Purpose != OTPPurposeSignup {
		return nil, errors.New("purpose must be login or signup")
	}
	if req.Method != "" && req.Method != OTPMethodEmail && req.Method != OTPMethodPhone {
		return nil, errors.New("method must be email or phone")
	}

	destination := ""
	var deliveries []otpDeliveryTarget
	if req.Purpose == OTPPurposeLogin {
		u, err := s.resolveOTPLoginUser(ctx, req)
		if err != nil {
			return nil, err
		}
		if !u.IsActive {
			return nil, errors.New("account is deactivated")
		}
		req.UserID = u.ID.String()
		deliveries = loginOTPDeliveryTargets(u)
		if len(deliveries) == 0 {
			return nil, errors.New("no OTP delivery channel is available for this account")
		}
		req.Method = deliveries[0].Method
		destination = deliveries[0].Destination
	} else {
		if req.Method == "" {
			req.Method = OTPMethodEmail
		}
		destination = strings.TrimSpace(req.Email)
		if req.Method == OTPMethodPhone {
			destination = strings.TrimSpace(req.Phone)
			if !smsOTPConfigured() {
				return nil, errors.New("phone OTP is not configured")
			}
			return nil, errors.New("phone signup OTP is not enabled")
		}
		if destination == "" {
			return nil, errors.New("destination is required")
		}
		if req.Method == OTPMethodEmail && req.Password == "" {
			return nil, errors.New("password is required for signup OTP")
		}
		deliveries = []otpDeliveryTarget{{Method: req.Method, Destination: destination}}
	}

	code, err := generateOTPCode()
	if err != nil {
		return nil, err
	}

	challengeID := uuid.New().String()
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	challenge := &otpChallenge{
		ID:          challengeID,
		Purpose:     req.Purpose,
		Method:      req.Method,
		Destination: destination,
		CodeHash:    hashOTP(challengeID, code),
		Payload:     payload,
		MaxAttempts: 5,
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	}
	if err := s.otpRepo.Create(ctx, challenge); err != nil {
		return nil, err
	}

	deliveryResults := s.sendOTPToTargets(ctx, deliveries, code)
	if !otpDelivered(deliveryResults) {
		return nil, otpDeliveryError(deliveryResults)
	}

	status := "SENT"
	if otpPartiallyDelivered(deliveryResults) {
		status = "PARTIAL"
	}
	if req.Purpose == OTPPurposeSignup {
		status = deliveryResults[0].Status
	}

	return &OTPChallengeResponse{
		ChallengeID:      challengeID,
		Method:           req.Method,
		Destination:      destination,
		ExpiresInSeconds: 300,
		DeliveryStatus:   status,
		Deliveries:       deliveryResults,
	}, nil
}

func (s *service) VerifyOTP(ctx context.Context, req OTPVerifyRequest) (*OTPVerifyResponse, error) {
	challenge, err := s.otpRepo.Get(ctx, req.ChallengeID)
	if err != nil {
		return nil, errors.New("invalid OTP challenge")
	}
	if challenge.ConsumedAt != nil {
		return nil, errors.New("OTP already used")
	}
	if time.Now().After(challenge.ExpiresAt) {
		return nil, errors.New("OTP expired")
	}
	if challenge.Attempts >= challenge.MaxAttempts {
		return nil, errors.New("too many OTP attempts")
	}
	if hashOTP(challenge.ID, req.Code) != challenge.CodeHash {
		_ = s.otpRepo.IncrementAttempts(ctx, challenge.ID)
		return nil, errors.New("invalid OTP code")
	}

	var payload OTPRequest
	if err := json.Unmarshal(challenge.Payload, &payload); err != nil {
		return nil, err
	}

	var u *user.User
	switch challenge.Purpose {
	case OTPPurposeSignup:
		u, err = s.userService.RegisterUser(ctx, payload.Email, payload.Password, payload.FirstName, payload.LastName, payload.Role)
	case OTPPurposeLogin:
		if payload.UserID != "" {
			u, err = s.userRepo.GetUserByID(ctx, payload.UserID)
		} else if challenge.Method == OTPMethodPhone {
			u, err = s.userRepo.GetUserByPhone(ctx, challenge.Destination)
		} else {
			u, err = s.userRepo.GetUserByEmail(ctx, challenge.Destination)
		}
	default:
		err = errors.New("unsupported OTP purpose")
	}
	if err != nil {
		return nil, err
	}

	token, err := appMiddleware.GenerateToken(u.ID.String(), u.Email, appMiddleware.Role(u.Role))
	if err != nil {
		return nil, err
	}
	if err := s.otpRepo.Consume(ctx, challenge.ID); err != nil {
		return nil, err
	}
	return &OTPVerifyResponse{Token: token, TokenType: "Bearer"}, nil
}

type otpDeliveryTarget struct {
	Method      OTPMethod
	Destination string
}

func (s *service) resolveOTPLoginUser(ctx context.Context, req OTPRequest) (*user.User, error) {
	email := strings.TrimSpace(req.Email)
	phone := strings.TrimSpace(req.Phone)
	if req.Method == OTPMethodEmail && email == "" {
		return nil, errors.New("email is required")
	}
	if req.Method == OTPMethodPhone && phone == "" {
		return nil, errors.New("phone is required")
	}
	if email != "" {
		u, err := s.userRepo.GetUserByEmail(ctx, email)
		if err == nil {
			return u, nil
		}
		if phone == "" || !errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
	}
	if phone != "" {
		u, err := s.userRepo.GetUserByPhone(ctx, phone)
		if err == nil {
			return u, nil
		}
		return nil, errors.New("user not found")
	}
	return nil, errors.New("email or phone is required")
}

func loginOTPDeliveryTargets(u *user.User) []otpDeliveryTarget {
	var targets []otpDeliveryTarget
	if strings.TrimSpace(u.Email) != "" {
		targets = append(targets, otpDeliveryTarget{Method: OTPMethodEmail, Destination: strings.TrimSpace(u.Email)})
	}
	if strings.TrimSpace(u.Phone) != "" && smsOTPConfigured() {
		targets = append(targets, otpDeliveryTarget{Method: OTPMethodPhone, Destination: strings.TrimSpace(u.Phone)})
	}
	return targets
}

func (s *service) sendOTPToTargets(ctx context.Context, targets []otpDeliveryTarget, code string) []OTPDelivery {
	results := make([]OTPDelivery, 0, len(targets))
	for _, target := range targets {
		result := OTPDelivery{Method: target.Method, Destination: target.Destination, Status: "SENT"}
		if err := s.sendOTP(ctx, target.Method, target.Destination, code); err != nil {
			result.Status = "FAILED"
			result.Error = err.Error()
		}
		results = append(results, result)
	}
	return results
}

func otpDelivered(deliveries []OTPDelivery) bool {
	for _, delivery := range deliveries {
		if delivery.Status == "SENT" {
			return true
		}
	}
	return false
}

func otpPartiallyDelivered(deliveries []OTPDelivery) bool {
	hasSent := false
	hasFailed := false
	for _, delivery := range deliveries {
		hasSent = hasSent || delivery.Status == "SENT"
		hasFailed = hasFailed || delivery.Status == "FAILED"
	}
	return hasSent && hasFailed
}

func otpDeliveryError(deliveries []OTPDelivery) error {
	if len(deliveries) == 0 {
		return errors.New("no OTP delivery channel is available")
	}
	for _, delivery := range deliveries {
		if delivery.Error != "" {
			return errors.New(delivery.Error)
		}
	}
	return errors.New("OTP delivery failed")
}

func (s *service) sendOTP(ctx context.Context, method OTPMethod, destination, code string) error {
	if s.comms == nil {
		return errors.New("communications service is not configured")
	}
	channel := comms.ChannelEmail
	subject := "Your Printa verification code"
	body := fmt.Sprintf("Your Printa verification code is %s. It expires in 5 minutes.", code)
	if method == OTPMethodPhone {
		channel = comms.ChannelSMS
		subject = ""
	}
	result, err := s.comms.Send(ctx, comms.SendRequest{
		Channel:        channel,
		Recipient:      destination,
		Subject:        subject,
		Body:           body,
		IdempotencyKey: fmt.Sprintf("otp:%s:%s:%d", method, destination, time.Now().UnixNano()),
	})
	if err != nil {
		return err
	}
	if result.Status == comms.DeliveryFailed {
		return errors.New(result.Error)
	}
	return nil
}

func generateOTPCode() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	n := int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
	if n < 0 {
		n = -n
	}
	return fmt.Sprintf("%06d", n%1000000), nil
}

func hashOTP(challengeID, code string) string {
	sum := sha256.Sum256([]byte(challengeID + ":" + strings.TrimSpace(code)))
	return hex.EncodeToString(sum[:])
}

func smsOTPConfigured() bool {
	hasAfricasTalkingKey := os.Getenv("AFRICASTALKING_API_KEY") != "" || os.Getenv("AT_API_KEY") != ""
	hasAfricasTalkingUsername := os.Getenv("AFRICASTALKING_USERNAME") != "" || os.Getenv("AT_USERNAME") != ""
	return (hasAfricasTalkingKey && hasAfricasTalkingUsername) || os.Getenv("TWILIO_SID") != ""
}

type googleTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	IDToken     string `json:"id_token"`
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

type googleProfile struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Name          string `json:"name"`
}

func exchangeGoogleCode(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_ID")))
	form.Set("client_secret", strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")))
	form.Set("redirect_uri", googleCallbackURL())
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("google token exchange failed: %s", string(body))
	}
	var parsed googleTokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if parsed.AccessToken == "" {
		if parsed.Error != "" {
			return "", fmt.Errorf("google token exchange failed: %s", parsed.Description)
		}
		return "", errors.New("google token exchange returned no access token")
	}
	return parsed.AccessToken, nil
}

func fetchGoogleProfile(ctx context.Context, accessToken string) (*googleProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("google userinfo failed: %s", string(body))
	}
	var profile googleProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func googleCallbackURL() string {
	if v := strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_REDIRECT_URL")); v != "" {
		return v
	}
	return "https://api.printa.co.zm/api/v1/auth/google/callback"
}

func resolveOAuthRedirectURI(requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		requested = strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_FRONTEND_REDIRECT_URL"))
	}
	if requested == "" {
		return "", errors.New("frontend redirect URI is not configured")
	}
	allowed := splitCSV(os.Getenv("GOOGLE_OAUTH_ALLOWED_REDIRECTS"))
	if len(allowed) == 0 {
		return requested, nil
	}
	for _, candidate := range allowed {
		if requested == candidate {
			return requested, nil
		}
	}
	return "", errors.New("frontend redirect URI is not allowed")
}

func normalizeOAuthRole(role, redirectURI string) (string, error) {
	role = strings.ToUpper(strings.TrimSpace(role))
	if role == "" {
		return string(appMiddleware.RoleCustomer), nil
	}
	switch appMiddleware.Role(role) {
	case appMiddleware.RoleCustomer:
		return role, nil
	case appMiddleware.RoleVendor:
		if isVendorOAuthRedirect(redirectURI) {
			return role, nil
		}
		return "", errors.New("OAuth role is not allowed for redirect URI")
	default:
		return "", errors.New("unsupported OAuth role")
	}
}

func isVendorOAuthRedirect(redirectURI string) bool {
	allowed := splitCSV(os.Getenv("GOOGLE_OAUTH_VENDOR_REDIRECTS"))
	if len(allowed) == 0 {
		allowed = []string{
			"https://vendor.printa.co.zm/auth/google/callback",
			"http://localhost:5173/auth/google/callback",
			"http://127.0.0.1:5173/auth/google/callback",
		}
	}
	for _, candidate := range allowed {
		if redirectURI == candidate {
			return true
		}
	}
	return false
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func randomToken(byteCount int) (string, error) {
	b := make([]byte, byteCount)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
