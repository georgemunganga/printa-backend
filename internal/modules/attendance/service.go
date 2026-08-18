package attendance

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	SetPIN(ctx context.Context, storeID, userID, pin string) error
	Clock(ctx context.Context, storeID, userID, pin, createdBy string) (*ClockResponse, error)
	ListRecent(ctx context.Context, storeID string, limit int) ([]*AttendanceEvent, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) SetPIN(ctx context.Context, storeID, userID, pin string) error {
	if !isValidPIN(pin) {
		return errors.New("PIN must contain 4 to 6 digits")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash PIN: %w", err)
	}
	return s.repo.SetPIN(ctx, storeID, userID, string(hash))
}

func (s *service) Clock(ctx context.Context, storeID, userID, pin, createdBy string) (*ClockResponse, error) {
	if !isValidPIN(pin) {
		return nil, errors.New("PIN must contain 4 to 6 digits")
	}
	hash, err := s.repo.GetPINHash(ctx, storeID, userID)
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pin)); err != nil {
		return nil, errors.New("invalid staff PIN")
	}

	last, err := s.repo.GetLastEventType(ctx, storeID, userID)
	if err != nil {
		return nil, err
	}
	eventType := EventClockIn
	nextAction := EventClockOut
	if last != nil && *last == EventClockIn {
		eventType = EventClockOut
		nextAction = EventClockIn
	}

	storeUUID, staffUUID, err := parseIDs(storeID, userID)
	if err != nil {
		return nil, err
	}
	actorUUID, err := uuid.Parse(createdBy)
	if err != nil {
		return nil, fmt.Errorf("invalid authenticated user ID: %w", err)
	}
	event := &AttendanceEvent{
		ID:        uuid.New(),
		StoreID:   storeUUID,
		UserID:    staffUUID,
		EventType: eventType,
		CreatedBy: &actorUUID,
	}
	if err := s.repo.CreateEvent(ctx, event); err != nil {
		return nil, err
	}
	return &ClockResponse{Event: *event, NextAction: nextAction}, nil
}

func (s *service) ListRecent(ctx context.Context, storeID string, limit int) ([]*AttendanceEvent, error) {
	return s.repo.ListRecent(ctx, storeID, limit)
}
