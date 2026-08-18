package comms

import (
	"context"
	"fmt"

	"github.com/georgemunganga/printa-backend/internal/modules/notification"
)

// Service is the communications orchestrator.
// It routes messages to the correct adapter, persists delivery logs,
// and implements notification.ChannelDispatcher so it can be injected
// into the notification service.
type Service interface {
	// Send delivers a message through the specified channel and logs the attempt.
	Send(ctx context.Context, req SendRequest) (*SendResult, error)

	// Send implements notification.ChannelDispatcher.
	// It maps a notification.Event to the appropriate channel(s) and delivers externally.
	SendEvent(ctx context.Context, event notification.Event) error

	// GetLog retrieves a delivery log by ID.
	GetLog(ctx context.Context, id string) (*DeliveryLog, error)

	// ListLogs returns paginated delivery logs.
	ListLogs(ctx context.Context, filter ListFilter) ([]*DeliveryLog, int, error)
}

// Registry maps channel types to their adapters.
type Registry map[ChannelType]Adapter

type commsService struct {
	repo     Repository
	adapters Registry
}

// NewService creates the communications service with all adapters registered.
func NewService(repo Repository, adapters ...Adapter) Service {
	registry := make(Registry)
	for _, a := range adapters {
		registry[a.Channel()] = a
	}
	return &commsService{repo: repo, adapters: registry}
}

func (s *commsService) Send(ctx context.Context, req SendRequest) (*SendResult, error) {
	// Idempotency check
	if req.IdempotencyKey != "" {
		existing, err := s.repo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
		if err == nil && existing != nil {
			return &SendResult{
				LogID:       existing.ID,
				Channel:     existing.Channel,
				Status:      existing.Status,
				ProviderRef: existing.ProviderRef,
			}, nil
		}
	}

	// Create pending log
	log := &DeliveryLog{
		Channel:        req.Channel,
		Recipient:      req.Recipient,
		RecipientID:    req.RecipientID,
		Subject:        req.Subject,
		Body:           req.Body,
		Status:         DeliveryPending,
		IdempotencyKey: req.IdempotencyKey,
	}
	if err := s.repo.Create(ctx, log); err != nil {
		return nil, fmt.Errorf("create delivery log: %w", err)
	}

	// Find adapter
	adapter, ok := s.adapters[req.Channel]
	if !ok {
		_ = s.repo.UpdateStatus(ctx, log.ID, DeliveryFailed, "", fmt.Sprintf("no adapter for channel %s", req.Channel))
		return &SendResult{
			LogID:   log.ID,
			Channel: req.Channel,
			Status:  DeliveryFailed,
			Error:   fmt.Sprintf("no adapter registered for channel %s", req.Channel),
		}, nil
	}

	// Send
	msg := Message{
		Channel:        req.Channel,
		Recipient:      req.Recipient,
		RecipientID:    req.RecipientID,
		Subject:        req.Subject,
		Body:           req.Body,
		HTMLBody:       req.HTMLBody,
		IdempotencyKey: req.IdempotencyKey,
		TemplateID:     req.TemplateID,
	}
	// Convert map[string]string metadata to map[string]string (already correct type)
	if req.Metadata != nil {
		msg.Metadata = req.Metadata
	}

	providerRef, err := adapter.Send(ctx, msg)
	if err != nil {
		_ = s.repo.UpdateStatus(ctx, log.ID, DeliveryFailed, "", err.Error())
		return &SendResult{
			LogID:   log.ID,
			Channel: req.Channel,
			Status:  DeliveryFailed,
			Error:   err.Error(),
		}, nil
	}

	_ = s.repo.UpdateStatus(ctx, log.ID, DeliverySent, providerRef, "")
	return &SendResult{
		LogID:       log.ID,
		Channel:     req.Channel,
		Status:      DeliverySent,
		ProviderRef: providerRef,
	}, nil
}

// SendEvent implements notification.ChannelDispatcher.
// Maps notification events to the appropriate external channel(s).
func (s *commsService) SendEvent(ctx context.Context, event notification.Event) error {
	channels := s.resolveChannels(event)
	for _, ch := range channels {
		recipient := s.resolveRecipient(event, ch)
		if recipient == "" {
			continue // no contact info for this channel — skip silently
		}
		req := SendRequest{
			Channel:     ch,
			Recipient:   recipient,
			RecipientID: event.RecipientID,
			Subject:     event.Title,
			Body:        event.Body,
		}
		result, err := s.Send(ctx, req)
		if err != nil {
			return fmt.Errorf("send %s notification event: %w", ch, err)
		}
		if result.Status == DeliveryFailed {
			return fmt.Errorf("send %s notification event: %s", ch, result.Error)
		}
	}
	return nil
}

func (s *commsService) GetLog(ctx context.Context, id string) (*DeliveryLog, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *commsService) ListLogs(ctx context.Context, filter ListFilter) ([]*DeliveryLog, int, error) {
	return s.repo.List(ctx, filter)
}

// resolveChannels returns which external channels to use for a given event priority.
// Edit this method to change the delivery routing policy.
func (s *commsService) resolveChannels(event notification.Event) []ChannelType {
	switch string(event.Priority) {
	case "URGENT":
		return []ChannelType{ChannelSMS, ChannelPush, ChannelEmail}
	case "HIGH":
		return []ChannelType{ChannelPush, ChannelEmail}
	default:
		return []ChannelType{ChannelPush}
	}
}

// resolveRecipient looks up the contact address for a channel from event metadata.
// In production, metadata should contain email, phone, device_token as needed.
func (s *commsService) resolveRecipient(event notification.Event, ch ChannelType) string {
	if event.Metadata == nil {
		return ""
	}
	switch ch {
	case ChannelEmail:
		return event.Metadata["email"]
	case ChannelSMS, ChannelWhatsApp:
		return event.Metadata["phone"]
	case ChannelPush:
		return event.Metadata["device_token"]
	}
	return ""
}
