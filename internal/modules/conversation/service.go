package conversation

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

type Service interface {
	Send(ctx context.Context, orderID, senderID, body string, assetIDStrings []string) (*Message, error)
	List(ctx context.Context, orderID, readerID string) ([]*Message, error)
	GetAttachment(ctx context.Context, orderID, messageID, assetID string) (*Attachment, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Send(ctx context.Context, orderID, senderID, body string, assetIDStrings []string) (*Message, error) {
	body = strings.TrimSpace(body)
	if len(body) > 5000 {
		return nil, errors.New("message body must not exceed 5000 characters")
	}
	if len(assetIDStrings) > 8 {
		return nil, errors.New("a message may include at most 8 attachments")
	}
	assetIDs := make([]uuid.UUID, 0, len(assetIDStrings))
	seen := make(map[uuid.UUID]struct{})
	for _, rawID := range assetIDStrings {
		assetID, err := uuid.Parse(strings.TrimSpace(rawID))
		if err != nil {
			return nil, errors.New("invalid attachment asset ID")
		}
		if _, exists := seen[assetID]; exists {
			return nil, errors.New("duplicate attachment asset ID")
		}
		seen[assetID] = struct{}{}
		assetIDs = append(assetIDs, assetID)
	}
	if body == "" && len(assetIDs) == 0 {
		return nil, errors.New("message body or an attachment is required")
	}
	orderUUID, err := uuid.Parse(orderID)
	if err != nil {
		return nil, errors.New("invalid order ID")
	}
	senderUUID, err := uuid.Parse(senderID)
	if err != nil {
		return nil, errors.New("invalid authenticated user ID")
	}
	message := &Message{ID: uuid.New(), OrderID: orderUUID, SenderID: senderUUID, Body: body}
	if err := s.repo.Create(ctx, message, assetIDs); err != nil {
		return nil, err
	}
	for _, assetID := range assetIDs {
		message.Attachments = append(message.Attachments, &Attachment{AssetID: assetID})
	}
	return message, nil
}

func (s *service) List(ctx context.Context, orderID, readerID string) ([]*Message, error) {
	messages, err := s.repo.ListByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.MarkReadByOrder(ctx, orderID, readerID); err != nil {
		return nil, err
	}
	if messages == nil {
		messages = make([]*Message, 0)
	}
	return messages, nil
}

func (s *service) GetAttachment(ctx context.Context, orderID, messageID, assetID string) (*Attachment, error) {
	return s.repo.GetAttachment(ctx, orderID, messageID, assetID)
}
