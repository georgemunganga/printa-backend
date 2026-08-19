package operatingstatus

import (
	"context"
	"fmt"
	"time"
)

type Service interface {
	GetStatus(ctx context.Context, vendorID string) (*OperatingStatus, error)
	RequestGrace(ctx context.Context, vendorID, userID string) (*GraceRequestResult, error)
	DecideCompliance(ctx context.Context, vendorID, reviewerID string, req ComplianceDecisionRequest) (*Compliance, error)
}

type service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) Service {
	return &service{repo: repo, now: time.Now}
}

func (s *service) GetStatus(ctx context.Context, vendorID string) (*OperatingStatus, error) {
	return s.getStatusAt(ctx, vendorID, s.now())
}

func (s *service) RequestGrace(ctx context.Context, vendorID, userID string) (*GraceRequestResult, error) {
	now := s.now()
	status, err := s.getStatusAt(ctx, vendorID, now)
	if err != nil {
		return nil, err
	}
	grace, granted, err := s.repo.CreateGraceIfEligible(ctx, vendorID, userID, status.Subscription, now)
	if err != nil {
		return nil, err
	}
	if grace != nil {
		status.GracePeriod = grace
	}
	if granted {
		status, err = s.getStatusAt(ctx, vendorID, now)
		if err != nil {
			return nil, err
		}
	}
	return &GraceRequestResult{Status: *status, Granted: granted}, nil
}

func (s *service) DecideCompliance(ctx context.Context, vendorID, reviewerID string, req ComplianceDecisionRequest) (*Compliance, error) {
	status := ComplianceStatus(req.Status)
	if status != ComplianceApproved && status != ComplianceRejected {
		return nil, fmt.Errorf("status must be APPROVED or REJECTED")
	}
	if status == ComplianceRejected && req.Reason == "" {
		return nil, fmt.Errorf("reason is required when rejecting compliance approval")
	}
	return s.repo.UpdateCompliance(ctx, vendorID, reviewerID, status, req.Reason, s.now())
}

func (s *service) getStatusAt(ctx context.Context, vendorID string, now time.Time) (*OperatingStatus, error) {
	compliance, err := s.repo.GetCompliance(ctx, vendorID)
	if err != nil {
		return nil, err
	}
	subscription, err := s.repo.GetSubscription(ctx, vendorID)
	if err != nil {
		return nil, err
	}
	grace, err := s.repo.GetActiveGrace(ctx, vendorID, now)
	if err != nil {
		return nil, err
	}

	status := &OperatingStatus{
		VendorID:        vendorID,
		Operational:     true,
		BlockingReasons: make([]BlockReason, 0, 2),
		Compliance:      *compliance,
		Subscription:    subscription,
		GracePeriod:     grace,
		Payment: PaymentAction{
			Available: false,
			Message:   "Subscription payment collection is not yet available in Printa. Contact support to arrange payment.",
		},
	}

	switch compliance.Status {
	case ComplianceApproved:
	case ComplianceRejected:
		status.Operational = false
		status.BlockingReasons = append(status.BlockingReasons, BlockComplianceRejected)
	default:
		status.Operational = false
		status.BlockingReasons = append(status.BlockingReasons, BlockCompliancePending)
	}

	if subscription == nil {
		status.Operational = false
		status.BlockingReasons = append(status.BlockingReasons, BlockSubscriptionAbsent)
		return status, nil
	}

	subscriptionCurrent := subscriptionIsCurrent(subscription, now)
	if !subscriptionCurrent {
		if grace != nil && grace.EndsAt.After(now) {
			status.GraceEligible = false
			return status, nil
		}
		status.Operational = false
		if subscription.Status == SubscriptionPastDue {
			status.BlockingReasons = append(status.BlockingReasons, BlockSubscriptionDue)
			status.GraceEligible = now.After(subscription.CurrentPeriodEnd)
		} else {
			status.BlockingReasons = append(status.BlockingReasons, BlockSubscriptionEnded)
		}
	}
	return status, nil
}

func subscriptionIsCurrent(subscription *Subscription, now time.Time) bool {
	switch subscription.Status {
	case SubscriptionActive:
		return subscription.CurrentPeriodEnd.After(now)
	case SubscriptionTrial:
		if subscription.TrialEndsAt != nil {
			return subscription.TrialEndsAt.After(now)
		}
		return subscription.CurrentPeriodEnd.After(now)
	default:
		return false
	}
}

func ValidateStatus(status *OperatingStatus) error {
	if status == nil {
		return fmt.Errorf("operating status is required")
	}
	return nil
}
