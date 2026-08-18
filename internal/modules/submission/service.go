package submission

import (
	"context"
	"fmt"
	"strings"
)

type Service interface {
	CreateSupport(ctx context.Context, requesterUserID string, requesterRole RequesterRole, req CreateSupportRequest) (*Record, error)
	CreateFeedback(ctx context.Context, requesterUserID string, requesterRole RequesterRole, req CreateFeedbackRequest) (*Record, error)
	ListOwn(ctx context.Context, requesterUserID string, requesterRole RequesterRole) ([]Record, error)
	ListByRole(ctx context.Context, requesterRole RequesterRole) ([]Record, error)
}

type service struct{ repository Repository }

func NewService(repository Repository) Service { return &service{repository: repository} }

func (s *service) CreateSupport(ctx context.Context, requesterUserID string, requesterRole RequesterRole, req CreateSupportRequest) (*Record, error) {
	if !isAllowedRole(requesterRole) {
		return nil, fmt.Errorf("customer or vendor role is required")
	}
	if !isSupportTopic(req.Topic) {
		return nil, fmt.Errorf("unsupported support topic")
	}
	return s.create(ctx, CreateInput{RequesterUserID: requesterUserID, RequesterRole: requesterRole, SubmissionKind: KindSupport, Topic: req.Topic, Subject: req.Subject, Message: req.Message})
}

func (s *service) CreateFeedback(ctx context.Context, requesterUserID string, requesterRole RequesterRole, req CreateFeedbackRequest) (*Record, error) {
	if !isAllowedRole(requesterRole) {
		return nil, fmt.Errorf("customer or vendor role is required")
	}
	if !isFeedbackCategory(req.Category) {
		return nil, fmt.Errorf("unsupported feedback category")
	}
	return s.create(ctx, CreateInput{RequesterUserID: requesterUserID, RequesterRole: requesterRole, SubmissionKind: KindFeedback, Topic: req.Category, Subject: req.Subject, Message: req.Message})
}

func (s *service) create(ctx context.Context, input CreateInput) (*Record, error) {
	input.Topic, input.Subject, input.Message = strings.TrimSpace(input.Topic), strings.TrimSpace(input.Subject), strings.TrimSpace(input.Message)
	if len(input.Topic) < 2 || len(input.Topic) > 80 || len(input.Subject) < 2 || len(input.Subject) > 160 || len(input.Message) < 10 || len(input.Message) > 5000 {
		return nil, fmt.Errorf("submission fields have invalid lengths")
	}
	return s.repository.Create(ctx, input)
}

func (s *service) ListOwn(ctx context.Context, requesterUserID string, requesterRole RequesterRole) ([]Record, error) {
	return s.repository.ListForRequester(ctx, requesterUserID, requesterRole)
}
func (s *service) ListByRole(ctx context.Context, requesterRole RequesterRole) ([]Record, error) {
	return s.repository.ListForRole(ctx, requesterRole)
}

func isAllowedRole(role RequesterRole) bool {
	return role == RequesterRoleCustomer || role == RequesterRoleVendor
}
func isFeedbackCategory(value string) bool {
	return value == "feedback" || value == "feature" || value == "complaint" || value == "bug"
}
func isSupportTopic(value string) bool {
	switch value {
	case "Account & Login Issues", "Payment & Billing", "Orders & Delivery", "Technical Problems", "Team Management", "Store Setup", "Other":
		return true
	}
	return false
}
