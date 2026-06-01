package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	comms       comms.Service
}

// NewService creates a new auth service.
func NewService(userRepo user.Repository, userService user.Service, otpRepo otpRepository, commsService comms.Service) Service {
	return &service{
		userRepo:    userRepo,
		userService: userService,
		otpRepo:     otpRepo,
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

func (s *service) RequestOTP(ctx context.Context, req OTPRequest) (*OTPChallengeResponse, error) {
	req.Method = OTPMethod(strings.ToLower(string(req.Method)))
	req.Purpose = OTPPurpose(strings.ToLower(string(req.Purpose)))
	if req.Purpose == "" {
		req.Purpose = OTPPurposeLogin
	}
	if req.Method != OTPMethodEmail && req.Method != OTPMethodPhone {
		return nil, errors.New("method must be email or phone")
	}

	destination := strings.TrimSpace(req.Email)
	if req.Method == OTPMethodPhone {
		destination = strings.TrimSpace(req.Phone)
		if !smsOTPConfigured() {
			return nil, errors.New("phone OTP is not configured")
		}
		if req.Purpose == OTPPurposeSignup {
			return nil, errors.New("phone signup OTP is not enabled")
		}
	}
	if destination == "" {
		return nil, errors.New("destination is required")
	}
	if req.Method == OTPMethodEmail && req.Purpose == OTPPurposeSignup && req.Password == "" {
		return nil, errors.New("password is required for signup OTP")
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

	if err := s.sendOTP(ctx, req.Method, destination, code); err != nil {
		return nil, err
	}

	return &OTPChallengeResponse{
		ChallengeID:      challengeID,
		Method:           req.Method,
		Destination:      destination,
		ExpiresInSeconds: 300,
		DeliveryStatus:   "SENT",
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
		if challenge.Method == OTPMethodPhone {
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
	return os.Getenv("AFRICASTALKING_API_KEY") != "" ||
		os.Getenv("AT_API_KEY") != "" ||
		os.Getenv("TWILIO_SID") != ""
}
