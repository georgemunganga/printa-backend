package delivery

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Service defines customer delivery-location behavior.
type Service interface {
	ListLocations(ctx context.Context, customerID string) ([]*Location, error)
	CreateLocation(ctx context.Context, customerID string, req UpsertLocationRequest) (*Location, error)
	UpdateLocation(ctx context.Context, id, customerID string, req UpsertLocationRequest) (*Location, error)
	DeleteLocation(ctx context.Context, id, customerID string) error
	SetDefaultLocation(ctx context.Context, id, customerID string) (*Location, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) ListLocations(ctx context.Context, customerID string) ([]*Location, error) {
	if _, err := uuid.Parse(customerID); err != nil {
		return nil, fmt.Errorf("invalid customer id: %w", err)
	}
	return s.repo.ListByCustomer(ctx, customerID)
}

func (s *service) CreateLocation(ctx context.Context, customerID string, req UpsertLocationRequest) (*Location, error) {
	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		return nil, fmt.Errorf("invalid customer id: %w", err)
	}
	if err := validateRequest(&req); err != nil {
		return nil, err
	}

	existing, err := s.repo.ListByCustomer(ctx, customerID)
	if err != nil {
		return nil, err
	}
	makeDefault := req.IsDefault || len(existing) == 0
	location := &Location{
		ID:             uuid.New(),
		CustomerID:     customerUUID,
		Label:          req.Label,
		RecipientName:  req.RecipientName,
		RecipientPhone: req.RecipientPhone,
		AddressLine1:   req.AddressLine1,
		AddressLine2:   req.AddressLine2,
		City:           req.City,
		Country:        req.Country,
		Latitude:       req.Latitude,
		Longitude:      req.Longitude,
		IsDefault:      false,
	}
	if err := s.repo.Create(ctx, location); err != nil {
		return nil, err
	}
	if makeDefault {
		if err := s.repo.SetDefault(ctx, location.ID.String(), customerID); err != nil {
			return nil, err
		}
	}
	return s.repo.GetByCustomer(ctx, location.ID.String(), customerID)
}

func (s *service) UpdateLocation(ctx context.Context, id, customerID string, req UpsertLocationRequest) (*Location, error) {
	if err := validateRequest(&req); err != nil {
		return nil, err
	}
	location, err := s.repo.GetByCustomer(ctx, id, customerID)
	if err != nil {
		return nil, err
	}
	makeDefault := req.IsDefault
	location.Label = req.Label
	location.RecipientName = req.RecipientName
	location.RecipientPhone = req.RecipientPhone
	location.AddressLine1 = req.AddressLine1
	location.AddressLine2 = req.AddressLine2
	location.City = req.City
	location.Country = req.Country
	location.Latitude = req.Latitude
	location.Longitude = req.Longitude
	if err := s.repo.Update(ctx, location); err != nil {
		return nil, err
	}
	if makeDefault {
		if err := s.repo.SetDefault(ctx, id, customerID); err != nil {
			return nil, err
		}
	}
	return s.repo.GetByCustomer(ctx, id, customerID)
}

func (s *service) DeleteLocation(ctx context.Context, id, customerID string) error {
	location, err := s.repo.GetByCustomer(ctx, id, customerID)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id, customerID); err != nil {
		return err
	}
	if location.IsDefault {
		return s.repo.PromoteEarliestAsDefault(ctx, customerID)
	}
	return nil
}

func (s *service) SetDefaultLocation(ctx context.Context, id, customerID string) (*Location, error) {
	if _, err := s.repo.GetByCustomer(ctx, id, customerID); err != nil {
		return nil, err
	}
	if err := s.repo.SetDefault(ctx, id, customerID); err != nil {
		return nil, err
	}
	return s.repo.GetByCustomer(ctx, id, customerID)
}

func validateRequest(req *UpsertLocationRequest) error {
	req.Label = strings.TrimSpace(req.Label)
	req.RecipientName = strings.TrimSpace(req.RecipientName)
	req.RecipientPhone = strings.TrimSpace(req.RecipientPhone)
	req.AddressLine1 = strings.TrimSpace(req.AddressLine1)
	req.AddressLine2 = strings.TrimSpace(req.AddressLine2)
	req.City = strings.TrimSpace(req.City)
	req.Country = strings.TrimSpace(req.Country)
	if req.Country == "" {
		req.Country = "Zambia"
	}
	if req.Label == "" || req.RecipientName == "" || req.RecipientPhone == "" || req.AddressLine1 == "" || req.City == "" {
		return fmt.Errorf("label, recipient_name, recipient_phone, address_line1, and city are required")
	}
	if (req.Latitude == nil) != (req.Longitude == nil) {
		return fmt.Errorf("latitude and longitude must be supplied together")
	}
	if req.Latitude != nil && (*req.Latitude < -90 || *req.Latitude > 90) {
		return fmt.Errorf("latitude must be between -90 and 90")
	}
	if req.Longitude != nil && (*req.Longitude < -180 || *req.Longitude > 180) {
		return fmt.Errorf("longitude must be between -180 and 180")
	}
	return nil
}

func isNotFound(err error) bool {
	return err == sql.ErrNoRows
}
