package conversation

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

type Service interface {
	Send(ctx context.Context, orderID, senderID, body string) (*Message, error)
	List(ctx context.Context, orderID, readerID string) ([]*Message, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Send(ctx context.Context, orderID, senderID, body string) (*Message, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, errors.New("message body is required")
	}
	if len(body) > 5000 {
		return nil, errors.New("message body must not exceed 5000 characters")
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
	if err := s.repo.Create(ctx, message); err != nil {
		return nil, err
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
