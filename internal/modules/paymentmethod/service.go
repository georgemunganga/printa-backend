package paymentmethod

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

var phonePattern = regexp.MustCompile(`^\+?[0-9]{9,15}$`)
var supportedProviders = map[string]bool{"MTN_MOMO": true, "AIRTEL_MONEY": true, "ZAMTEL_MONEY": true}

type Service interface {
	List(ctx context.Context, customerID string) ([]*Method, error)
	Create(ctx context.Context, customerID string, req UpsertRequest) (*Method, error)
	Update(ctx context.Context, id, customerID string, req UpsertRequest) (*Method, error)
	Delete(ctx context.Context, id, customerID string) error
	SetDefault(ctx context.Context, id, customerID string) (*Method, error)
}
type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func (s *service) List(ctx context.Context, customerID string) ([]*Method, error) {
	if _, err := uuid.Parse(customerID); err != nil {
		return nil, fmt.Errorf("invalid customer id")
	}
	return s.repo.List(ctx, customerID)
}
func (s *service) Create(ctx context.Context, customerID string, req UpsertRequest) (*Method, error) {
	uid, err := uuid.Parse(customerID)
	if err != nil {
		return nil, fmt.Errorf("invalid customer id")
	}
	if err = normalize(&req); err != nil {
		return nil, err
	}
	existing, err := s.repo.List(ctx, customerID)
	if err != nil {
		return nil, err
	}
	method := &Method{ID: uuid.New(), CustomerID: uid, Provider: req.Provider, PhoneNumber: req.PhoneNumber, Label: req.Label}
	if err = s.repo.Create(ctx, method); err != nil {
		return nil, paymentMethodWriteError(err)
	}
	if req.IsDefault || len(existing) == 0 {
		if err = s.repo.SetDefault(ctx, method.ID.String(), customerID); err != nil {
			return nil, err
		}
	}
	return s.repo.Get(ctx, method.ID.String(), customerID)
}
func (s *service) Update(ctx context.Context, id, customerID string, req UpsertRequest) (*Method, error) {
	if err := normalize(&req); err != nil {
		return nil, err
	}
	method, err := s.repo.Get(ctx, id, customerID)
	if err != nil {
		return nil, err
	}
	method.Provider, method.PhoneNumber, method.Label = req.Provider, req.PhoneNumber, req.Label
	if err = s.repo.Update(ctx, method); err != nil {
		return nil, paymentMethodWriteError(err)
	}
	if req.IsDefault {
		if err = s.repo.SetDefault(ctx, id, customerID); err != nil {
			return nil, err
		}
	}
	return s.repo.Get(ctx, id, customerID)
}
func (s *service) Delete(ctx context.Context, id, customerID string) error {
	method, err := s.repo.Get(ctx, id, customerID)
	if err != nil {
		return err
	}
	if err = s.repo.Delete(ctx, id, customerID); err != nil {
		return err
	}
	if method.IsDefault {
		return s.repo.PromoteEarliest(ctx, customerID)
	}
	return nil
}
func (s *service) SetDefault(ctx context.Context, id, customerID string) (*Method, error) {
	if _, err := s.repo.Get(ctx, id, customerID); err != nil {
		return nil, err
	}
	if err := s.repo.SetDefault(ctx, id, customerID); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id, customerID)
}
func normalize(req *UpsertRequest) error {
	req.Provider = strings.ToUpper(strings.TrimSpace(req.Provider))
	req.PhoneNumber = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(req.PhoneNumber))
	req.Label = strings.TrimSpace(req.Label)
	if !supportedProviders[req.Provider] {
		return fmt.Errorf("choose a supported mobile money provider")
	}
	if !phonePattern.MatchString(req.PhoneNumber) {
		return fmt.Errorf("enter a valid mobile money number")
	}
	if len(req.Label) > 60 {
		return fmt.Errorf("label must be 60 characters or fewer")
	}
	return nil
}

func paymentMethodWriteError(err error) error {
	if strings.Contains(err.Error(), "customer_payment_methods_customer_provider_phone_unique") {
		return fmt.Errorf("this mobile money number is already saved for that provider")
	}
	return err
}
