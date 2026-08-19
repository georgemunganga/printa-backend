package attendance

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/georgemunganga/printa-backend/internal/modules/comms"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type pinResetRepositoryStub struct {
	Repository
	owner         *ownerContact
	created       *PINResetRecord
	consumedHash  string
	storedPINHash string
	consumeErr    error
}

func (s *pinResetRepositoryStub) GetStoreOwnerContact(context.Context, string) (*ownerContact, error) {
	return s.owner, nil
}

func (s *pinResetRepositoryStub) CreatePINReset(_ context.Context, reset *PINResetRecord) error {
	copy := *reset
	s.created = &copy
	return nil
}

func (s *pinResetRepositoryStub) ConsumePINResetAndSetOwnerPIN(_ context.Context, tokenHash, pinHash string, _ time.Time) error {
	s.consumedHash = tokenHash
	s.storedPINHash = pinHash
	return s.consumeErr
}

type pinResetMailerStub struct {
	req *comms.SendRequest
}

func (s *pinResetMailerStub) Send(_ context.Context, req comms.SendRequest) (*comms.SendResult, error) {
	copy := req
	s.req = &copy
	return &comms.SendResult{Status: comms.DeliverySent}, nil
}

func TestRequestOwnerPINResetStoresOnlyDigestAndEmailsOneTimeLink(t *testing.T) {
	repo := &pinResetRepositoryStub{owner: &ownerContact{StoreID: uuid.New(), OwnerID: uuid.New(), Email: "owner@example.test"}}
	mailer := &pinResetMailerStub{}
	svc := NewService(repo, WithPINResetMailer(mailer, "https://vendor.example.test"))

	if err := svc.RequestOwnerPINReset(context.Background(), repo.owner.StoreID.String()); err != nil {
		t.Fatalf("RequestOwnerPINReset() error = %v", err)
	}
	if repo.created == nil || len(repo.created.TokenHash) != 64 || strings.Contains(mailer.req.Body, repo.created.TokenHash) {
		t.Fatalf("reset token persistence/email safety failed: record=%#v email=%#v", repo.created, mailer.req)
	}
	if mailer.req == nil || !strings.Contains(mailer.req.Body, "https://vendor.example.test/staff-pin/reset?token=") || mailer.req.Recipient != "owner@example.test" {
		t.Fatalf("reset email = %#v", mailer.req)
	}
	if time.Until(repo.created.ExpiresAt) < 14*time.Minute || time.Until(repo.created.ExpiresAt) > 16*time.Minute {
		t.Fatalf("reset expiry = %v, want approximately 15 minutes", repo.created.ExpiresAt)
	}
}

func TestConfirmOwnerPINResetHashesNewPINAndConsumesToken(t *testing.T) {
	repo := &pinResetRepositoryStub{}
	svc := NewService(repo)
	const token = "single-use-token"

	if err := svc.ConfirmOwnerPINReset(context.Background(), token, "654321"); err != nil {
		t.Fatalf("ConfirmOwnerPINReset() error = %v", err)
	}
	if repo.consumedHash != hashPINResetToken(token) {
		t.Fatalf("consumed hash = %q", repo.consumedHash)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(repo.storedPINHash), []byte("654321")); err != nil {
		t.Fatalf("stored password is not a bcrypt hash for the new PIN: %v", err)
	}
}

func TestConfirmOwnerPINResetRejectsInvalidPINBeforeConsumingToken(t *testing.T) {
	repo := &pinResetRepositoryStub{consumeErr: errors.New("must not run")}
	svc := NewService(repo)

	if err := svc.ConfirmOwnerPINReset(context.Background(), "token", "123"); err == nil {
		t.Fatal("expected invalid PIN rejection")
	}
	if repo.consumedHash != "" {
		t.Fatal("invalid PIN must not consume reset token")
	}
}
