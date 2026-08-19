package billing

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ── Tier catalogue ────────────────────────────────────────────────────────────

// TierFeature is a customer-visible capability included or unavailable in a tier.
type TierFeature struct {
	Text     string `json:"text"`
	Included bool   `json:"included"`
}

// VendorTier is the server-authoritative subscription catalogue record.
type VendorTier struct {
	ID           uuid.UUID     `json:"id"`
	Name         string        `json:"name"`
	MonthlyPrice float64       `json:"monthly_price"`
	Description  string        `json:"description"`
	DisplayOrder int           `json:"display_order"`
	IsAvailable  bool          `json:"is_available"`
	IsPopular    bool          `json:"is_popular"`
	Features     []TierFeature `json:"features"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// ── Subscription checkout ────────────────────────────────────────────────────

// CheckoutStatus describes the state of a server-created subscription checkout.
type CheckoutStatus string

const (
	CheckoutPending    CheckoutStatus = "PENDING"
	CheckoutSuccessful CheckoutStatus = "SUCCESSFUL"
	CheckoutFailed     CheckoutStatus = "FAILED"
	CheckoutExpired    CheckoutStatus = "EXPIRED"
)

// SubscriptionCheckout locks all commercially significant data before a payment
// request is sent. No value from the browser can change its amount, currency, tier, or reference.
type SubscriptionCheckout struct {
	ID                   uuid.UUID      `json:"id"`
	VendorID             uuid.UUID      `json:"vendor_id"`
	TierID               uuid.UUID      `json:"tier_id"`
	TierName             string         `json:"tier_name"`
	Amount               float64        `json:"amount"`
	Currency             string         `json:"currency"`
	Reference            string         `json:"reference"`
	Status               CheckoutStatus `json:"status"`
	ProviderCollectionID string         `json:"provider_collection_id,omitempty"`
	ProviderStatus       string         `json:"provider_status,omitempty"`
	SubscriptionID       *uuid.UUID     `json:"subscription_id,omitempty"`
	InvoiceID            *uuid.UUID     `json:"invoice_id,omitempty"`
	ExpiresAt            time.Time      `json:"expires_at"`
	CompletedAt          *time.Time     `json:"completed_at,omitempty"`
	FailureReason        string         `json:"failure_reason,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

// CreateCheckoutRequest contains only a requested tier. The vendor identity,
// reference, amount, currency, and provider configuration are server-derived.
type CreateCheckoutRequest struct {
	TierID string `json:"tier_id"`
}

// CheckoutSession is a browser-safe record of a server-created checkout. The
// portal uses the checkout identifier for its next authenticated step; it never
// receives provider credentials or a hosted-widget configuration.
type CheckoutSession struct {
	Checkout *SubscriptionCheckout `json:"checkout"`
}

// InitiateMobileMoneyCollectionRequest contains only payer-provided information.
// Amount, currency, and reference remain locked in SubscriptionCheckout.
type InitiateMobileMoneyCollectionRequest struct {
	Phone    string `json:"phone"`
	Operator string `json:"operator"`
}

// MobileMoneyCollectionRequest is the normalized request issued by the backend
// to the payment provider after it has loaded the checkout record.
type MobileMoneyCollectionRequest struct {
	Amount    float64
	Currency  string
	Reference string
	Phone     string
	Operator  string
	Country   string
	Bearer    string
}

// ProviderCollection is the normalized read-only collection result returned by
// the payment provider during initiation or server-side verification.
type ProviderCollection struct {
	ID        string
	Reference string
	Amount    float64
	Currency  string
	Status    string
	Reason    string
}

// CollectionVerifier verifies a collection reference using a server-held secret.
type CollectionVerifier interface {
	VerifyCollection(ctx context.Context, reference string) (*ProviderCollection, error)
}

// CollectionInitiator starts a collection using a server-held provider secret.
type CollectionInitiator interface {
	InitiateMobileMoneyCollection(ctx context.Context, request MobileMoneyCollectionRequest) (*ProviderCollection, error)
}

// ── Subscription ──────────────────────────────────────────────────────────────

// SubscriptionStatus represents the lifecycle state of a vendor subscription.
type SubscriptionStatus string

const (
	SubTrial     SubscriptionStatus = "TRIAL"
	SubActive    SubscriptionStatus = "ACTIVE"
	SubPastDue   SubscriptionStatus = "PAST_DUE"
	SubSuspended SubscriptionStatus = "SUSPENDED"
	SubCancelled SubscriptionStatus = "CANCELLED"
)

// validSubTransitions defines allowed subscription state machine transitions.
var validSubTransitions = map[SubscriptionStatus][]SubscriptionStatus{
	SubTrial:     {SubActive, SubCancelled},
	SubActive:    {SubPastDue, SubCancelled},
	SubPastDue:   {SubActive, SubSuspended, SubCancelled},
	SubSuspended: {SubActive, SubCancelled},
	SubCancelled: {},
}

// CanTransitionSub returns true if the subscription transition is valid.
func CanTransitionSub(current, next SubscriptionStatus) bool {
	allowed, ok := validSubTransitions[current]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == next {
			return true
		}
	}
	return false
}

// BillingCycle represents how often a vendor is billed.
type BillingCycle string

const (
	CycleMonthly BillingCycle = "MONTHLY"
	CycleAnnual  BillingCycle = "ANNUAL"
)

// VendorSubscription represents a vendor's active subscription to a tier.
type VendorSubscription struct {
	ID                 uuid.UUID          `json:"id"`
	VendorID           uuid.UUID          `json:"vendor_id"`
	TierID             uuid.UUID          `json:"tier_id"`
	TierName           string             `json:"tier_name,omitempty"`
	TierPrice          float64            `json:"tier_price,omitempty"`
	Status             SubscriptionStatus `json:"status"`
	BillingCycle       BillingCycle       `json:"billing_cycle"`
	CurrentPeriodStart time.Time          `json:"current_period_start"`
	CurrentPeriodEnd   time.Time          `json:"current_period_end"`
	TrialEndsAt        *time.Time         `json:"trial_ends_at,omitempty"`
	CancelledAt        *time.Time         `json:"cancelled_at,omitempty"`
	CancelReason       string             `json:"cancel_reason,omitempty"`
	AutoRenew          bool               `json:"auto_renew"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}

// CreateSubscriptionRequest is the payload for creating a new subscription.
type CreateSubscriptionRequest struct {
	VendorID     string `json:"vendor_id"`
	TierID       string `json:"tier_id"`
	BillingCycle string `json:"billing_cycle,omitempty"` // defaults to MONTHLY
	TrialDays    int    `json:"trial_days,omitempty"`    // 0 = no trial, start ACTIVE
}

// ChangeTierRequest is the payload for upgrading or downgrading a subscription tier.
type ChangeTierRequest struct {
	TierID string `json:"tier_id"`
	Reason string `json:"reason,omitempty"`
}

// CancelSubscriptionRequest is the payload for cancelling a subscription.
type CancelSubscriptionRequest struct {
	Reason string `json:"reason"`
}

// UpdateStatusRequest is the payload for manually updating subscription status (admin).
type UpdateSubStatusRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// ── Invoice ───────────────────────────────────────────────────────────────────

// InvoiceStatus represents the lifecycle state of a billing invoice.
type InvoiceStatus string

const (
	InvDraft         InvoiceStatus = "DRAFT"
	InvOpen          InvoiceStatus = "OPEN"
	InvPaid          InvoiceStatus = "PAID"
	InvVoid          InvoiceStatus = "VOID"
	InvUncollectible InvoiceStatus = "UNCOLLECTIBLE"
)

// LineItem represents a single line on an invoice.
type LineItem struct {
	Description string  `json:"description"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Amount      float64 `json:"amount"`
}

// BillingInvoice represents a billing invoice for a vendor subscription cycle.
type BillingInvoice struct {
	ID               uuid.UUID     `json:"id"`
	SubscriptionID   uuid.UUID     `json:"subscription_id"`
	VendorID         uuid.UUID     `json:"vendor_id"`
	InvoiceNumber    string        `json:"invoice_number"`
	Amount           float64       `json:"amount"`
	Currency         string        `json:"currency"`
	Status           InvoiceStatus `json:"status"`
	PeriodStart      time.Time     `json:"period_start"`
	PeriodEnd        time.Time     `json:"period_end"`
	DueDate          time.Time     `json:"due_date"`
	PaidAt           *time.Time    `json:"paid_at,omitempty"`
	PaymentReference string        `json:"payment_reference,omitempty"`
	LineItems        []LineItem    `json:"line_items"`
	Notes            string        `json:"notes,omitempty"`
	IdempotencyKey   string        `json:"idempotency_key,omitempty"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

// MarkPaidRequest is the payload for marking an invoice as paid.
type MarkPaidRequest struct {
	PaymentReference string `json:"payment_reference"`
	Notes            string `json:"notes,omitempty"`
}
