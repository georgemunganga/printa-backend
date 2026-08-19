package billing

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

type tierCatalogueRepositoryStub struct {
	Repository
	tiers []*VendorTier
	err   error
}

func (s tierCatalogueRepositoryStub) ListTiers(context.Context) ([]*VendorTier, error) {
	return s.tiers, s.err
}

type tierCatalogueServiceStub struct {
	Service
	tiers []*VendorTier
	err   error
}

func (s tierCatalogueServiceStub) ListTiers(context.Context) ([]*VendorTier, error) {
	return s.tiers, s.err
}

func TestListTiersReturnsRepositoryCatalogue(t *testing.T) {
	expected := []*VendorTier{{
		ID:           uuid.New(),
		Name:         "CORE",
		MonthlyPrice: 250,
		Description:  "For getting started",
		IsAvailable:  true,
		Features:     []TierFeature{{Text: "20 jobs/day", Included: true}},
	}}

	svc := NewService(tierCatalogueRepositoryStub{tiers: expected})
	actual, err := svc.ListTiers(context.Background())
	if err != nil {
		t.Fatalf("ListTiers() error = %v", err)
	}
	if len(actual) != 1 || actual[0].Name != "CORE" || actual[0].MonthlyPrice != 250 {
		t.Fatalf("ListTiers() = %#v, want CORE at K250", actual)
	}
}

func TestTierCatalogueHandlerReturnsDatabaseBackedTiers(t *testing.T) {
	h := NewHandler(tierCatalogueServiceStub{tiers: []*VendorTier{{
		ID:           uuid.New(),
		Name:         "PRO",
		MonthlyPrice: 500,
		IsPopular:    true,
	}}}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/billing/tiers", nil)
	rec := httptest.NewRecorder()

	h.listTiers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body []VendorTier
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 1 || body[0].Name != "PRO" || body[0].MonthlyPrice != 500 || !body[0].IsPopular {
		t.Fatalf("response = %#v, want PRO at K500", body)
	}
}

func TestTierCatalogueHandlerDoesNotLeakStorageErrors(t *testing.T) {
	h := NewHandler(tierCatalogueServiceStub{err: errors.New("database unavailable")}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/billing/tiers", nil)
	rec := httptest.NewRecorder()

	h.listTiers(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if rec.Body.String() == "" || rec.Body.String() == "database unavailable" {
		t.Fatalf("response leaked storage error: %q", rec.Body.String())
	}
}
