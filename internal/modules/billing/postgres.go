package billing

import (
	"context"
	"database/sql"
	"encoding/json"
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

func (r *postgresRepo) GetTierByID(ctx context.Context, tierID string) (string, float64, error) {
	var name string
	var price float64
	err := r.db.QueryRowContext(ctx, `SELECT name, monthly_price FROM vendor_tiers WHERE id=$1`, tierID).
		Scan(&name, &price)
	return name, price, err
}

// ── Scanners ──────────────────────────────────────────────────────────────────

type rowScanner interface{ Scan(dest ...interface{}) error }

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
