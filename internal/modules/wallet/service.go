package wallet

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service interface {
	GetOverviewByVendor(ctx context.Context, vendorID uuid.UUID) (*WalletOverview, error)
	PostInternal(ctx context.Context, posting Posting) (journalID uuid.UUID, duplicate bool, err error)
}

type service struct {
	repository Repository
}

func NewService(repository Repository) Service {
	return &service{repository: repository}
}

func (s *service) GetOverviewByVendor(ctx context.Context, vendorID uuid.UUID) (*WalletOverview, error) {
	account, err := s.repository.GetAccountByVendor(ctx, vendorID)
	if err == ErrAccountNotFound {
		return &WalletOverview{
			Entries:     make([]LedgerEntry, 0),
			Withdrawals: make([]WithdrawalRequest, 0),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	balance, err := s.repository.GetBalance(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	entries, err := s.repository.ListEntries(ctx, account.ID, 100)
	if err != nil {
		return nil, err
	}
	withdrawals, err := s.repository.ListWithdrawals(ctx, account.VendorID, 100)
	if err != nil {
		return nil, err
	}
	return &WalletOverview{Account: account, Balance: balance, Entries: entries, Withdrawals: withdrawals}, nil
}

func (s *service) PostInternal(ctx context.Context, posting Posting) (uuid.UUID, bool, error) {
	if err := validatePosting(&posting); err != nil {
		return uuid.Nil, false, err
	}
	return s.repository.Post(ctx, posting)
}

func validatePosting(posting *Posting) error {
	posting.IdempotencyKey = strings.TrimSpace(posting.IdempotencyKey)
	posting.SourceType = strings.TrimSpace(posting.SourceType)
	posting.SourceReference = strings.TrimSpace(posting.SourceReference)
	posting.ProviderReference = strings.TrimSpace(posting.ProviderReference)
	posting.Currency = strings.ToUpper(strings.TrimSpace(posting.Currency))
	posting.Narrative = strings.TrimSpace(posting.Narrative)
	posting.ActorType = strings.ToUpper(strings.TrimSpace(posting.ActorType))

	if posting.IdempotencyKey == "" || len(posting.IdempotencyKey) > 160 {
		return fmt.Errorf("a wallet posting idempotency key of at most 160 characters is required")
	}
	if posting.SourceType == "" || posting.SourceReference == "" || posting.Narrative == "" {
		return fmt.Errorf("wallet posting source type, source reference, and narrative are required")
	}
	if len(posting.SourceType) > 48 || len(posting.SourceReference) > 160 || len(posting.Narrative) > 2000 {
		return fmt.Errorf("wallet posting text field exceeds its supported length")
	}
	if posting.Currency == "" {
		posting.Currency = defaultCurrency
	}
	if len(posting.Currency) != 3 {
		return fmt.Errorf("wallet posting currency must be an ISO 4217 code")
	}
	if posting.ActorType == "" {
		posting.ActorType = "SYSTEM"
	}
	switch posting.ActorType {
	case "SYSTEM", "USER", "ADMIN", "RECONCILIATION":
	default:
		return fmt.Errorf("wallet posting actor type is invalid")
	}
	if posting.OccurredAt.IsZero() {
		posting.OccurredAt = time.Now().UTC()
	}
	if len(posting.Entries) < 2 {
		return fmt.Errorf("a wallet journal requires at least two balanced entries")
	}
	if len(posting.Entries) > 20 {
		return fmt.Errorf("a wallet journal may contain at most 20 entries")
	}

	var journalTotal int64
	for i := range posting.Entries {
		entry := &posting.Entries[i]
		if entry.AmountMinor == 0 {
			return fmt.Errorf("wallet entry %d must have a non-zero minor-unit amount", i+1)
		}
		if !validEntryType(entry.EntryType) || !validLedgerAccount(entry.LedgerAccount) {
			return fmt.Errorf("wallet entry %d has an unsupported type or ledger account", i+1)
		}
		isVendorAccount := entry.LedgerAccount == LedgerVendorAvailable || entry.LedgerAccount == LedgerVendorPending || entry.LedgerAccount == LedgerVendorHeld
		if isVendorAccount && (entry.WalletAccountID == nil || entry.VendorID == nil) {
			return fmt.Errorf("wallet entry %d requires a vendor wallet account and vendor", i+1)
		}
		if !isVendorAccount && (entry.WalletAccountID != nil || entry.VendorID != nil) {
			return fmt.Errorf("wallet entry %d must not attach a vendor to a platform ledger account", i+1)
		}
		journalTotal += entry.AmountMinor
	}
	if journalTotal != 0 {
		return fmt.Errorf("wallet journal entries must balance to zero")
	}
	return nil
}

func validLedgerAccount(account LedgerAccount) bool {
	switch account {
	case LedgerVendorAvailable, LedgerVendorPending, LedgerVendorHeld, LedgerPlatformClearing, LedgerPlatformTransactionFee, LedgerPlatformExpense:
		return true
	default:
		return false
	}
}

func validEntryType(entryType EntryType) bool {
	switch entryType {
	case EntryOrderSaleCredit, EntryPOSCashReceipt, EntryPOSCardReceipt, EntryManualDepositPending, EntryCollectionPending, EntryCollectionSettled, EntryCollectionReversed, EntryTransactionCharge, EntryVendorExpenseDebit, EntryRefundDebit, EntryWithdrawalHold, EntryWithdrawalPaid, EntryWithdrawalFailedRelease, EntryAdjustment:
		return true
	default:
		return false
	}
}
