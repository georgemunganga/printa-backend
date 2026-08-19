package wallet

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const defaultCurrency = "ZMW"

type AccountState string

const (
	AccountPending   AccountState = "PENDING"
	AccountActive    AccountState = "ACTIVE"
	AccountSuspended AccountState = "SUSPENDED"
	AccountClosed    AccountState = "CLOSED"
)

type LedgerAccount string

const (
	LedgerVendorAvailable        LedgerAccount = "VENDOR_AVAILABLE"
	LedgerVendorPending          LedgerAccount = "VENDOR_PENDING"
	LedgerVendorHeld             LedgerAccount = "VENDOR_HELD"
	LedgerPlatformClearing       LedgerAccount = "PLATFORM_CLEARING"
	LedgerPlatformTransactionFee LedgerAccount = "PLATFORM_TRANSACTION_CHARGE"
	LedgerPlatformExpense        LedgerAccount = "PLATFORM_EXPENSE"
)

type EntryType string

const (
	EntryOrderSaleCredit         EntryType = "ORDER_SALE_CREDIT"
	EntryPOSCashReceipt          EntryType = "POS_CASH_RECEIPT"
	EntryPOSCardReceipt          EntryType = "POS_CARD_RECEIPT"
	EntryManualDepositPending    EntryType = "MANUAL_DEPOSIT_PENDING_REVIEW"
	EntryCollectionPending       EntryType = "COLLECTION_PENDING"
	EntryCollectionSettled       EntryType = "COLLECTION_SETTLED"
	EntryCollectionReversed      EntryType = "COLLECTION_REVERSED"
	EntryTransactionCharge       EntryType = "TRANSACTION_CHARGE"
	EntryVendorExpenseDebit      EntryType = "VENDOR_EXPENSE_DEBIT"
	EntryRefundDebit             EntryType = "REFUND_DEBIT"
	EntryWithdrawalHold          EntryType = "WITHDRAWAL_HOLD"
	EntryWithdrawalPaid          EntryType = "WITHDRAWAL_PAID"
	EntryWithdrawalFailedRelease EntryType = "WITHDRAWAL_FAILED_RELEASE"
	EntryAdjustment              EntryType = "ADJUSTMENT"
)

type WithdrawalStatus string

const (
	WithdrawalPendingReview WithdrawalStatus = "PENDING_REVIEW"
	WithdrawalApproved      WithdrawalStatus = "APPROVED"
	WithdrawalSubmitted     WithdrawalStatus = "SUBMITTED"
	WithdrawalPaid          WithdrawalStatus = "PAID"
	WithdrawalFailed        WithdrawalStatus = "FAILED"
	WithdrawalCancelled     WithdrawalStatus = "CANCELLED"
	WithdrawalRejected      WithdrawalStatus = "REJECTED"
)

type Account struct {
	ID                        uuid.UUID    `json:"id"`
	VendorID                  uuid.UUID    `json:"vendor_id"`
	Currency                  string       `json:"currency"`
	State                     AccountState `json:"state"`
	ProviderVirtualAccountRef string       `json:"-"`
	ProviderAccountStatus     string       `json:"-"`
	CreatedAt                 time.Time    `json:"created_at"`
	UpdatedAt                 time.Time    `json:"updated_at"`
	ActivatedAt               *time.Time   `json:"activated_at,omitempty"`
}

type Balance struct {
	WalletAccountID uuid.UUID `json:"wallet_account_id"`
	Currency        string    `json:"currency"`
	AvailableMinor  int64     `json:"available_minor"`
	PendingMinor    int64     `json:"pending_minor"`
	HeldMinor       int64     `json:"held_minor"`
	CalculatedAt    time.Time `json:"calculated_at"`
}

type LedgerEntry struct {
	ID              uuid.UUID     `json:"id"`
	JournalID       uuid.UUID     `json:"journal_id"`
	WalletAccountID *uuid.UUID    `json:"wallet_account_id,omitempty"`
	VendorID        *uuid.UUID    `json:"vendor_id,omitempty"`
	EntryType       EntryType     `json:"entry_type"`
	LedgerAccount   LedgerAccount `json:"ledger_account"`
	AmountMinor     int64         `json:"amount_minor"`
	Currency        string        `json:"currency"`
	ProviderRef     string        `json:"provider_reference,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
}

type WithdrawalRequest struct {
	ID              uuid.UUID        `json:"id"`
	WalletAccountID uuid.UUID        `json:"wallet_account_id"`
	VendorID        uuid.UUID        `json:"vendor_id"`
	AmountMinor     int64            `json:"amount_minor"`
	Currency        string           `json:"currency"`
	Status          WithdrawalStatus `json:"status"`
	RequestedAt     time.Time        `json:"requested_at"`
	ReviewedAt      *time.Time       `json:"reviewed_at,omitempty"`
	CompletedAt     *time.Time       `json:"completed_at,omitempty"`
	FailureReason   string           `json:"failure_reason,omitempty"`
}

// Posting is an internal-only command. No HTTP route accepts it until the source
// modules and financial controls have been individually approved.
type Posting struct {
	IdempotencyKey    string
	SourceType        string
	SourceReference   string
	ProviderReference string
	Currency          string
	Narrative         string
	ActorType         string
	ActorID           *uuid.UUID
	OccurredAt        time.Time
	Metadata          json.RawMessage
	Entries           []PostingEntry
}

type PostingEntry struct {
	WalletAccountID   *uuid.UUID
	VendorID          *uuid.UUID
	EntryType         EntryType
	LedgerAccount     LedgerAccount
	AmountMinor       int64
	FeePolicyID       *uuid.UUID
	ProviderReference string
	Metadata          json.RawMessage
}

type WalletOverview struct {
	Account     *Account            `json:"account,omitempty"`
	Balance     *Balance            `json:"balance,omitempty"`
	Entries     []LedgerEntry       `json:"entries"`
	Withdrawals []WithdrawalRequest `json:"withdrawals"`
}
