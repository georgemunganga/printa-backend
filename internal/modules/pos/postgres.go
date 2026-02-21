package pos

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type postgresRepo struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) Repository { return &postgresRepo{db: db} }

func (r *postgresRepo) Create(ctx context.Context, tx *POSTransaction) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO pos_transactions
		  (id, order_id, store_id, cashier_id, amount, currency, payment_method,
		   reference, status, change_given, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tx.ID, tx.OrderID, tx.StoreID, tx.CashierID, tx.Amount, tx.Currency,
		tx.PaymentMethod, tx.Reference, tx.Status, tx.ChangeGiven, tx.Notes)
	return err
}

func (r *postgresRepo) GetByID(ctx context.Context, id string) (*POSTransaction, error) {
	return r.scan(r.db.QueryRowContext(ctx, `
		SELECT id,order_id,store_id,cashier_id,amount,currency,payment_method,
		       reference,status,change_given,notes,transacted_at,created_at,updated_at
		FROM pos_transactions WHERE id=$1`, id))
}

func (r *postgresRepo) GetByOrderID(ctx context.Context, orderID string) (*POSTransaction, error) {
	return r.scan(r.db.QueryRowContext(ctx, `
		SELECT id,order_id,store_id,cashier_id,amount,currency,payment_method,
		       reference,status,change_given,notes,transacted_at,created_at,updated_at
		FROM pos_transactions WHERE order_id=$1 ORDER BY created_at DESC LIMIT 1`, orderID))
}

func (r *postgresRepo) ListByStore(ctx context.Context, storeID string) ([]*POSTransaction, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id,order_id,store_id,cashier_id,amount,currency,payment_method,
		       reference,status,change_given,notes,transacted_at,created_at,updated_at
		FROM pos_transactions WHERE store_id=$1 ORDER BY created_at DESC`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var txs []*POSTransaction
	for rows.Next() {
		t, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		txs = append(txs, t)
	}
	return txs, nil
}

func (r *postgresRepo) UpdateStatus(ctx context.Context, id string, status TxStatus) error {
	_, err := r.db.ExecContext(ctx, `UPDATE pos_transactions SET status=$1, updated_at=$2 WHERE id=$3`,
		status, time.Now(), id)
	return err
}

// ── scanner ───────────────────────────────────────────────────────────────────

type rowScanner interface{ Scan(dest ...interface{}) error }

func (r *postgresRepo) scan(row rowScanner) (*POSTransaction, error) {
	t := &POSTransaction{}
	var cashierID sql.NullString
	var reference sql.NullString
	err := row.Scan(&t.ID, &t.OrderID, &t.StoreID, &cashierID,
		&t.Amount, &t.Currency, &t.PaymentMethod, &reference,
		&t.Status, &t.ChangeGiven, &t.Notes,
		&t.TransactedAt, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if cashierID.Valid {
		uid, _ := uuid.Parse(cashierID.String)
		t.CashierID = &uid
	}
	if reference.Valid {
		t.Reference = reference.String
	}
	return t, nil
}
