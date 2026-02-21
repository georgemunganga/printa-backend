package vendor

import (
	"context"

	"github.com/google/uuid"
)

type Service interface {
	OnboardVendor(ctx context.Context, ownerID, businessName, taxID string) (*Vendor, error)
	GetVendor(ctx context.Context, ownerID string) (*Vendor, error)
}

type service struct {
	vendorRepo Repository
	tierRepo  TierRepository
}

func NewService(vendorRepo Repository, tierRepo TierRepository) Service {
	return &service{vendorRepo: vendorRepo, tierRepo: tierRepo}
}

func (s *service) OnboardVendor(ctx context.Context, ownerID, businessName, taxID string) (*Vendor, error) {
	coreTier, err := s.tierRepo.GetTierByName(ctx, "CORE")
	if err != nil {
		return nil, err
	}

	parsedOwnerID, err := uuid.Parse(ownerID)
	if err != nil {
		return nil, err
	}

	vendor := &Vendor{
		ID:           uuid.New(),
		OwnerID:      parsedOwnerID,
		TierID:       coreTier.ID,
		BusinessName: businessName,
		TaxID:        taxID,
	}

	if err := s.vendorRepo.CreateVendor(ctx, vendor); err != nil {
		return nil, err
	}

	return vendor, nil
}

func (s *service) GetVendor(ctx context.Context, ownerID string) (*Vendor, error) {
	return s.vendorRepo.GetVendorByOwnerID(ctx, ownerID)
}
