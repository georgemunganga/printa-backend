package operatingstatus

import "time"

type BlockReason string

const (
	BlockCompliancePending  BlockReason = "COMPLIANCE_APPROVAL_REQUIRED"
	BlockComplianceRejected BlockReason = "COMPLIANCE_APPROVAL_REJECTED"
	BlockSubscriptionDue    BlockReason = "SUBSCRIPTION_PAYMENT_DUE"
	BlockSubscriptionAbsent BlockReason = "SUBSCRIPTION_REQUIRED"
	BlockSubscriptionEnded  BlockReason = "SUBSCRIPTION_INACTIVE"
)

type ComplianceStatus string

const (
	CompliancePending  ComplianceStatus = "PENDING"
	ComplianceApproved ComplianceStatus = "APPROVED"
	ComplianceRejected ComplianceStatus = "REJECTED"
)

type SubscriptionStatus string

const (
	SubscriptionTrial     SubscriptionStatus = "TRIAL"
	SubscriptionActive    SubscriptionStatus = "ACTIVE"
	SubscriptionPastDue   SubscriptionStatus = "PAST_DUE"
	SubscriptionSuspended SubscriptionStatus = "SUSPENDED"
	SubscriptionCancelled SubscriptionStatus = "CANCELLED"
)

type Compliance struct {
	Status         ComplianceStatus `json:"status"`
	SubmittedAt    time.Time        `json:"submitted_at"`
	ReviewedAt     *time.Time       `json:"reviewed_at,omitempty"`
	DecisionReason string           `json:"decision_reason,omitempty"`
}

type Subscription struct {
	ID               string             `json:"id"`
	Status           SubscriptionStatus `json:"status"`
	CurrentPeriodEnd time.Time          `json:"current_period_end"`
	TrialEndsAt      *time.Time         `json:"trial_ends_at,omitempty"`
}

type GracePeriod struct {
	ID     string    `json:"id"`
	Status string    `json:"status"`
	EndsAt time.Time `json:"ends_at"`
}

type PaymentAction struct {
	Available bool   `json:"available"`
	URL       string `json:"url,omitempty"`
	Message   string `json:"message"`
}

type OperatingStatus struct {
	VendorID        string        `json:"vendor_id"`
	Operational     bool          `json:"operational"`
	BlockingReasons []BlockReason `json:"blocking_reasons"`
	Compliance      Compliance    `json:"compliance"`
	Subscription    *Subscription `json:"subscription,omitempty"`
	GracePeriod     *GracePeriod  `json:"grace_period,omitempty"`
	GraceEligible   bool          `json:"grace_eligible"`
	Payment         PaymentAction `json:"payment"`
}

type GraceRequestResult struct {
	Status  OperatingStatus `json:"status"`
	Granted bool            `json:"granted"`
}

type ComplianceDecisionRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}
