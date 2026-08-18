package operatinghours

import (
	"context"
	"fmt"
	"time"
)

// Service enforces the store operating-hours invariant before persistence.
type Service interface {
	List(ctx context.Context, storeID string) ([]OperatingHour, error)
	Replace(ctx context.Context, storeID string, req ReplaceRequest) ([]OperatingHour, error)
}

type service struct {
	repository Repository
}

func NewService(repository Repository) Service {
	return &service{repository: repository}
}

func (s *service) List(ctx context.Context, storeID string) ([]OperatingHour, error) {
	return s.repository.List(ctx, storeID)
}

func (s *service) Replace(ctx context.Context, storeID string, req ReplaceRequest) ([]OperatingHour, error) {
	if err := validateHours(req.Hours); err != nil {
		return nil, err
	}
	if err := s.repository.Replace(ctx, storeID, req.Hours); err != nil {
		return nil, err
	}
	return s.repository.List(ctx, storeID)
}

func validateHours(hours []OperatingHour) error {
	if len(hours) != 7 {
		return fmt.Errorf("exactly seven weekday entries are required")
	}

	seen := make(map[int]bool, 7)
	for _, hour := range hours {
		if hour.DayOfWeek < 0 || hour.DayOfWeek > 6 || seen[hour.DayOfWeek] {
			return fmt.Errorf("each weekday from 0 through 6 must appear exactly once")
		}
		seen[hour.DayOfWeek] = true

		if !hour.IsOpen {
			if hour.OpensAt != "" || hour.ClosesAt != "" {
				return fmt.Errorf("closed weekdays must not include opening or closing times")
			}
			continue
		}

		opensAt, err := time.Parse("15:04", hour.OpensAt)
		if err != nil {
			return fmt.Errorf("opening time for weekday %d must use HH:MM", hour.DayOfWeek)
		}
		closesAt, err := time.Parse("15:04", hour.ClosesAt)
		if err != nil {
			return fmt.Errorf("closing time for weekday %d must use HH:MM", hour.DayOfWeek)
		}
		if !opensAt.Before(closesAt) {
			return fmt.Errorf("opening time must be earlier than closing time for weekday %d", hour.DayOfWeek)
		}
	}
	return nil
}
