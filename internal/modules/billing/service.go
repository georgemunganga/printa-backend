package billing

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service defines billing business logic.
type Service interface {
	// Tier catalogue
	ListTiers(ctx context.Context) ([]*VendorTier, error)

	// Subscription checkout
	CreateSubscriptionCheckout(ctx context.Context, vendorID string, req CreateCheckoutRequest) (*CheckoutSession, error)
	GetSubscriptionCheckout(ctx context.Context, vendorID, checkoutID string) (*SubscriptionCheckout, error)
	InitiateSubscriptionMobileMoneyCollection(ctx context.Context, vendorID, checkoutID string, req InitiateMobileMoneyCollectionRequest) (*SubscriptionCheckout, error)
	VerifySubscriptionCheckout(ctx context.Context, vendorID, checkoutID string) (*SubscriptionCheckout, error)
	VerifySubscriptionCheckoutByReference(ctx context.Context, reference string) (*SubscriptionCheckout, error)

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

type CheckoutConfig struct {
	Verifier  CollectionVerifier
	Initiator CollectionInitiator
}

type ServiceOption func(*service)

// WithCheckoutConfig enables server-mediated subscription collection. Provider
// credentials remain inside the verifier and initiator implementations.
func WithCheckoutConfig(config CheckoutConfig) ServiceOption {
	return func(s *service) {
		s.checkout = config
	}
}

type service struct {
	repo     Repository
	checkout CheckoutConfig
}

func NewService(repo Repository, options ...ServiceOption) Service {
	s := &service{repo: repo}
	for _, option := range options {
		option(s)
	}
	return s
}

func (s *service) ListTiers(ctx context.Context) ([]*VendorTier, error) {
	return s.repo.ListTiers(ctx)
}

// ── Subscription checkout ────────────────────────────────────────────────────

func (s *service) CreateSubscriptionCheckout(ctx context.Context, vendorID string, req CreateCheckoutRequest) (*CheckoutSession, error) {
	if strings.TrimSpace(vendorID) == "" {
		return nil, fmt.Errorf("vendor_id is required")
	}
	if strings.TrimSpace(req.TierID) == "" {
		return nil, fmt.Errorf("tier_id is required")
	}
	if s.checkout.Verifier == nil || s.checkout.Initiator == nil {
		return nil, fmt.Errorf("subscription payment collection is not configured")
	}
	vendorUUID, err := uuid.Parse(vendorID)
	if err != nil {
		return nil, fmt.Errorf("invalid vendor_id")
	}
	tier, err := s.repo.GetTierCatalogueEntry(ctx, req.TierID)
	if err != nil {
		return nil, fmt.Errorf("subscription tier not found")
	}
	if !tier.IsAvailable || tier.MonthlyPrice <= 0 {
		return nil, fmt.Errorf("subscription tier is not available for checkout")
	}

	now := time.Now().UTC()
	if existing, lookupErr := s.repo.GetReusablePendingCheckout(ctx, vendorID, req.TierID, now); lookupErr == nil && existing != nil {
		return &CheckoutSession{Checkout: existing}, nil
	} else if lookupErr != nil && lookupErr != sql.ErrNoRows {
		return nil, lookupErr
	}

	checkoutID := uuid.New()
	checkout := &SubscriptionCheckout{
		ID: checkoutID, VendorID: vendorUUID, TierID: tier.ID, TierName: tier.Name,
		Amount: tier.MonthlyPrice, Currency: "ZMW", Reference: "SUB-" + checkoutID.String(),
		Status: CheckoutPending, ExpiresAt: now.Add(30 * time.Minute),
	}
	if err := s.repo.CreateCheckout(ctx, checkout); err != nil {
		return nil, err
	}
	created, err := s.repo.GetCheckoutByID(ctx, checkoutID.String())
	if err != nil {
		return nil, err
	}
	return &CheckoutSession{Checkout: created}, nil
}

func (s *service) GetSubscriptionCheckout(ctx context.Context, vendorID, checkoutID string) (*SubscriptionCheckout, error) {
	checkout, err := s.repo.GetCheckoutByID(ctx, checkoutID)
	if err != nil {
		return nil, err
	}
	if checkout.VendorID.String() != vendorID {
		return nil, fmt.Errorf("checkout is not accessible to this vendor")
	}
	return checkout, nil
}

func (s *service) InitiateSubscriptionMobileMoneyCollection(ctx context.Context, vendorID, checkoutID string, req InitiateMobileMoneyCollectionRequest) (*SubscriptionCheckout, error) {
	checkout, err := s.GetSubscriptionCheckout(ctx, vendorID, checkoutID)
	if err != nil {
		return nil, err
	}
	if checkout.Status != CheckoutPending {
		return checkout, nil
	}
	if !checkout.ExpiresAt.After(time.Now().UTC()) {
		if err := s.repo.MarkCheckoutFailed(ctx, checkout.ID.String(), "EXPIRED", "Checkout expired before collection could be initiated"); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("checkout has expired")
	}
	if s.checkout.Initiator == nil {
		return nil, fmt.Errorf("subscription payment collection is not configured")
	}
	if checkout.ProviderCollectionID != "" {
		return checkout, nil
	}

	phone := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(req.Phone))
	phone = strings.TrimPrefix(phone, "+")
	if len(phone) < 9 || len(phone) > 15 || strings.Trim(phone, "0123456789") != "" {
		return nil, fmt.Errorf("a valid mobile-money phone number is required")
	}
	operator := strings.ToLower(strings.TrimSpace(req.Operator))
	if operator != "airtel" && operator != "mtn" && operator != "zamtel" {
		return nil, fmt.Errorf("mobile-money operator must be airtel, mtn, or zamtel")
	}

	collection, err := s.checkout.Initiator.InitiateMobileMoneyCollection(ctx, MobileMoneyCollectionRequest{
		Amount: checkout.Amount, Currency: checkout.Currency, Reference: checkout.Reference,
		Phone: phone, Operator: operator, Country: "zm", Bearer: "merchant",
	})
	if err != nil {
		return nil, err
	}
	if collection == nil || collection.ID == "" || collection.Reference != checkout.Reference || math.Abs(collection.Amount-checkout.Amount) > 0.005 || !strings.EqualFold(collection.Currency, checkout.Currency) {
		return nil, fmt.Errorf("payment collection did not match checkout")
	}
	if err := s.repo.RecordCheckoutCollection(ctx, checkout.ID.String(), collection.ID, collection.Status); err != nil {
		return nil, err
	}
	if strings.EqualFold(collection.Status, "successful") || strings.EqualFold(collection.Status, "failed") {
		return s.VerifySubscriptionCheckout(ctx, vendorID, checkoutID)
	}
	return s.GetSubscriptionCheckout(ctx, vendorID, checkoutID)
}

func (s *service) VerifySubscriptionCheckoutByReference(ctx context.Context, reference string) (*SubscriptionCheckout, error) {
	checkout, err := s.repo.GetCheckoutByReference(ctx, reference)
	if err != nil {
		return nil, err
	}
	return s.VerifySubscriptionCheckout(ctx, checkout.VendorID.String(), checkout.ID.String())
}

func (s *service) VerifySubscriptionCheckout(ctx context.Context, vendorID, checkoutID string) (*SubscriptionCheckout, error) {
	checkout, err := s.GetSubscriptionCheckout(ctx, vendorID, checkoutID)
	if err != nil {
		return nil, err
	}
	if checkout.Status == CheckoutSuccessful || checkout.Status == CheckoutFailed || checkout.Status == CheckoutExpired {
		return checkout, nil
	}
	if !checkout.ExpiresAt.After(time.Now().UTC()) {
		if err := s.repo.MarkCheckoutFailed(ctx, checkoutID, "EXPIRED", "Checkout expired before payment could be verified"); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("checkout has expired")
	}
	if s.checkout.Verifier == nil {
		return nil, fmt.Errorf("subscription payment collection is not configured")
	}
	collection, err := s.checkout.Verifier.VerifyCollection(ctx, checkout.Reference)
	if err != nil {
		return nil, err
	}
	if collection == nil || collection.Reference != checkout.Reference || collection.ID == "" {
		return nil, fmt.Errorf("payment verification did not match checkout reference")
	}
	if math.Abs(collection.Amount-checkout.Amount) > 0.005 || !strings.EqualFold(collection.Currency, checkout.Currency) {
		return nil, fmt.Errorf("payment verification amount or currency did not match checkout")
	}
	switch strings.ToLower(collection.Status) {
	case "successful":
		return s.repo.ActivateCheckout(ctx, checkout.ID.String(), collection.ID, collection.Status, time.Now().UTC())
	case "failed":
		if err := s.repo.MarkCheckoutFailed(ctx, checkout.ID.String(), collection.Status, collection.Reason); err != nil {
			return nil, err
		}
		return s.repo.GetCheckoutByID(ctx, checkout.ID.String())
	default:
		return checkout, nil
	}
}

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
