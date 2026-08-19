package policyconsent

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	ListPublishedVendorPolicies(ctx context.Context) ([]Policy, error)
	ListAcceptedPolicyIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	Accept(ctx context.Context, userID, vendorID uuid.UUID, policies []Policy, ip net.IP, userAgent, source string) error
	AttachToVendor(ctx context.Context, userID, vendorID uuid.UUID, ip net.IP, userAgent string) error
}

type postgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) ListPublishedVendorPolicies(ctx context.Context) ([]Policy, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, slug, version, title, summary, status, required_for_vendor,
		       COALESCE(document_url, ''), effective_at, published_at
		FROM platform_policies
		WHERE status = 'PUBLISHED'
		  AND required_for_vendor = TRUE
		  AND effective_at <= NOW()
		ORDER BY slug ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	policies := make([]Policy, 0)
	for rows.Next() {
		var policy Policy
		var effectiveAt, publishedAt sql.NullTime
		if err := rows.Scan(&policy.ID, &policy.Slug, &policy.Version, &policy.Title, &policy.Summary,
			&policy.Status, &policy.RequiredForVendor, &policy.DocumentURL, &effectiveAt, &publishedAt); err != nil {
			return nil, err
		}
		if effectiveAt.Valid {
			policy.EffectiveAt = &effectiveAt.Time
		}
		if publishedAt.Valid {
			policy.PublishedAt = &publishedAt.Time
		}
		policies = append(policies, policy)
	}
	return policies, rows.Err()
}

func (r *postgresRepository) ListAcceptedPolicyIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT policy_id
		FROM vendor_policy_consents
		WHERE user_id = $1 AND withdrawn_at IS NULL`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *postgresRepository) Accept(ctx context.Context, userID, vendorID uuid.UUID, policies []Policy, ip net.IP, userAgent, source string) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, policy := range policies {
		var consentID uuid.UUID
		err = tx.QueryRowContext(ctx, `
			INSERT INTO vendor_policy_consents (
				user_id, vendor_id, policy_id, policy_version, accepted_at, accepted_ip, user_agent, source
			) VALUES ($1,NULLIF($2::text,'00000000-0000-0000-0000-000000000000')::uuid,$3,$4,NOW(),NULLIF($5,'')::inet,NULLIF($6,''),$7)
			ON CONFLICT (user_id, policy_id) DO UPDATE SET
				vendor_id = EXCLUDED.vendor_id,
				policy_version = EXCLUDED.policy_version,
				accepted_at = EXCLUDED.accepted_at,
				accepted_ip = EXCLUDED.accepted_ip,
				user_agent = EXCLUDED.user_agent,
				source = EXCLUDED.source,
				withdrawn_at = NULL,
				withdrawal_reason = NULL,
				updated_at = NOW()
			RETURNING id`, userID, vendorID, policy.ID, policy.Version, ip.String(), userAgent, source).Scan(&consentID)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO vendor_policy_consent_events (
				consent_id, event_type, event_at, actor_user_id, actor_ip, user_agent, detail
			) VALUES ($1,'ACCEPTED',NOW(),$2,NULLIF($3,'')::inet,NULLIF($4,''),'{}'::jsonb)`,
			consentID, userID, ip.String(), userAgent)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *postgresRepository) AttachToVendor(ctx context.Context, userID, vendorID uuid.UUID, ip net.IP, userAgent string) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		UPDATE vendor_policy_consents
		SET vendor_id = $2, updated_at = NOW()
		WHERE user_id = $1 AND withdrawn_at IS NULL AND (vendor_id IS NULL OR vendor_id <> $2)
		RETURNING id`, userID, vendorID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var consentID uuid.UUID
		if err := rows.Scan(&consentID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO vendor_policy_consent_events (
				consent_id, event_type, event_at, actor_user_id, actor_ip, user_agent, detail
			) VALUES ($1,'ATTACHED_TO_VENDOR',NOW(),$2,NULLIF($3,'')::inet,NULLIF($4,''),'{}'::jsonb)`,
			consentID, userID, ip.String(), userAgent); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return tx.Commit()
}

func policyIDsToSet(ids []uuid.UUID) map[uuid.UUID]struct{} {
	out := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}

func missingPolicies(policies []Policy, acceptedIDs []uuid.UUID) []Policy {
	accepted := policyIDsToSet(acceptedIDs)
	missing := make([]Policy, 0)
	for _, policy := range policies {
		if _, ok := accepted[policy.ID]; !ok {
			missing = append(missing, policy)
		}
	}
	return missing
}

func isNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func currentTime() time.Time { return time.Now().UTC() }
