package submission

import "time"

type Kind string
type RequesterRole string

const (
	KindSupport  Kind = "SUPPORT"
	KindFeedback Kind = "FEEDBACK"

	RequesterRoleCustomer RequesterRole = "CUSTOMER"
	RequesterRoleVendor   RequesterRole = "VENDOR"
)

type Record struct {
	ID              string        `json:"id"`
	RequesterUserID string        `json:"requester_user_id"`
	RequesterRole   RequesterRole `json:"requester_role"`
	SubmissionKind  Kind          `json:"submission_kind"`
	Topic           string        `json:"topic"`
	Subject         string        `json:"subject"`
	Message         string        `json:"message"`
	Status          string        `json:"status"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

type CreateSupportRequest struct {
	Topic   string `json:"topic"`
	Subject string `json:"subject"`
	Message string `json:"message"`
}

type CreateFeedbackRequest struct {
	Category string `json:"category"`
	Subject  string `json:"subject"`
	Message  string `json:"message"`
}

type CreateInput struct {
	RequesterUserID string
	RequesterRole   RequesterRole
	SubmissionKind  Kind
	Topic           string
	Subject         string
	Message         string
}
