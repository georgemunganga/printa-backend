package operatingstatus

import (
	"context"
	"testing"
	"time"
)

type fakeRepository struct {
	compliance   *Compliance
	subscription *Subscription
	activeGrace  *GracePeriod
	graceCreated bool
}

func (f *fakeRepository) GetCompliance(context.Context, string) (*Compliance, error) {
	return f.compliance, nil
}

func (f *fakeRepository) GetSubscription(context.Context, string) (*Subscription, error) {
	return f.subscription, nil
}

func (f *fakeRepository) GetActiveGrace(_ context.Context, _ string, _ time.Time) (*GracePeriod, error) {
	return f.activeGrace, nil
}

func (f *fakeRepository) CreateGraceIfEligible(_ context.Context, _ string, _ string, subscription *Subscription, now time.Time) (*GracePeriod, bool, error) {
	if subscription == nil || subscription.Status != SubscriptionPastDue || !now.After(subscription.CurrentPeriodEnd) {
		return nil, false, nil
	}
	if f.activeGrace != nil {
		return f.activeGrace, false, nil
	}
	f.graceCreated = true
	f.activeGrace = &GracePeriod{ID: "grace-1", Status: "ACTIVE", EndsAt: now.AddDate(0, 0, 5)}
	return f.activeGrace, true, nil
}

func (f *fakeRepository) UpdateCompliance(context.Context, string, string, ComplianceStatus, string, time.Time) (*Compliance, error) {
	return f.compliance, nil
}

func TestPendingComplianceAlwaysBlocksVendorOperations(t *testing.T) {
	repo := &fakeRepository{compliance: &Compliance{Status: CompliancePending}}
	service := NewService(repo).(*service)
	service.now = func() time.Time { return time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC) }

	status, err := service.GetStatus(context.Background(), "vendor-1")
	if err != nil {
		t.Fatalf("GetStatus returned error: %v", err)
	}
	if status.Operational {
		t.Fatal("pending compliance must block operations")
	}
	if len(status.BlockingReasons) != 2 || status.BlockingReasons[0] != BlockCompliancePending || status.BlockingReasons[1] != BlockSubscriptionAbsent {
		t.Fatalf("unexpected blocking reasons: %#v", status.BlockingReasons)
	}
}

func TestPastDueSubscriptionCanReceiveExactlyOneGraceForCurrentPeriod(t *testing.T) {
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		compliance: &Compliance{Status: ComplianceApproved},
		subscription: &Subscription{
			ID:               "b28eaab4-a038-4732-b920-f327ee6dced8",
			Status:           SubscriptionPastDue,
			CurrentPeriodEnd: now.Add(-time.Hour),
		},
	}
	service := NewService(repo).(*service)
	service.now = func() time.Time { return now }

	before, err := service.GetStatus(context.Background(), "vendor-1")
	if err != nil {
		t.Fatalf("GetStatus returned error: %v", err)
	}
	if before.Operational || !before.GraceEligible || before.BlockingReasons[0] != BlockSubscriptionDue {
		t.Fatalf("expected past-due subscription block with grace eligibility, got %#v", before)
	}

	first, err := service.RequestGrace(context.Background(), "vendor-1", "user-1")
	if err != nil {
		t.Fatalf("first grace request returned error: %v", err)
	}
	if !first.Granted || !first.Status.Operational {
		t.Fatalf("expected first grace request to unlock subscription, got %#v", first)
	}
	if !repo.graceCreated {
		t.Fatal("expected grace record creation")
	}

	second, err := service.RequestGrace(context.Background(), "vendor-1", "user-1")
	if err != nil {
		t.Fatalf("second grace request returned error: %v", err)
	}
	if second.Granted {
		t.Fatal("an active grace period must not be granted twice")
	}
}
