package policyconsent

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
)

type fakeRepository struct {
	policies    []Policy
	acceptedIDs []uuid.UUID
	accepted    bool
}

func (f *fakeRepository) ListPublishedVendorPolicies(_ context.Context) ([]Policy, error) {
	return f.policies, nil
}

func (f *fakeRepository) ListAcceptedPolicyIDs(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
	return f.acceptedIDs, nil
}

func (f *fakeRepository) Accept(_ context.Context, _ uuid.UUID, _ uuid.UUID, policies []Policy, _ net.IP, _ string, _ string) error {
	f.accepted = true
	f.acceptedIDs = make([]uuid.UUID, 0, len(policies))
	for _, policy := range policies {
		f.acceptedIDs = append(f.acceptedIDs, policy.ID)
	}
	return nil
}

func (f *fakeRepository) AttachToVendor(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ net.IP, _ string) error {
	return nil
}

func TestAcceptRequiresEveryCurrentPolicyVersion(t *testing.T) {
	terms := Policy{ID: uuid.New(), Slug: "vendor-terms", Version: "v1", Status: PolicyPublished, RequiredForVendor: true}
	privacy := Policy{ID: uuid.New(), Slug: "vendor-privacy-notice", Version: "v1", Status: PolicyPublished, RequiredForVendor: true}
	repo := &fakeRepository{policies: []Policy{terms, privacy}}
	service := NewService(repo)
	userID := uuid.New().String()

	if _, err := service.Accept(context.Background(), userID, "", []PolicyAcceptance{{Slug: terms.Slug, Version: terms.Version}}, nil, "test", "VENDOR_ONBOARDING"); err != ErrIncompleteAcceptance {
		t.Fatalf("expected incomplete acceptance error, got %v", err)
	}
	if repo.accepted {
		t.Fatal("repository must not persist an incomplete acceptance")
	}

	if _, err := service.Accept(context.Background(), userID, "", []PolicyAcceptance{
		{Slug: terms.Slug, Version: terms.Version},
		{Slug: privacy.Slug, Version: "wrong"},
	}, nil, "test", "VENDOR_ONBOARDING"); err != ErrInvalidAcceptance {
		t.Fatalf("expected invalid version error, got %v", err)
	}
}

func TestAcceptMakesCurrentPoliciesSatisfied(t *testing.T) {
	terms := Policy{ID: uuid.New(), Slug: "vendor-terms", Version: "v1", Status: PolicyPublished, RequiredForVendor: true}
	privacy := Policy{ID: uuid.New(), Slug: "vendor-privacy-notice", Version: "v1", Status: PolicyPublished, RequiredForVendor: true}
	repo := &fakeRepository{policies: []Policy{terms, privacy}}
	service := NewService(repo)
	userID := uuid.New().String()

	status, err := service.Accept(context.Background(), userID, "", []PolicyAcceptance{
		{Slug: privacy.Slug, Version: privacy.Version},
		{Slug: terms.Slug, Version: terms.Version},
	}, net.ParseIP("127.0.0.1"), "test", "VENDOR_ONBOARDING")
	if err != nil {
		t.Fatalf("accept returned error: %v", err)
	}
	if status.AcceptanceRequired {
		t.Fatal("expected acceptance to satisfy all current policies")
	}
	accepted, err := service.HasRequiredAcceptance(context.Background(), userID)
	if err != nil {
		t.Fatalf("status check returned error: %v", err)
	}
	if !accepted {
		t.Fatal("expected current policy acceptance")
	}
}
