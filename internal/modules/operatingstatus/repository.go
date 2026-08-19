package operatingstatus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	GetCompliance(ctx context.Context, vendorID string) (*Compliance, error)
	GetSubscription(ctx context.Context, vendorID string) (*Subscription, error)
	GetActiveGrace(ctx context.Context, vendorID string, now time.Time) (*GracePeriod, error)
	CreateGraceIfEligible(ctx context.Context, vendorID, userID string, subscription *Subscription, now time.Time) (*GracePeriod, bool, error)
	UpdateCompliance(ctx context.Context, vendorID, reviewerID string, status ComplianceStatus, reason string, now time.Time) (*Compliance, error)
}

type postgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) GetCompliance(ctx context.Context, vendorID string) (*Compliance, error) {
	var status string
	var submittedAt time.Time
	var reviewedAt sql.NullTime
	var reason sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT status, submitted_at, reviewed_at, decision_reason
		FROM vendor_compliance_reviews
		WHERE vendor_id = $1`, vendorID).Scan(&status, &submittedAt, &reviewedAt, &reason)
	if errors.Is(err, sql.ErrNoRows) {
		return &Compliance{Status: CompliancePending}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get compliance review: %w", err)
	}
	result := &Compliance{Status: ComplianceStatus(status), SubmittedAt: submittedAt, DecisionReason: reason.String}
	if reviewedAt.Valid {
		result.ReviewedAt = &reviewedAt.Time
	}
	return result, nil
}

func (r *postgresRepository) UpdateCompliance(ctx context.Context, vendorID, reviewerID string, status ComplianceStatus, reason string, now time.Time) (*Compliance, error) {
	vendorUUID, err := uuid.Parse(vendorID)
	if err != nil {
		return nil, fmt.Errorf("parse vendor id: %w", err)
	}
	reviewerUUID, err := uuid.Parse(reviewerID)
	if err != nil {
		return nil, fmt.Errorf("parse reviewer id: %w", err)
	}
	var persistedStatus string
	var submittedAt time.Time
	var reviewedAt sql.NullTime
	var persistedReason sql.NullString
	err = r.db.QueryRowContext(ctx, `
		UPDATE vendor_compliance_reviews
		SET status = $1, reviewed_at = $2, reviewed_by = $3,
			decision_reason = NULLIF($4, ''), updated_at = $2
		WHERE vendor_id = $5
		RETURNING status, submitted_at, reviewed_at, decision_reason`, status, now, reviewerUUID, reason, vendorUUID).
		Scan(&persistedStatus, &submittedAt, &reviewedAt, &persistedReason)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("compliance review not found")
	}
	if err != nil {
		return nil, fmt.Errorf("update compliance review: %w", err)
	}
	result := &Compliance{Status: ComplianceStatus(persistedStatus), SubmittedAt: submittedAt, DecisionReason: persistedReason.String}
	if reviewedAt.Valid {
		result.ReviewedAt = &reviewedAt.Time
	}
	return result, nil
}

func (r *postgresRepository) GetSubscription(ctx context.Context, vendorID string) (*Subscription, error) {
	var id uuid.UUID
	var status string
	var periodEnd time.Time
	var trialEndsAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, status, current_period_end, trial_ends_at
		FROM vendor_subscriptions
		WHERE vendor_id = $1`, vendorID).Scan(&id, &status, &periodEnd, &trialEndsAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get vendor subscription: %w", err)
	}
	result := &Subscription{ID: id.String(), Status: SubscriptionStatus(status), CurrentPeriodEnd: periodEnd}
	if trialEndsAt.Valid {
		result.TrialEndsAt = &trialEndsAt.Time
	}
	return result, nil
}

func (r *postgresRepository) GetActiveGrace(ctx context.Context, vendorID string, now time.Time) (*GracePeriod, error) {
	var id uuid.UUID
	var status string
	var endsAt time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT id, status, ends_at
		FROM vendor_subscription_grace_periods
		WHERE vendor_id = $1 AND status = 'ACTIVE' AND ends_at > $2
		ORDER BY ends_at DESC
		LIMIT 1`, vendorID, now).Scan(&id, &status, &endsAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get active grace period: %w", err)
	}
	return &GracePeriod{ID: id.String(), Status: status, EndsAt: endsAt}, nil
}

func (r *postgresRepository) CreateGraceIfEligible(ctx context.Context, vendorID, userID string, subscription *Subscription, now time.Time) (*GracePeriod, bool, error) {
	if subscription == nil {
		return nil, false, nil
	}
	vendorUUID, err := uuid.Parse(vendorID)
	if err != nil {
		return nil, false, fmt.Errorf("parse vendor id: %w", err)
	}
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, false, fmt.Errorf("parse user id: %w", err)
	}
	subscriptionUUID, err := uuid.Parse(subscription.ID)
	if err != nil {
		return nil, false, fmt.Errorf("parse subscription id: %w", err)
	}
	if subscription.Status != SubscriptionPastDue || !now.After(subscription.CurrentPeriodEnd) {
		return nil, false, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("start grace transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE vendor_subscription_grace_periods
		SET status = 'EXPIRED', updated_at = $1
		WHERE vendor_id = $2 AND status = 'ACTIVE' AND ends_at <= $1`, now, vendorUUID); err != nil {
		return nil, false, fmt.Errorf("expire stale grace periods: %w", err)
	}

	var existingID uuid.UUID
	var existingStatus string
	var existingEndsAt time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT id, status, ends_at
		FROM vendor_subscription_grace_periods
		WHERE vendor_id = $1 AND subscription_end_at = $2
		FOR UPDATE`, vendorUUID, subscription.CurrentPeriodEnd).Scan(&existingID, &existingStatus, &existingEndsAt)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return nil, false, fmt.Errorf("commit existing grace: %w", err)
		}
		return &GracePeriod{ID: existingID.String(), Status: existingStatus, EndsAt: existingEndsAt}, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, fmt.Errorf("lock grace period: %w", err)
	}

	endsAt := now.AddDate(0, 0, 5)
	var id uuid.UUID
	err = tx.QueryRowContext(ctx, `
		INSERT INTO vendor_subscription_grace_periods (
			vendor_id, subscription_id, requested_by, granted_at, ends_at, subscription_end_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`, vendorUUID, subscriptionUUID, userUUID, now, endsAt, subscription.CurrentPeriodEnd).Scan(&id)
	if err != nil {
		return nil, false, fmt.Errorf("create grace period: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("commit grace period: %w", err)
	}
	return &GracePeriod{ID: id.String(), Status: "ACTIVE", EndsAt: endsAt}, true, nil
}
