package policyconsent

import (
	"context"
	"errors"
	"net"

	"github.com/google/uuid"
)

var (
	ErrAcceptanceUnavailable = errors.New("no published vendor policies are available for acceptance")
	ErrIncompleteAcceptance  = errors.New("all currently required vendor policies must be accepted together")
	ErrInvalidAcceptance     = errors.New("policy acceptance does not match the current published policy version")
)

type Service interface {
	GetStatus(ctx context.Context, userID string) (ConsentStatus, error)
	Accept(ctx context.Context, userID, vendorID string, requested []PolicyAcceptance, ip net.IP, userAgent, source string) (ConsentStatus, error)
	HasRequiredAcceptance(ctx context.Context, userID string) (bool, error)
	AttachAcceptedPoliciesToVendor(ctx context.Context, userID, vendorID string, ip net.IP, userAgent string) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetStatus(ctx context.Context, userID string) (ConsentStatus, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return ConsentStatus{}, err
	}
	policies, err := s.repo.ListPublishedVendorPolicies(ctx)
	if err != nil {
		return ConsentStatus{}, err
	}
	acceptedIDs, err := s.repo.ListAcceptedPolicyIDs(ctx, userUUID)
	if err != nil {
		return ConsentStatus{}, err
	}
	accepted := policyIDsToSet(acceptedIDs)
	acceptedSlugs := make([]string, 0, len(acceptedIDs))
	for _, policy := range policies {
		if _, ok := accepted[policy.ID]; ok {
			acceptedSlugs = append(acceptedSlugs, policy.Slug)
		}
	}
	return ConsentStatus{
		RequiredPolicies:    policies,
		AcceptedPolicySlugs: acceptedSlugs,
		AcceptanceRequired:  len(missingPolicies(policies, acceptedIDs)) > 0,
	}, nil
}

func (s *service) HasRequiredAcceptance(ctx context.Context, userID string) (bool, error) {
	status, err := s.GetStatus(ctx, userID)
	if err != nil {
		return false, err
	}
	return !status.AcceptanceRequired, nil
}

func (s *service) AttachAcceptedPoliciesToVendor(ctx context.Context, userID, vendorID string, ip net.IP, userAgent string) error {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	vendorUUID, err := uuid.Parse(vendorID)
	if err != nil {
		return err
	}
	return s.repo.AttachToVendor(ctx, userUUID, vendorUUID, ip, userAgent)
}

func (s *service) Accept(ctx context.Context, userID, vendorID string, requested []PolicyAcceptance, ip net.IP, userAgent, source string) (ConsentStatus, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return ConsentStatus{}, err
	}
	vendorUUID := uuid.Nil
	if vendorID != "" {
		vendorUUID, err = uuid.Parse(vendorID)
		if err != nil {
			return ConsentStatus{}, err
		}
	}

	policies, err := s.repo.ListPublishedVendorPolicies(ctx)
	if err != nil {
		return ConsentStatus{}, err
	}
	if len(policies) == 0 {
		return ConsentStatus{}, ErrAcceptanceUnavailable
	}
	if len(requested) != len(policies) {
		return ConsentStatus{}, ErrIncompleteAcceptance
	}

	received := make(map[string]string, len(requested))
	for _, acceptance := range requested {
		if acceptance.Slug == "" || acceptance.Version == "" {
			return ConsentStatus{}, ErrInvalidAcceptance
		}
		if _, duplicate := received[acceptance.Slug]; duplicate {
			return ConsentStatus{}, ErrInvalidAcceptance
		}
		received[acceptance.Slug] = acceptance.Version
	}
	for _, policy := range policies {
		if received[policy.Slug] != policy.Version {
			return ConsentStatus{}, ErrInvalidAcceptance
		}
	}

	if err := s.repo.Accept(ctx, userUUID, vendorUUID, policies, ip, userAgent, source); err != nil {
		return ConsentStatus{}, err
	}
	return s.GetStatus(ctx, userID)
}
