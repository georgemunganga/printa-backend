package wallet

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeRepository struct {
	posted Posting
	called bool
}

func (r *fakeRepository) GetAccountByVendor(context.Context, uuid.UUID) (*Account, error) {
	return nil, ErrAccountNotFound
}
func (r *fakeRepository) GetBalance(context.Context, uuid.UUID) (*Balance, error) { return nil, nil }
func (r *fakeRepository) ListEntries(context.Context, uuid.UUID, int) ([]LedgerEntry, error) {
	return nil, nil
}
func (r *fakeRepository) ListWithdrawals(context.Context, uuid.UUID, int) ([]WithdrawalRequest, error) {
	return nil, nil
}
func (r *fakeRepository) Post(_ context.Context, posting Posting) (uuid.UUID, bool, error) {
	r.called = true
	r.posted = posting
	return uuid.New(), false, nil
}

func validPosting() Posting {
	walletID := uuid.New()
	vendorID := uuid.New()
	return Posting{
		IdempotencyKey:  "wallet-test-1",
		SourceType:      "TEST",
		SourceReference: "test-source-1",
		Currency:        "zmw",
		Narrative:       "Test balanced wallet journal",
		OccurredAt:      time.Now().UTC(),
		Entries: []PostingEntry{
			{
				WalletAccountID: &walletID,
				VendorID:        &vendorID,
				EntryType:       EntryCollectionSettled,
				LedgerAccount:   LedgerVendorAvailable,
				AmountMinor:     10000,
			},
			{
				EntryType:     EntryCollectionSettled,
				LedgerAccount: LedgerPlatformClearing,
				AmountMinor:   -10000,
			},
		},
	}
}

func TestPostInternalRejectsUnbalancedJournal(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo)
	posting := validPosting()
	posting.Entries[1].AmountMinor = -9999

	if _, _, err := service.PostInternal(context.Background(), posting); err == nil {
		t.Fatal("expected unbalanced journal to be rejected")
	}
	if repo.called {
		t.Fatal("repository must not receive an invalid posting")
	}
}

func TestPostInternalRejectsVendorlessVendorLedgerEntry(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo)
	posting := validPosting()
	posting.Entries[0].VendorID = nil

	if _, _, err := service.PostInternal(context.Background(), posting); err == nil {
		t.Fatal("expected vendor ledger entry without vendor identity to be rejected")
	}
	if repo.called {
		t.Fatal("repository must not receive an invalid posting")
	}
}

func TestPostInternalNormalizesCurrencyBeforeRepository(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo)
	posting := validPosting()

	if _, _, err := service.PostInternal(context.Background(), posting); err != nil {
		t.Fatalf("expected valid posting: %v", err)
	}
	if !repo.called {
		t.Fatal("expected repository call for a valid posting")
	}
	if repo.posted.Currency != "ZMW" {
		t.Fatalf("expected normalized ZMW currency, received %q", repo.posted.Currency)
	}
}
