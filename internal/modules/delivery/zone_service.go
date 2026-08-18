package delivery

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ZoneService defines vendor delivery-zone management and storefront coverage lookup.
type ZoneService interface {
	ListZones(ctx context.Context, storeID string) ([]*Zone, error)
	CreateZone(ctx context.Context, storeID string, req UpsertZoneRequest) (*Zone, error)
	UpdateZone(ctx context.Context, id, storeID string, req UpsertZoneRequest) (*Zone, error)
	DeleteZone(ctx context.Context, id, storeID string) error
	CheckEligibility(ctx context.Context, storeID string, req EligibilityRequest) (*EligibilityResponse, error)
}

type zoneService struct {
	repo ZoneRepository
}

func NewZoneService(repo ZoneRepository) ZoneService {
	return &zoneService{repo: repo}
}

func (s *zoneService) ListZones(ctx context.Context, storeID string) ([]*Zone, error) {
	if _, err := uuid.Parse(storeID); err != nil {
		return nil, fmt.Errorf("invalid store id: %w", err)
	}
	return s.repo.ListByStore(ctx, storeID)
}

func (s *zoneService) CreateZone(ctx context.Context, storeID string, req UpsertZoneRequest) (*Zone, error) {
	storeUUID, err := uuid.Parse(storeID)
	if err != nil {
		return nil, fmt.Errorf("invalid store id: %w", err)
	}
	if err := normalizeZoneRequest(&req); err != nil {
		return nil, err
	}
	zone := &Zone{
		ID:       uuid.New(),
		StoreID:  storeUUID,
		Name:     req.Name,
		City:     req.City,
		Country:  req.Country,
		IsActive: req.IsActive,
	}
	if err := s.repo.Create(ctx, zone); err != nil {
		return nil, err
	}
	return s.repo.GetByStore(ctx, zone.ID.String(), storeID)
}

func (s *zoneService) UpdateZone(ctx context.Context, id, storeID string, req UpsertZoneRequest) (*Zone, error) {
	if err := normalizeZoneRequest(&req); err != nil {
		return nil, err
	}
	zone, err := s.repo.GetByStore(ctx, id, storeID)
	if err != nil {
		return nil, err
	}
	zone.Name = req.Name
	zone.City = req.City
	zone.Country = req.Country
	zone.IsActive = req.IsActive
	if err := s.repo.Update(ctx, zone); err != nil {
		return nil, err
	}
	return s.repo.GetByStore(ctx, id, storeID)
}

func (s *zoneService) DeleteZone(ctx context.Context, id, storeID string) error {
	if _, err := s.repo.GetByStore(ctx, id, storeID); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id, storeID)
}

func (s *zoneService) CheckEligibility(ctx context.Context, storeID string, req EligibilityRequest) (*EligibilityResponse, error) {
	if _, err := uuid.Parse(storeID); err != nil {
		return nil, fmt.Errorf("invalid store id: %w", err)
	}
	req.City = strings.TrimSpace(req.City)
	req.Country = strings.TrimSpace(req.Country)
	if req.Country == "" {
		req.Country = "Zambia"
	}
	if req.City == "" {
		return nil, fmt.Errorf("city is required")
	}

	zone, err := s.repo.FindActiveByStoreCity(ctx, storeID, req.City, req.Country)
	if err == nil {
		return &EligibilityResponse{
			Eligible: true,
			Code:     "COVERED",
			Message:  "This store declares delivery coverage for the selected city.",
			Zone:     zone,
		}, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	hasZones, err := s.repo.HasAnyForStore(ctx, storeID)
	if err != nil {
		return nil, err
	}
	if !hasZones {
		return &EligibilityResponse{
			Eligible: false,
			Code:     "NOT_CONFIGURED",
			Message:  "This store has not configured delivery coverage yet.",
		}, nil
	}
	return &EligibilityResponse{
		Eligible: false,
		Code:     "CITY_NOT_COVERED",
		Message:  "This store does not declare delivery coverage for the selected city.",
	}, nil
}

func normalizeZoneRequest(req *UpsertZoneRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	req.City = strings.TrimSpace(req.City)
	req.Country = strings.TrimSpace(req.Country)
	if req.Country == "" {
		req.Country = "Zambia"
	}
	if req.Name == "" || req.City == "" {
		return fmt.Errorf("name and city are required")
	}
	return nil
}
