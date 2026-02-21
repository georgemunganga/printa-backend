package production

import (
	"time"

	"github.com/google/uuid"
)

// JobStatus represents the lifecycle state of a production job.
type JobStatus string

const (
	JobQueued     JobStatus = "QUEUED"
	JobInProgress JobStatus = "IN_PROGRESS"
	JobOnHold     JobStatus = "ON_HOLD"
	JobCompleted  JobStatus = "COMPLETED"
	JobCancelled  JobStatus = "CANCELLED"
)

// validTransitions defines the allowed state machine transitions for production jobs.
var validTransitions = map[JobStatus][]JobStatus{
	JobQueued:     {JobInProgress, JobCancelled},
	JobInProgress: {JobOnHold, JobCompleted, JobCancelled},
	JobOnHold:     {JobInProgress, JobCancelled},
	JobCompleted:  {},
	JobCancelled:  {},
}

// CanTransition returns true if the transition from current to next is valid.
func CanTransition(current, next JobStatus) bool {
	allowed, ok := validTransitions[current]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == next {
			return true
		}
	}
	return false
}

// ProductionJob represents a print/production job in the queue.
type ProductionJob struct {
	ID          uuid.UUID  `json:"id"`
	OrderID     uuid.UUID  `json:"order_id"`
	StoreID     uuid.UUID  `json:"store_id"`
	AssignedTo  *uuid.UUID `json:"assigned_to,omitempty"`
	Status      JobStatus  `json:"status"`
	Priority    int        `json:"priority"`
	Notes       string     `json:"notes,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DueAt       *time.Time `json:"due_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CreateJobRequest is the payload for creating a new production job.
type CreateJobRequest struct {
	OrderID    string `json:"order_id"`
	StoreID    string `json:"store_id"`
	AssignedTo string `json:"assigned_to,omitempty"`
	Priority   int    `json:"priority,omitempty"`
	Notes      string `json:"notes,omitempty"`
	DueAt      string `json:"due_at,omitempty"`
}

// UpdateStatusRequest is the payload for advancing a job's status.
type UpdateStatusRequest struct {
	Status string `json:"status"`
	Notes  string `json:"notes,omitempty"`
}

// AssignRequest is the payload for assigning a job to a staff member.
type AssignRequest struct {
	UserID string `json:"user_id"`
}
