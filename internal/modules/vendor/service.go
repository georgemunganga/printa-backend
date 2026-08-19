package vendor

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	OnboardVendor(ctx context.Context, ownerID, businessName, taxID string) (*Vendor, error)
	OnboardVendorWithFirstStore(ctx context.Context, ownerID, businessName, taxID string, firstStore FirstStoreInput) (*Vendor, error)
	GetVendor(ctx context.Context, ownerID string) (*Vendor, error)
}

type service struct {
	vendorRepo Repository
	tierRepo   TierRepository
}

func NewService(vendorRepo Repository, tierRepo TierRepository) Service {
	return &service{vendorRepo: vendorRepo, tierRepo: tierRepo}
}

func (s *service) OnboardVendor(ctx context.Context, ownerID, businessName, taxID string) (*Vendor, error) {
	if existing, err := s.vendorRepo.GetVendorByOwnerID(ctx, ownerID); err == nil {
		return existing, nil
	}

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
		BusinessName: strings.TrimSpace(businessName),
		TaxID:        strings.TrimSpace(taxID),
	}

	if vendor.BusinessName == "" {
		return nil, fmt.Errorf("business_name is required")
	}
	if err := s.vendorRepo.CreateVendor(ctx, vendor); err != nil {
		return nil, err
	}

	return vendor, nil
}

// OnboardVendorWithFirstStore is the vendor onboarding completion path. It
// requires a truthful physical store location and delegates the vendor/store
// write to one transaction so the platform cannot retain a profile without the
// first storefront or vice versa.
func (s *service) OnboardVendorWithFirstStore(ctx context.Context, ownerID, businessName, taxID string, firstStore FirstStoreInput) (*Vendor, error) {
	businessName = strings.TrimSpace(businessName)
	if businessName == "" {
		return nil, fmt.Errorf("business_name is required")
	}

	firstStore.Name = strings.TrimSpace(firstStore.Name)
	firstStore.Address = strings.TrimSpace(firstStore.Address)
	firstStore.City = strings.TrimSpace(firstStore.City)
	firstStore.Country = strings.TrimSpace(firstStore.Country)
	if firstStore.Name == "" || firstStore.Address == "" || firstStore.City == "" || firstStore.Country == "" {
		return nil, fmt.Errorf("store_name, store_address, store_city, and store_country are required for first-store onboarding")
	}
	if err := validateFirstStoreCoordinates(firstStore.Latitude, firstStore.Longitude); err != nil {
		return nil, err
	}
	if !isValidStaffPIN(firstStore.OwnerPIN) {
		return nil, fmt.Errorf("staff_pin must contain 4 to 6 digits for first-store onboarding")
	}
	pinHash, err := bcrypt.GenerateFromPassword([]byte(firstStore.OwnerPIN), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash initial staff PIN: %w", err)
	}
	firstStore.OwnerPIN = ""
	firstStore.OwnerPINHash = string(pinHash)

	parsedOwnerID, err := uuid.Parse(ownerID)
	if err != nil {
		return nil, fmt.Errorf("invalid owner_id: %w", err)
	}

	coreTier, err := s.tierRepo.GetTierByName(ctx, "CORE")
	if err != nil {
		return nil, err
	}

	return s.vendorRepo.EnsureVendorWithFirstStore(ctx, &Vendor{
		ID:           uuid.New(),
		OwnerID:      parsedOwnerID,
		TierID:       coreTier.ID,
		BusinessName: businessName,
		TaxID:        strings.TrimSpace(taxID),
	}, firstStore)
}

func (s *service) GetVendor(ctx context.Context, ownerID string) (*Vendor, error) {
	return s.vendorRepo.GetVendorByOwnerID(ctx, ownerID)
}

func isValidStaffPIN(pin string) bool {
	if len(pin) < 4 || len(pin) > 6 {
		return false
	}
	for _, char := range pin {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func validateFirstStoreCoordinates(latitude, longitude *float64) error {
	if (latitude == nil) != (longitude == nil) {
		return fmt.Errorf("store_latitude and store_longitude must be supplied together")
	}
	if latitude != nil && (*latitude < -90 || *latitude > 90) {
		return fmt.Errorf("store_latitude must be between -90 and 90")
	}
	if longitude != nil && (*longitude < -180 || *longitude > 180) {
		return fmt.Errorf("store_longitude must be between -180 and 180")
	}
	return nil
}
