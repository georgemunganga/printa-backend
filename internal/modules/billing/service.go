package billing

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service defines billing business logic.
type Service interface {
	// Subscription
	CreateSubscription(ctx context.Context, req CreateSubscriptionRequest) (*VendorSubscription, error)
	GetSubscription(ctx context.Context, vendorID string) (*VendorSubscription, error)
	ChangeTier(ctx context.Context, vendorID string, req ChangeTierRequest) (*VendorSubscription, error)
	CancelSubscription(ctx context.Context, vendorID string, req CancelSubscriptionRequest) (*VendorSubscription, error)
	UpdateStatus(ctx context.Context, vendorID string, req UpdateSubStatusRequest) (*VendorSubscription, error)

	// Invoice
	GenerateInvoice(ctx context.Context, vendorID string, idempotencyKey string) (*BillingInvoice, error)
	GetInvoice(ctx context.Context, id string) (*BillingInvoice, error)
	GetInvoiceByNumber(ctx context.Context, number string) (*BillingInvoice, error)
	ListVendorInvoices(ctx context.Context, vendorID string) ([]*BillingInvoice, error)
	MarkInvoicePaid(ctx context.Context, id string, req MarkPaidRequest) (*BillingInvoice, error)
	VoidInvoice(ctx context.Context, id string) (*BillingInvoice, error)
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

// ── Subscription ──────────────────────────────────────────────────────────────

func (s *service) CreateSubscription(ctx context.Context, req CreateSubscriptionRequest) (*VendorSubscription, error) {
	if req.VendorID == "" {
		return nil, fmt.Errorf("vendor_id is required")
	}
	if req.TierID == "" {
		return nil, fmt.Errorf("tier_id is required")
	}

	// Validate tier exists
	_, _, err := s.repo.GetTierByID(ctx, req.TierID)
	if err != nil {
		return nil, fmt.Errorf("tier not found: %w", err)
	}

	// Check for existing subscription
	existing, err := s.repo.GetSubscriptionByVendor(ctx, req.VendorID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("vendor already has an active subscription (id: %s)", existing.ID)
	}

	cycle := CycleMonthly
	if strings.ToUpper(req.BillingCycle) == "ANNUAL" {
		cycle = CycleAnnual
	}

	now := time.Now()
	periodEnd := now.AddDate(0, 1, 0) // 1 month
	if cycle == CycleAnnual {
		periodEnd = now.AddDate(1, 0, 0) // 1 year
	}

	sub := &VendorSubscription{
		ID:                 uuid.New(),
		VendorID:           uuid.MustParse(req.VendorID),
		TierID:             uuid.MustParse(req.TierID),
		Status:             SubActive,
		BillingCycle:       cycle,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   periodEnd,
		AutoRenew:          true,
	}

	// Apply trial if requested
	if req.TrialDays > 0 {
		sub.Status = SubTrial
		trialEnd := now.AddDate(0, 0, req.TrialDays)
		sub.TrialEndsAt = &trialEnd
		sub.CurrentPeriodEnd = trialEnd
	}

	if err := s.repo.CreateSubscription(ctx, sub); err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, fmt.Errorf("vendor already has a subscription")
		}
		return nil, err
	}

	return s.repo.GetSubscriptionByVendor(ctx, req.VendorID)
}

func (s *service) GetSubscription(ctx context.Context, vendorID string) (*VendorSubscription, error) {
	sub, err := s.repo.GetSubscriptionByVendor(ctx, vendorID)
	if err != nil {
		return nil, fmt.Errorf("subscription not found for vendor %s: %w", vendorID, err)
	}
	return sub, nil
}

func (s *service) ChangeTier(ctx context.Context, vendorID string, req ChangeTierRequest) (*VendorSubscription, error) {
	if req.TierID == "" {
		return nil, fmt.Errorf("tier_id is required")
	}

	sub, err := s.repo.GetSubscriptionByVendor(ctx, vendorID)
	if err != nil {
		return nil, fmt.Errorf("subscription not found: %w", err)
	}

	if sub.Status == SubCancelled || sub.Status == SubSuspended {
		return nil, fmt.Errorf("cannot change tier on a %s subscription", sub.Status)
	}

	if sub.TierID.String() == req.TierID {
		return nil, fmt.Errorf("vendor is already on this tier")
	}

	// Validate new tier exists
	_, _, err = s.repo.GetTierByID(ctx, req.TierID)
	if err != nil {
		return nil, fmt.Errorf("new tier not found: %w", err)
	}

	if err := s.repo.UpdateSubscriptionTier(ctx, sub.ID.String(), req.TierID); err != nil {
		return nil, err
	}

	return s.repo.GetSubscriptionByVendor(ctx, vendorID)
}

func (s *service) CancelSubscription(ctx context.Context, vendorID string, req CancelSubscriptionRequest) (*VendorSubscription, error) {
	sub, err := s.repo.GetSubscriptionByVendor(ctx, vendorID)
	if err != nil {
		return nil, fmt.Errorf("subscription not found: %w", err)
	}

	if sub.Status == SubCancelled {
		return nil, fmt.Errorf("subscription is already cancelled")
	}

	if !CanTransitionSub(sub.Status, SubCancelled) {
		return nil, fmt.Errorf("cannot cancel a subscription in %s status", sub.Status)
	}

	if err := s.repo.UpdateSubscriptionStatus(ctx, sub.ID.String(), SubCancelled, req.Reason); err != nil {
		return nil, err
	}

	return s.repo.GetSubscriptionByVendor(ctx, vendorID)
}

func (s *service) UpdateStatus(ctx context.Context, vendorID string, req UpdateSubStatusRequest) (*VendorSubscription, error) {
	if req.Status == "" {
		return nil, fmt.Errorf("status is required")
	}

	sub, err := s.repo.GetSubscriptionByVendor(ctx, vendorID)
	if err != nil {
		return nil, fmt.Errorf("subscription not found: %w", err)
	}

	next := SubscriptionStatus(strings.ToUpper(req.Status))
	if !CanTransitionSub(sub.Status, next) {
		return nil, fmt.Errorf("cannot transition subscription from %s to %s", sub.Status, next)
	}

	if err := s.repo.UpdateSubscriptionStatus(ctx, sub.ID.String(), next, req.Reason); err != nil {
		return nil, err
	}

	return s.repo.GetSubscriptionByVendor(ctx, vendorID)
}

// ── Invoice ───────────────────────────────────────────────────────────────────

func (s *service) GenerateInvoice(ctx context.Context, vendorID string, idempotencyKey string) (*BillingInvoice, error) {
	// Idempotency: return existing invoice if key already used
	if idempotencyKey != "" {
		existing, err := s.repo.GetInvoiceByIdempotencyKey(ctx, idempotencyKey)
		if err == nil && existing != nil {
			return existing, nil
		}
	}

	sub, err := s.repo.GetSubscriptionByVendor(ctx, vendorID)
	if err != nil {
		return nil, fmt.Errorf("subscription not found: %w", err)
	}

	if sub.Status == SubCancelled {
		return nil, fmt.Errorf("cannot generate invoice for a cancelled subscription")
	}

	tierName, tierPrice, err := s.repo.GetTierByID(ctx, sub.TierID.String())
	if err != nil {
		return nil, fmt.Errorf("tier not found: %w", err)
	}

	// Free tier (CORE) = ZMW 0 — still generate invoice for audit trail
	amount := tierPrice
	if sub.BillingCycle == CycleAnnual {
		amount = tierPrice * 12 * 0.9 // 10% annual discount
	}

	now := time.Now()
	inv := &BillingInvoice{
		ID:             uuid.New(),
		SubscriptionID: sub.ID,
		VendorID:       sub.VendorID,
		InvoiceNumber:  generateInvoiceNumber(now),
		Amount:         amount,
		Currency:       "ZMW",
		Status:         InvOpen,
		PeriodStart:    sub.CurrentPeriodStart,
		PeriodEnd:      sub.CurrentPeriodEnd,
		DueDate:        now.AddDate(0, 0, 7), // 7-day payment window
		LineItems: []LineItem{
			{
				Description: fmt.Sprintf("Printa %s Plan (%s)", tierName, sub.BillingCycle),
				Quantity:    1,
				UnitPrice:   amount,
				Amount:      amount,
			},
		},
		IdempotencyKey: idempotencyKey,
	}

	if err := s.repo.CreateInvoice(ctx, inv); err != nil {
		return nil, err
	}

	return s.repo.GetInvoiceByID(ctx, inv.ID.String())
}

func (s *service) GetInvoice(ctx context.Context, id string) (*BillingInvoice, error) {
	return s.repo.GetInvoiceByID(ctx, id)
}

func (s *service) GetInvoiceByNumber(ctx context.Context, number string) (*BillingInvoice, error) {
	return s.repo.GetInvoiceByNumber(ctx, number)
}

func (s *service) ListVendorInvoices(ctx context.Context, vendorID string) ([]*BillingInvoice, error) {
	return s.repo.ListInvoicesByVendor(ctx, vendorID)
}

func (s *service) MarkInvoicePaid(ctx context.Context, id string, req MarkPaidRequest) (*BillingInvoice, error) {
	inv, err := s.repo.GetInvoiceByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("invoice not found: %w", err)
	}
	if inv.Status == InvPaid {
		return nil, fmt.Errorf("invoice is already marked as paid")
	}
	if inv.Status == InvVoid {
		return nil, fmt.Errorf("cannot mark a voided invoice as paid")
	}
	if err := s.repo.MarkInvoicePaid(ctx, id, req.PaymentReference, req.Notes); err != nil {
		return nil, err
	}
	return s.repo.GetInvoiceByID(ctx, id)
}

func (s *service) VoidInvoice(ctx context.Context, id string) (*BillingInvoice, error) {
	inv, err := s.repo.GetInvoiceByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("invoice not found: %w", err)
	}
	if inv.Status == InvPaid {
		return nil, fmt.Errorf("cannot void a paid invoice — issue a refund instead")
	}
	if inv.Status == InvVoid {
		return nil, fmt.Errorf("invoice is already voided")
	}
	if err := s.repo.VoidInvoice(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.GetInvoiceByID(ctx, id)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func generateInvoiceNumber(t time.Time) string {
	suffix := fmt.Sprintf("%04d", rand.Intn(10000))
	return fmt.Sprintf("INV-%s-%s", t.Format("200601"), suffix)
}
