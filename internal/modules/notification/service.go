package notification

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/georgemunganga/printa-backend/internal/outbox"
	"github.com/google/uuid"
)

// Service is the core notification engine interface.
type Service interface {
	// Create stores a single notification for a recipient.
	Create(ctx context.Context, req CreateRequest) (*Notification, error)

	// BulkCreate stores the same notification for multiple recipients (e.g. broadcast to all store staff).
	BulkCreate(ctx context.Context, req BulkCreateRequest) error

	// Dispatch is the primary entry point for domain events from other modules.
	// It creates a notification record and (in Phase D) hands off to the comms module.
	Dispatch(ctx context.Context, event Event) error

	// GetByID retrieves a single notification.
	GetByID(ctx context.Context, id string) (*Notification, error)

	// List returns paginated notifications for a recipient with optional filters.
	List(ctx context.Context, filter ListFilter) ([]*Notification, int, error)

	// MarkRead marks a single notification as read.
	MarkRead(ctx context.Context, id, recipientID string) error

	// MarkAllRead marks all unread notifications for a recipient as read.
	MarkAllRead(ctx context.Context, recipientID string) error

	// Dismiss marks a notification as dismissed (hidden from inbox but not deleted).
	Dismiss(ctx context.Context, id, recipientID string) error

	// Delete permanently removes a notification.
	Delete(ctx context.Context, id, recipientID string) error

	// GetUnreadCount returns the badge count for a recipient.
	GetUnreadCount(ctx context.Context, recipientID string) (int, error)
}

// ChannelDispatcher remains the stable adapter contract used by the communications module.
// Notification delivery now reaches this boundary from the durable worker.
type ChannelDispatcher interface {
	Send(ctx context.Context, event Event) error
}

type service struct {
	repo   Repository
	outbox *outbox.Repository
}

// NewService creates notification records and records external-delivery work in
// the durable outbox. The separately deployed worker owns the side effect.
func NewService(repo Repository, outboxRepository *outbox.Repository) Service {
	return &service{repo: repo, outbox: outboxRepository}
}

func (s *service) Create(ctx context.Context, req CreateRequest) (*Notification, error) {
	if req.RecipientID == "" {
		return nil, fmt.Errorf("recipient_id is required")
	}
	if req.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if req.Priority == "" {
		req.Priority = PriorityNormal
	}
	n := &Notification{
		RecipientID: req.RecipientID,
		Type:        req.Type,
		Title:       req.Title,
		Body:        req.Body,
		Priority:    req.Priority,
		Metadata:    req.Metadata,
	}
	if err := s.repo.Create(ctx, n); err != nil {
		return nil, fmt.Errorf("create notification: %w", err)
	}
	return n, nil
}

func (s *service) BulkCreate(ctx context.Context, req BulkCreateRequest) error {
	if len(req.RecipientIDs) == 0 {
		return fmt.Errorf("at least one recipient_id is required")
	}
	if req.Priority == "" {
		req.Priority = PriorityNormal
	}
	var notifications []*Notification
	for _, rid := range req.RecipientIDs {
		notifications = append(notifications, &Notification{
			RecipientID: rid,
			Type:        req.Type,
			Title:       req.Title,
			Body:        req.Body,
			Priority:    req.Priority,
			Metadata:    req.Metadata,
		})
	}
	return s.repo.BulkCreate(ctx, notifications)
}

func (s *service) Dispatch(ctx context.Context, event Event) error {
	if event.Priority == "" {
		event.Priority = PriorityNormal
	}
	// 1. Persist the notification record
	n := &Notification{
		RecipientID: event.RecipientID,
		Type:        event.Type,
		Title:       event.Title,
		Body:        event.Body,
		Priority:    event.Priority,
		Metadata:    event.Metadata,
	}
	if err := s.repo.Create(ctx, n); err != nil {
		return fmt.Errorf("dispatch notification: %w", err)
	}
	// 2. Record delivery work for the separately supervised worker. Events are
	// never sent in a fire-and-forget goroutine from the API process.
	if s.outbox == nil {
		return fmt.Errorf("notification outbox is not configured")
	}
	notificationID, err := uuid.Parse(n.ID)
	if err != nil {
		return fmt.Errorf("notification has invalid id: %w", err)
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal notification event: %w", err)
	}
	if _, err := s.outbox.Enqueue(ctx, outbox.Event{
		AggregateType: "notification",
		AggregateID:   notificationID,
		EventType:     "notification.dispatch.v1",
		Payload:       payload,
	}); err != nil {
		return fmt.Errorf("enqueue notification delivery: %w", err)
	}
	return nil
}

func (s *service) GetByID(ctx context.Context, id string) (*Notification, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *service) List(ctx context.Context, filter ListFilter) ([]*Notification, int, error) {
	return s.repo.List(ctx, filter)
}

func (s *service) MarkRead(ctx context.Context, id, recipientID string) error {
	return s.repo.MarkRead(ctx, id, recipientID)
}

func (s *service) MarkAllRead(ctx context.Context, recipientID string) error {
	return s.repo.MarkAllRead(ctx, recipientID)
}

func (s *service) Dismiss(ctx context.Context, id, recipientID string) error {
	return s.repo.Dismiss(ctx, id, recipientID)
}

func (s *service) Delete(ctx context.Context, id, recipientID string) error {
	return s.repo.Delete(ctx, id, recipientID)
}

func (s *service) GetUnreadCount(ctx context.Context, recipientID string) (int, error) {
	return s.repo.GetUnreadCount(ctx, recipientID)
}
