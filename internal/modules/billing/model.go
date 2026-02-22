package billing

import (
	"time"

	"github.com/google/uuid"
)

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
	InvDraft          InvoiceStatus = "DRAFT"
	InvOpen           InvoiceStatus = "OPEN"
	InvPaid           InvoiceStatus = "PAID"
	InvVoid           InvoiceStatus = "VOID"
	InvUncollectible  InvoiceStatus = "UNCOLLECTIBLE"
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
