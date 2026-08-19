package attendance

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/georgemunganga/printa-backend/internal/modules/comms"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const pinResetLifetime = 15 * time.Minute

var (
	ErrPINResetInvalid  = errors.New("staff PIN reset link is invalid, expired, or has already been used")
	ErrPINResetDelivery = errors.New("unable to send the staff PIN reset email")
)

// PINResetMailer is deliberately narrow so reset links are delivered only through
// the established communications service and retain its delivery audit trail.
type PINResetMailer interface {
	Send(ctx context.Context, req comms.SendRequest) (*comms.SendResult, error)
}

type PINResetRecord struct {
	ID        uuid.UUID
	StoreID   uuid.UUID
	OwnerID   uuid.UUID
	TokenHash string
	ExpiresAt time.Time
}

type ownerContact struct {
	StoreID uuid.UUID
	OwnerID uuid.UUID
	Email   string
}

// WithPINResetMailer configures optional owner PIN reset delivery. PIN setup and
// attendance continue to work when delivery is not configured; reset requests do not.
type ServiceOption func(*service)

func WithPINResetMailer(mailer PINResetMailer, portalURL string) ServiceOption {
	return func(s *service) {
		s.resetMailer = mailer
		s.portalURL = strings.TrimRight(strings.TrimSpace(portalURL), "/")
	}
}

func (s *service) RequestOwnerPINReset(ctx context.Context, storeID string) error {
	if s.resetMailer == nil {
		return errors.New("staff PIN reset email delivery is not configured")
	}
	owner, err := s.repo.GetStoreOwnerContact(ctx, storeID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(owner.Email) == "" {
		return errors.New("the store owner does not have an email address for PIN reset")
	}

	rawToken, tokenHash, err := newPINResetToken()
	if err != nil {
		return err
	}
	record := &PINResetRecord{
		ID:        uuid.New(),
		StoreID:   owner.StoreID,
		OwnerID:   owner.OwnerID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().UTC().Add(pinResetLifetime),
	}
	if err := s.repo.CreatePINReset(ctx, record); err != nil {
		return fmt.Errorf("create staff PIN reset: %w", err)
	}

	portalURL := s.portalURL
	if portalURL == "" {
		portalURL = "https://vendor.printa.co.zm"
	}
	resetURL := portalURL + "/staff-pin/reset?token=" + rawToken
	result, err := s.resetMailer.Send(ctx, comms.SendRequest{
		Channel:        comms.ChannelEmail,
		Recipient:      owner.Email,
		RecipientID:    owner.OwnerID.String(),
		Subject:        "Reset your Printa staff PIN",
		Body:           "A request was made to reset the staff PIN for your store. Open this secure link within 15 minutes: " + resetURL + " If you did not request this, you can ignore this email.",
		HTMLBody:       "<p>A request was made to reset the staff PIN for your Printa store.</p><p><a href=\"" + resetURL + "\">Reset staff PIN</a></p><p>This link expires in 15 minutes and can be used once. If you did not request this, you can ignore this email.</p>",
		IdempotencyKey: "store-staff-pin-reset:" + record.ID.String(),
	})
	if err != nil || result == nil || result.Status == comms.DeliveryFailed {
		return ErrPINResetDelivery
	}
	return nil
}

func (s *service) ConfirmOwnerPINReset(ctx context.Context, token, pin string) error {
	if !isValidPIN(pin) {
		return errors.New("PIN must contain 4 to 6 digits")
	}
	tokenHash := hashPINResetToken(token)
	if tokenHash == "" {
		return ErrPINResetInvalid
	}
	pinHash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash PIN: %w", err)
	}
	if err := s.repo.ConsumePINResetAndSetOwnerPIN(ctx, tokenHash, string(pinHash), time.Now().UTC()); err != nil {
		return err
	}
	return nil
}

func newPINResetToken() (raw string, hash string, err error) {
	bytes := make([]byte, 32)
	if _, err = rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("generate staff PIN reset token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(bytes)
	return raw, hashPINResetToken(raw), nil
}

func hashPINResetToken(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", digest[:])
}
