package billing

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type checkoutRepositoryStub struct {
	Repository
	tier                   *VendorTier
	existing               *SubscriptionCheckout
	checkout               *SubscriptionCheckout
	activated              *SubscriptionCheckout
	createErr              error
	activateCalled         bool
	markFailedCalled       bool
	recordCollectionCalled bool
}

func (s *checkoutRepositoryStub) GetTierCatalogueEntry(context.Context, string) (*VendorTier, error) {
	if s.tier == nil {
		return nil, sql.ErrNoRows
	}
	return s.tier, nil
}

func (s *checkoutRepositoryStub) GetReusablePendingCheckout(context.Context, string, string, time.Time) (*SubscriptionCheckout, error) {
	if s.existing == nil {
		return nil, sql.ErrNoRows
	}
	return s.existing, nil
}

func (s *checkoutRepositoryStub) CreateCheckout(_ context.Context, checkout *SubscriptionCheckout) error {
	if s.createErr != nil {
		return s.createErr
	}
	copy := *checkout
	s.checkout = &copy
	return nil
}

func (s *checkoutRepositoryStub) GetCheckoutByID(context.Context, string) (*SubscriptionCheckout, error) {
	if s.checkout == nil {
		return nil, sql.ErrNoRows
	}
	return s.checkout, nil
}

func (s *checkoutRepositoryStub) GetCheckoutByReference(context.Context, string) (*SubscriptionCheckout, error) {
	return s.GetCheckoutByID(context.Background(), "")
}

func (s *checkoutRepositoryStub) ActivateCheckout(_ context.Context, _, providerCollectionID, providerStatus string, completedAt time.Time) (*SubscriptionCheckout, error) {
	s.activateCalled = true
	copy := *s.checkout
	copy.Status = CheckoutSuccessful
	copy.ProviderCollectionID = providerCollectionID
	copy.ProviderStatus = providerStatus
	copy.CompletedAt = &completedAt
	s.activated = &copy
	s.checkout = &copy
	return &copy, nil
}

func (s *checkoutRepositoryStub) RecordCheckoutCollection(_ context.Context, _ string, providerCollectionID, providerStatus string) error {
	s.recordCollectionCalled = true
	s.checkout.ProviderCollectionID = providerCollectionID
	s.checkout.ProviderStatus = providerStatus
	return nil
}

func (s *checkoutRepositoryStub) MarkCheckoutFailed(context.Context, string, string, string) error {
	s.markFailedCalled = true
	return nil
}

type collectionVerifierStub struct {
	collection *ProviderCollection
	err        error
	reference  string
}

func (s *collectionVerifierStub) VerifyCollection(_ context.Context, reference string) (*ProviderCollection, error) {
	s.reference = reference
	return s.collection, s.err
}

type collectionInitiatorStub struct {
	collection *ProviderCollection
	err        error
	request    MobileMoneyCollectionRequest
}

func (s *collectionInitiatorStub) InitiateMobileMoneyCollection(_ context.Context, request MobileMoneyCollectionRequest) (*ProviderCollection, error) {
	s.request = request
	return s.collection, s.err
}

func configuredCheckoutService(repo Repository, verifier CollectionVerifier, initiator CollectionInitiator) Service {
	return NewService(repo, WithCheckoutConfig(CheckoutConfig{Verifier: verifier, Initiator: initiator}))
}

func TestCreateSubscriptionCheckoutLocksDatabaseTierAmount(t *testing.T) {
	vendorID := uuid.New()
	tierID := uuid.New()
	repo := &checkoutRepositoryStub{tier: &VendorTier{ID: tierID, Name: "Core", MonthlyPrice: 250, IsAvailable: true}}
	svc := configuredCheckoutService(repo, &collectionVerifierStub{}, &collectionInitiatorStub{})

	session, err := svc.CreateSubscriptionCheckout(context.Background(), vendorID.String(), CreateCheckoutRequest{TierID: tierID.String()})
	if err != nil {
		t.Fatalf("CreateSubscriptionCheckout() error = %v", err)
	}
	if session.Checkout.Amount != 250 || session.Checkout.Currency != "ZMW" || session.Checkout.TierID != tierID {
		t.Fatalf("checkout = %#v, want database-backed Core amount K250", session.Checkout)
	}
	if !strings.HasPrefix(session.Checkout.Reference, "SUB-") {
		t.Fatalf("session exposes incorrect checkout reference: %#v", session)
	}
}

func TestCreateSubscriptionCheckoutRejectsUnavailableTier(t *testing.T) {
	repo := &checkoutRepositoryStub{tier: &VendorTier{ID: uuid.New(), Name: "Enterprise", MonthlyPrice: 1500, IsAvailable: false}}
	svc := configuredCheckoutService(repo, &collectionVerifierStub{}, &collectionInitiatorStub{})
	_, err := svc.CreateSubscriptionCheckout(context.Background(), uuid.New().String(), CreateCheckoutRequest{TierID: repo.tier.ID.String()})
	if err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("CreateSubscriptionCheckout() error = %v, want unavailable-tier rejection", err)
	}
}

func TestInitiateMobileMoneyCollectionUsesLockedCheckoutValues(t *testing.T) {
	vendorID := uuid.New()
	checkoutID := uuid.New()
	repo := &checkoutRepositoryStub{checkout: &SubscriptionCheckout{
		ID: checkoutID, VendorID: vendorID, TierID: uuid.New(), Amount: 500, Currency: "ZMW",
		Reference: "SUB-" + checkoutID.String(), Status: CheckoutPending, ExpiresAt: time.Now().Add(10 * time.Minute),
	}}
	initiator := &collectionInitiatorStub{collection: &ProviderCollection{
		ID: "collection-1", Reference: repo.checkout.Reference, Amount: 500, Currency: "ZMW", Status: "pay-offline",
	}}
	svc := configuredCheckoutService(repo, &collectionVerifierStub{}, initiator)

	checkout, err := svc.InitiateSubscriptionMobileMoneyCollection(context.Background(), vendorID.String(), checkoutID.String(), InitiateMobileMoneyCollectionRequest{Phone: "0977 433 571", Operator: "mtn"})
	if err != nil {
		t.Fatalf("InitiateSubscriptionMobileMoneyCollection() error = %v", err)
	}
	if initiator.request.Amount != 500 || initiator.request.Currency != "ZMW" || initiator.request.Reference != repo.checkout.Reference || initiator.request.Phone != "0977433571" || initiator.request.Operator != "mtn" {
		t.Fatalf("provider request = %#v, want database-locked checkout values plus normalized payer details", initiator.request)
	}
	if !repo.recordCollectionCalled || checkout.ProviderCollectionID != "collection-1" || checkout.Status != CheckoutPending {
		t.Fatalf("checkout = %#v; collection record persisted=%v", checkout, repo.recordCollectionCalled)
	}
}

func TestVerifySubscriptionCheckoutActivatesOnlyExactSuccessfulCollection(t *testing.T) {
	vendorID := uuid.New()
	checkoutID := uuid.New()
	repo := &checkoutRepositoryStub{checkout: &SubscriptionCheckout{
		ID: checkoutID, VendorID: vendorID, TierID: uuid.New(), TierName: "Core", Amount: 250, Currency: "ZMW",
		Reference: "SUB-" + checkoutID.String(), Status: CheckoutPending, ExpiresAt: time.Now().Add(10 * time.Minute),
	}}
	verifier := &collectionVerifierStub{collection: &ProviderCollection{
		ID: "collection-1", Reference: repo.checkout.Reference, Amount: 250, Currency: "ZMW", Status: "successful",
	}}
	svc := configuredCheckoutService(repo, verifier, &collectionInitiatorStub{})

	checkout, err := svc.VerifySubscriptionCheckout(context.Background(), vendorID.String(), checkoutID.String())
	if err != nil {
		t.Fatalf("VerifySubscriptionCheckout() error = %v", err)
	}
	if checkout.Status != CheckoutSuccessful || !repo.activateCalled || verifier.reference != repo.checkout.Reference {
		t.Fatalf("checkout = %#v; activate=%v; reference=%q", checkout, repo.activateCalled, verifier.reference)
	}
}

func TestVerifySubscriptionCheckoutRejectsAmountMismatchWithoutActivation(t *testing.T) {
	vendorID := uuid.New()
	checkoutID := uuid.New()
	repo := &checkoutRepositoryStub{checkout: &SubscriptionCheckout{
		ID: checkoutID, VendorID: vendorID, TierID: uuid.New(), Amount: 250, Currency: "ZMW",
		Reference: "SUB-" + checkoutID.String(), Status: CheckoutPending, ExpiresAt: time.Now().Add(10 * time.Minute),
	}}
	verifier := &collectionVerifierStub{collection: &ProviderCollection{
		ID: "collection-1", Reference: repo.checkout.Reference, Amount: 1, Currency: "ZMW", Status: "successful",
	}}
	svc := configuredCheckoutService(repo, verifier, &collectionInitiatorStub{})

	_, err := svc.VerifySubscriptionCheckout(context.Background(), vendorID.String(), checkoutID.String())
	if err == nil || !strings.Contains(err.Error(), "amount or currency") {
		t.Fatalf("VerifySubscriptionCheckout() error = %v, want amount mismatch", err)
	}
	if repo.activateCalled {
		t.Fatal("amount mismatch must not activate a subscription")
	}
}

func TestVerifySubscriptionCheckoutProviderFailureDoesNotActivate(t *testing.T) {
	vendorID := uuid.New()
	checkoutID := uuid.New()
	repo := &checkoutRepositoryStub{checkout: &SubscriptionCheckout{
		ID: checkoutID, VendorID: vendorID, TierID: uuid.New(), Amount: 250, Currency: "ZMW",
		Reference: "SUB-" + checkoutID.String(), Status: CheckoutPending, ExpiresAt: time.Now().Add(10 * time.Minute),
	}}
	verifier := &collectionVerifierStub{err: errors.New("provider unavailable")}
	svc := configuredCheckoutService(repo, verifier, &collectionInitiatorStub{})

	_, err := svc.VerifySubscriptionCheckout(context.Background(), vendorID.String(), checkoutID.String())
	if err == nil || repo.activateCalled {
		t.Fatalf("provider error must fail safely; err=%v activated=%v", err, repo.activateCalled)
	}
}
