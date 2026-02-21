package production

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service defines production job business logic.
type Service interface {
	CreateJob(ctx context.Context, req CreateJobRequest) (*ProductionJob, error)
	GetJob(ctx context.Context, id string) (*ProductionJob, error)
	GetJobByOrder(ctx context.Context, orderID string) (*ProductionJob, error)
	ListStoreJobs(ctx context.Context, storeID string, status string) ([]*ProductionJob, error)
	ListMyJobs(ctx context.Context, userID string) ([]*ProductionJob, error)
	UpdateStatus(ctx context.Context, id string, req UpdateStatusRequest) (*ProductionJob, error)
	AssignJob(ctx context.Context, id string, req AssignRequest) (*ProductionJob, error)
	QueueDepth(ctx context.Context, storeID string) (int, error)
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func (s *service) CreateJob(ctx context.Context, req CreateJobRequest) (*ProductionJob, error) {
	if req.OrderID == "" {
		return nil, fmt.Errorf("order_id is required")
	}
	if req.StoreID == "" {
		return nil, fmt.Errorf("store_id is required")
	}

	priority := req.Priority
	if priority <= 0 {
		priority = 5 // NORMAL
	}

	job := &ProductionJob{
		ID:       uuid.New(),
		OrderID:  uuid.MustParse(req.OrderID),
		StoreID:  uuid.MustParse(req.StoreID),
		Status:   JobQueued,
		Priority: priority,
		Notes:    req.Notes,
	}

	if req.AssignedTo != "" {
		uid, err := uuid.Parse(req.AssignedTo)
		if err != nil {
			return nil, fmt.Errorf("invalid assigned_to: %w", err)
		}
		job.AssignedTo = &uid
	}

	if req.DueAt != "" {
		t, err := time.Parse(time.RFC3339, req.DueAt)
		if err != nil {
			return nil, fmt.Errorf("invalid due_at format, use RFC3339: %w", err)
		}
		job.DueAt = &t
	}

	if err := s.repo.Create(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *service) GetJob(ctx context.Context, id string) (*ProductionJob, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *service) GetJobByOrder(ctx context.Context, orderID string) (*ProductionJob, error) {
	return s.repo.GetByOrderID(ctx, orderID)
}

func (s *service) ListStoreJobs(ctx context.Context, storeID string, status string) ([]*ProductionJob, error) {
	return s.repo.ListByStore(ctx, storeID, strings.ToUpper(status))
}

func (s *service) ListMyJobs(ctx context.Context, userID string) ([]*ProductionJob, error) {
	return s.repo.ListByAssignee(ctx, userID)
}

func (s *service) UpdateStatus(ctx context.Context, id string, req UpdateStatusRequest) (*ProductionJob, error) {
	if req.Status == "" {
		return nil, fmt.Errorf("status is required")
	}

	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("job not found: %w", err)
	}

	next := JobStatus(strings.ToUpper(req.Status))
	if !CanTransition(job.Status, next) {
		return nil, fmt.Errorf("cannot transition job from %s to %s", job.Status, next)
	}

	if err := s.repo.UpdateStatus(ctx, id, next, req.Notes); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, id)
}

func (s *service) AssignJob(ctx context.Context, id string, req AssignRequest) (*ProductionJob, error) {
	if req.UserID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return nil, fmt.Errorf("job not found: %w", err)
	}
	if err := s.repo.UpdateAssignee(ctx, id, req.UserID); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *service) QueueDepth(ctx context.Context, storeID string) (int, error) {
	return s.repo.CountActiveByStore(ctx, storeID)
}
