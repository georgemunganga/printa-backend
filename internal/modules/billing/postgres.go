package billing

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type postgresRepo struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) Repository { return &postgresRepo{db: db} }

// ── Subscription ──────────────────────────────────────────────────────────────

func (r *postgresRepo) CreateSubscription(ctx context.Context, sub *VendorSubscription) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO vendor_subscriptions
		  (id, vendor_id, tier_id, status, billing_cycle,
		   current_period_start, current_period_end, trial_ends_at, auto_renew)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		sub.ID, sub.VendorID, sub.TierID, sub.Status, sub.BillingCycle,
		sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.TrialEndsAt, sub.AutoRenew)
	return err
}

func (r *postgresRepo) GetSubscriptionByVendor(ctx context.Context, vendorID string) (*VendorSubscription, error) {
	return r.scanSub(r.db.QueryRowContext(ctx, `
		SELECT vs.id, vs.vendor_id, vs.tier_id, vt.name, vt.monthly_price,
		       vs.status, vs.billing_cycle, vs.current_period_start, vs.current_period_end,
		       vs.trial_ends_at, vs.cancelled_at, vs.cancel_reason, vs.auto_renew,
		       vs.created_at, vs.updated_at
		FROM vendor_subscriptions vs
		JOIN vendor_tiers vt ON vt.id = vs.tier_id
		WHERE vs.vendor_id = $1`, vendorID))
}

func (r *postgresRepo) GetSubscriptionByID(ctx context.Context, id string) (*VendorSubscription, error) {
	return r.scanSub(r.db.QueryRowContext(ctx, `
		SELECT vs.id, vs.vendor_id, vs.tier_id, vt.name, vt.monthly_price,
		       vs.status, vs.billing_cycle, vs.current_period_start, vs.current_period_end,
		       vs.trial_ends_at, vs.cancelled_at, vs.cancel_reason, vs.auto_renew,
		       vs.created_at, vs.updated_at
		FROM vendor_subscriptions vs
		JOIN vendor_tiers vt ON vt.id = vs.tier_id
		WHERE vs.id = $1`, id))
}

func (r *postgresRepo) UpdateSubscriptionStatus(ctx context.Context, id string, status SubscriptionStatus, reason string) error {
	now := time.Now()
	var cancelledAt interface{}
	if status == SubCancelled {
		cancelledAt = now
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE vendor_subscriptions
		SET status=$1, cancel_reason=COALESCE(NULLIF($2,''), cancel_reason),
		    cancelled_at=COALESCE($3, cancelled_at), updated_at=$4
		WHERE id=$5`,
		status, reason, cancelledAt, now, id)
	return err
}

func (r *postgresRepo) UpdateSubscriptionTier(ctx context.Context, id string, tierID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE vendor_subscriptions SET tier_id=$1, updated_at=$2 WHERE id=$3`,
		tierID, time.Now(), id)
	return err
}

func (r *postgresRepo) RenewSubscriptionPeriod(ctx context.Context, id string, start, end interface{}) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE vendor_subscriptions SET current_period_start=$1, current_period_end=$2, updated_at=$3 WHERE id=$4`,
		start, end, time.Now(), id)
	return err
}

func (r *postgresRepo) ListExpiredSubscriptions(ctx context.Context) ([]*VendorSubscription, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT vs.id, vs.vendor_id, vs.tier_id, vt.name, vt.monthly_price,
		       vs.status, vs.billing_cycle, vs.current_period_start, vs.current_period_end,
		       vs.trial_ends_at, vs.cancelled_at, vs.cancel_reason, vs.auto_renew,
		       vs.created_at, vs.updated_at
		FROM vendor_subscriptions vs
		JOIN vendor_tiers vt ON vt.id = vs.tier_id
		WHERE vs.current_period_end < NOW() AND vs.status IN ('ACTIVE','TRIAL')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subs []*VendorSubscription
	for rows.Next() {
		s, err := r.scanSub(rows)
		if err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, nil
}

// ── Invoice ───────────────────────────────────────────────────────────────────

func (r *postgresRepo) CreateInvoice(ctx context.Context, inv *BillingInvoice) error {
	lineItemsJSON, err := json.Marshal(inv.LineItems)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO billing_invoices
		  (id, subscription_id, vendor_id, invoice_number, amount, currency,
		   status, period_start, period_end, due_date, line_items, notes, idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		inv.ID, inv.SubscriptionID, inv.VendorID, inv.InvoiceNumber, inv.Amount,
		inv.Currency, inv.Status, inv.PeriodStart, inv.PeriodEnd, inv.DueDate,
		lineItemsJSON, inv.Notes, inv.IdempotencyKey)
	return err
}

func (r *postgresRepo) GetInvoiceByID(ctx context.Context, id string) (*BillingInvoice, error) {
	return r.scanInv(r.db.QueryRowContext(ctx, `
		SELECT id,subscription_id,vendor_id,invoice_number,amount,currency,status,
		       period_start,period_end,due_date,paid_at,payment_reference,line_items,
		       notes,idempotency_key,created_at,updated_at
		FROM billing_invoices WHERE id=$1`, id))
}

func (r *postgresRepo) GetInvoiceByNumber(ctx context.Context, number string) (*BillingInvoice, error) {
	return r.scanInv(r.db.QueryRowContext(ctx, `
		SELECT id,subscription_id,vendor_id,invoice_number,amount,currency,status,
		       period_start,period_end,due_date,paid_at,payment_reference,line_items,
		       notes,idempotency_key,created_at,updated_at
		FROM billing_invoices WHERE invoice_number=$1`, number))
}

func (r *postgresRepo) GetInvoiceByIdempotencyKey(ctx context.Context, key string) (*BillingInvoice, error) {
	return r.scanInv(r.db.QueryRowContext(ctx, `
		SELECT id,subscription_id,vendor_id,invoice_number,amount,currency,status,
		       period_start,period_end,due_date,paid_at,payment_reference,line_items,
		       notes,idempotency_key,created_at,updated_at
		FROM billing_invoices WHERE idempotency_key=$1`, key))
}

func (r *postgresRepo) ListInvoicesByVendor(ctx context.Context, vendorID string) ([]*BillingInvoice, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id,subscription_id,vendor_id,invoice_number,amount,currency,status,
		       period_start,period_end,due_date,paid_at,payment_reference,line_items,
		       notes,idempotency_key,created_at,updated_at
		FROM billing_invoices WHERE vendor_id=$1 ORDER BY created_at DESC`, vendorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var invs []*BillingInvoice
	for rows.Next() {
		inv, err := r.scanInv(rows)
		if err != nil {
			return nil, err
		}
		invs = append(invs, inv)
	}
	return invs, nil
}

func (r *postgresRepo) ListInvoicesBySubscription(ctx context.Context, subscriptionID string) ([]*BillingInvoice, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id,subscription_id,vendor_id,invoice_number,amount,currency,status,
		       period_start,period_end,due_date,paid_at,payment_reference,line_items,
		       notes,idempotency_key,created_at,updated_at
		FROM billing_invoices WHERE subscription_id=$1 ORDER BY created_at DESC`, subscriptionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var invs []*BillingInvoice
	for rows.Next() {
		inv, err := r.scanInv(rows)
		if err != nil {
			return nil, err
		}
		invs = append(invs, inv)
	}
	return invs, nil
}

func (r *postgresRepo) MarkInvoicePaid(ctx context.Context, id string, ref string, notes string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE billing_invoices
		SET status='PAID', paid_at=$1, payment_reference=$2,
		    notes=COALESCE(NULLIF($3,''), notes), updated_at=$4
		WHERE id=$5`,
		now, ref, notes, now, id)
	return err
}

func (r *postgresRepo) VoidInvoice(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE billing_invoices SET status='VOID', updated_at=$1 WHERE id=$2`,
		time.Now(), id)
	return err
}

func (r *postgresRepo) ListTiers(ctx context.Context) ([]*VendorTier, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, monthly_price, features, created_at, updated_at
		FROM vendor_tiers
		ORDER BY COALESCE(NULLIF(features->>'display_order', '')::INT, 999), name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tiers := make([]*VendorTier, 0)
	for rows.Next() {
		tier := &VendorTier{}
		var featuresJSON []byte
		if err := rows.Scan(&tier.ID, &tier.Name, &tier.MonthlyPrice, &featuresJSON, &tier.CreatedAt, &tier.UpdatedAt); err != nil {
			return nil, err
		}

		var metadata struct {
			Description  string        `json:"description"`
			DisplayOrder int           `json:"display_order"`
			IsAvailable  bool          `json:"is_available"`
			IsPopular    bool          `json:"is_popular"`
			Features     []TierFeature `json:"features"`
		}
		if len(featuresJSON) > 0 {
			if err := json.Unmarshal(featuresJSON, &metadata); err != nil {
				return nil, err
			}
		}
		tier.Description = metadata.Description
		tier.DisplayOrder = metadata.DisplayOrder
		tier.IsAvailable = metadata.IsAvailable
		tier.IsPopular = metadata.IsPopular
		tier.Features = metadata.Features
		if tier.Features == nil {
			tier.Features = []TierFeature{}
		}
		tiers = append(tiers, tier)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tiers, nil
}

func (r *postgresRepo) GetTierByID(ctx context.Context, tierID string) (string, float64, error) {
	var name string
	var price float64
	err := r.db.QueryRowContext(ctx, `SELECT name, monthly_price FROM vendor_tiers WHERE id=$1`, tierID).
		Scan(&name, &price)
	return name, price, err
}

// ── Subscription checkout ────────────────────────────────────────────────────

func (r *postgresRepo) GetTierCatalogueEntry(ctx context.Context, tierID string) (*VendorTier, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, monthly_price, features, created_at, updated_at
		FROM vendor_tiers WHERE id=$1`, tierID)
	return scanTier(row)
}

func (r *postgresRepo) CreateCheckout(ctx context.Context, checkout *SubscriptionCheckout) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO subscription_checkouts
		  (id, vendor_id, tier_id, amount, currency, reference, status, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		checkout.ID, checkout.VendorID, checkout.TierID, checkout.Amount, checkout.Currency,
		checkout.Reference, checkout.Status, checkout.ExpiresAt)
	return err
}

func (r *postgresRepo) GetCheckoutByID(ctx context.Context, id string) (*SubscriptionCheckout, error) {
	return r.scanCheckout(r.db.QueryRowContext(ctx, checkoutSelect+` WHERE sc.id=$1`, id))
}

func (r *postgresRepo) GetCheckoutByReference(ctx context.Context, reference string) (*SubscriptionCheckout, error) {
	return r.scanCheckout(r.db.QueryRowContext(ctx, checkoutSelect+` WHERE sc.reference=$1`, reference))
}

func (r *postgresRepo) GetReusablePendingCheckout(ctx context.Context, vendorID, tierID string, now time.Time) (*SubscriptionCheckout, error) {
	return r.scanCheckout(r.db.QueryRowContext(ctx, checkoutSelect+`
		WHERE sc.vendor_id=$1 AND sc.tier_id=$2 AND sc.status='PENDING' AND sc.expires_at>$3
		ORDER BY sc.created_at DESC LIMIT 1`, vendorID, tierID, now))
}

func (r *postgresRepo) RecordCheckoutCollection(ctx context.Context, id, providerCollectionID, providerStatus string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE subscription_checkouts
		SET provider_collection_id=$1, provider_status=$2, updated_at=NOW()
		WHERE id=$3 AND status='PENDING'`, providerCollectionID, providerStatus, id)
	return err
}

func (r *postgresRepo) MarkCheckoutFailed(ctx context.Context, id, providerStatus, reason string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE subscription_checkouts
		SET status='FAILED', provider_status=$1, failure_reason=$2, updated_at=NOW()
		WHERE id=$3 AND status='PENDING'`, providerStatus, reason, id)
	return err
}

func (r *postgresRepo) ActivateCheckout(ctx context.Context, checkoutID, providerCollectionID, providerStatus string, completedAt time.Time) (*SubscriptionCheckout, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	checkout, err := r.scanCheckout(tx.QueryRowContext(ctx, checkoutSelect+` WHERE sc.id=$1 FOR UPDATE`, checkoutID))
	if err != nil {
		return nil, err
	}
	if checkout.Status == CheckoutSuccessful {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return checkout, nil
	}
	if checkout.Status != CheckoutPending {
		return nil, fmt.Errorf("checkout is not pending")
	}
	if !checkout.ExpiresAt.After(completedAt) {
		_, _ = tx.ExecContext(ctx, `UPDATE subscription_checkouts SET status='EXPIRED', updated_at=NOW() WHERE id=$1`, checkout.ID)
		return nil, fmt.Errorf("checkout has expired")
	}

	periodStart := completedAt.UTC()
	periodEnd := periodStart.AddDate(0, 1, 0)
	var subscriptionID uuid.UUID
	err = tx.QueryRowContext(ctx, `
		INSERT INTO vendor_subscriptions
		  (id, vendor_id, tier_id, status, billing_cycle, current_period_start, current_period_end, auto_renew)
		VALUES (gen_random_uuid(), $1, $2, 'ACTIVE', 'MONTHLY', $3, $4, TRUE)
		ON CONFLICT (vendor_id) DO UPDATE
		SET tier_id=EXCLUDED.tier_id, status='ACTIVE', billing_cycle='MONTHLY',
		    current_period_start=EXCLUDED.current_period_start,
		    current_period_end=EXCLUDED.current_period_end,
		    trial_ends_at=NULL, cancelled_at=NULL, cancel_reason=NULL, auto_renew=TRUE,
		    updated_at=NOW()
		RETURNING id`, checkout.VendorID, checkout.TierID, periodStart, periodEnd).Scan(&subscriptionID)
	if err != nil {
		return nil, err
	}

	invoiceKey := "subscription-checkout:" + checkout.ID.String()
	var invoiceID uuid.UUID
	err = tx.QueryRowContext(ctx, `SELECT id FROM billing_invoices WHERE idempotency_key=$1 FOR UPDATE`, invoiceKey).Scan(&invoiceID)
	if err == sql.ErrNoRows {
		invoiceID = uuid.New()
		lineItems, marshalErr := json.Marshal([]LineItem{{
			Description: fmt.Sprintf("Printa %s Plan (MONTHLY)", checkout.TierName),
			Quantity:    1, UnitPrice: checkout.Amount, Amount: checkout.Amount,
		}})
		if marshalErr != nil {
			return nil, marshalErr
		}
		invoiceNumber := fmt.Sprintf("INV-%s-%s", periodStart.Format("200601"), checkout.ID.String()[:8])
		_, err = tx.ExecContext(ctx, `
			INSERT INTO billing_invoices
			  (id, subscription_id, vendor_id, invoice_number, amount, currency, status,
			   period_start, period_end, due_date, paid_at, payment_reference, line_items, notes, idempotency_key)
			VALUES ($1,$2,$3,$4,$5,$6,'PAID',$7,$8,$9,$10,$11,$12,$13,$14)`,
			invoiceID, subscriptionID, checkout.VendorID, invoiceNumber, checkout.Amount, checkout.Currency,
			periodStart, periodEnd, periodStart, completedAt, providerCollectionID, lineItems,
			"Verified subscription collection", invoiceKey)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE subscription_checkouts
		SET status='SUCCESSFUL', provider_collection_id=$1, provider_status=$2,
		    subscription_id=$3, invoice_id=$4, completed_at=$5, failure_reason=NULL, updated_at=NOW()
		WHERE id=$6`, providerCollectionID, providerStatus, subscriptionID, invoiceID, completedAt, checkout.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetCheckoutByID(ctx, checkout.ID.String())
}

const checkoutSelect = `
	SELECT sc.id, sc.vendor_id, sc.tier_id, vt.name, sc.amount, sc.currency, sc.reference,
	       sc.status, COALESCE(sc.provider_collection_id,''), COALESCE(sc.provider_status,''),
	       sc.subscription_id, sc.invoice_id, sc.expires_at, sc.completed_at,
	       COALESCE(sc.failure_reason,''), sc.created_at, sc.updated_at
	FROM subscription_checkouts sc
	JOIN vendor_tiers vt ON vt.id=sc.tier_id`

func (r *postgresRepo) scanCheckout(row rowScanner) (*SubscriptionCheckout, error) {
	checkout := &SubscriptionCheckout{}
	if err := row.Scan(
		&checkout.ID, &checkout.VendorID, &checkout.TierID, &checkout.TierName,
		&checkout.Amount, &checkout.Currency, &checkout.Reference, &checkout.Status,
		&checkout.ProviderCollectionID, &checkout.ProviderStatus, &checkout.SubscriptionID,
		&checkout.InvoiceID, &checkout.ExpiresAt, &checkout.CompletedAt, &checkout.FailureReason,
		&checkout.CreatedAt, &checkout.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return checkout, nil
}

func scanTier(row rowScanner) (*VendorTier, error) {
	tier := &VendorTier{}
	var featuresJSON []byte
	if err := row.Scan(&tier.ID, &tier.Name, &tier.MonthlyPrice, &featuresJSON, &tier.CreatedAt, &tier.UpdatedAt); err != nil {
		return nil, err
	}
	var metadata struct {
		Description  string        `json:"description"`
		DisplayOrder int           `json:"display_order"`
		IsAvailable  bool          `json:"is_available"`
		IsPopular    bool          `json:"is_popular"`
		Features     []TierFeature `json:"features"`
	}
	if len(featuresJSON) > 0 {
		if err := json.Unmarshal(featuresJSON, &metadata); err != nil {
			return nil, err
		}
	}
	tier.Description = metadata.Description
	tier.DisplayOrder = metadata.DisplayOrder
	tier.IsAvailable = metadata.IsAvailable
	tier.IsPopular = metadata.IsPopular
	tier.Features = metadata.Features
	if tier.Features == nil {
		tier.Features = []TierFeature{}
	}
	return tier, nil
}

// ── Scanners ──────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func (r *postgresRepo) scanSub(row rowScanner) (*VendorSubscription, error) {
	s := &VendorSubscription{}
	var trialEndsAt, cancelledAt sql.NullTime
	var cancelReason sql.NullString
	err := row.Scan(&s.ID, &s.VendorID, &s.TierID, &s.TierName, &s.TierPrice,
		&s.Status, &s.BillingCycle, &s.CurrentPeriodStart, &s.CurrentPeriodEnd,
		&trialEndsAt, &cancelledAt, &cancelReason, &s.AutoRenew,
		&s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if trialEndsAt.Valid {
		s.TrialEndsAt = &trialEndsAt.Time
	}
	if cancelledAt.Valid {
		s.CancelledAt = &cancelledAt.Time
	}
	if cancelReason.Valid {
		s.CancelReason = cancelReason.String
	}
	return s, nil
}

func (r *postgresRepo) scanInv(row rowScanner) (*BillingInvoice, error) {
	inv := &BillingInvoice{}
	var paidAt sql.NullTime
	var payRef, iKey, notes sql.NullString
	var lineItemsJSON []byte
	err := row.Scan(&inv.ID, &inv.SubscriptionID, &inv.VendorID, &inv.InvoiceNumber,
		&inv.Amount, &inv.Currency, &inv.Status, &inv.PeriodStart, &inv.PeriodEnd,
		&inv.DueDate, &paidAt, &payRef, &lineItemsJSON, &notes, &iKey,
		&inv.CreatedAt, &inv.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if paidAt.Valid {
		inv.PaidAt = &paidAt.Time
	}
	if payRef.Valid {
		inv.PaymentReference = payRef.String
	}
	if iKey.Valid {
		inv.IdempotencyKey = iKey.String
	}
	if notes.Valid {
		inv.Notes = notes.String
	}
	if len(lineItemsJSON) > 0 {
		_ = json.Unmarshal(lineItemsJSON, &inv.LineItems)
	}
	// Ensure non-nil slice for JSON output
	if inv.LineItems == nil {
		inv.LineItems = []LineItem{}
	}
	_ = uuid.New() // ensure uuid import is used
	return inv, nil
}
