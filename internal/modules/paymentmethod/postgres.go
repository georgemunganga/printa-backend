package paymentmethod

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type postgresRepository struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) Repository { return &postgresRepository{db: db} }

const columns = `id, customer_id, provider, phone_number, label, is_default, created_at, updated_at`

func (r *postgresRepository) List(ctx context.Context, customerID string) ([]*Method, error) {
	uid, err := uuid.Parse(customerID)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+columns+` FROM customer_payment_methods WHERE customer_id=$1 ORDER BY is_default DESC, created_at ASC`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	methods := make([]*Method, 0)
	for rows.Next() {
		method := &Method{}
		if err := scan(rows, method); err != nil {
			return nil, err
		}
		methods = append(methods, method)
	}
	return methods, rows.Err()
}

func (r *postgresRepository) Get(ctx context.Context, id, customerID string) (*Method, error) {
	methodID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	uid, err := uuid.Parse(customerID)
	if err != nil {
		return nil, err
	}
	method := &Method{}
	err = scan(r.db.QueryRowContext(ctx, `SELECT `+columns+` FROM customer_payment_methods WHERE id=$1 AND customer_id=$2`, methodID, uid), method)
	return method, err
}

func (r *postgresRepository) Create(ctx context.Context, method *Method) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO customer_payment_methods (id, customer_id, provider, phone_number, label, is_default) VALUES ($1,$2,$3,$4,$5,$6)`, method.ID, method.CustomerID, method.Provider, method.PhoneNumber, method.Label, method.IsDefault)
	return err
}

func (r *postgresRepository) Update(ctx context.Context, method *Method) error {
	result, err := r.db.ExecContext(ctx, `UPDATE customer_payment_methods SET provider=$3, phone_number=$4, label=$5, updated_at=NOW() WHERE id=$1 AND customer_id=$2`, method.ID, method.CustomerID, method.Provider, method.PhoneNumber, method.Label)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *postgresRepository) Delete(ctx context.Context, id, customerID string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM customer_payment_methods WHERE id=$1 AND customer_id=$2`, id, customerID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *postgresRepository) SetDefault(ctx context.Context, id, customerID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `UPDATE customer_payment_methods SET is_default=false, updated_at=NOW() WHERE customer_id=$1 AND is_default=true`, customerID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE customer_payment_methods SET is_default=true, updated_at=NOW() WHERE id=$1 AND customer_id=$2`, id, customerID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (r *postgresRepository) PromoteEarliest(ctx context.Context, customerID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE customer_payment_methods SET is_default=true, updated_at=NOW() WHERE id=(SELECT id FROM customer_payment_methods WHERE customer_id=$1 ORDER BY created_at ASC LIMIT 1)`, customerID)
	return err
}

type scanner interface{ Scan(...interface{}) error }

func scan(row scanner, method *Method) error {
	return row.Scan(&method.ID, &method.CustomerID, &method.Provider, &method.PhoneNumber, &method.Label, &method.IsDefault, &method.CreatedAt, &method.UpdatedAt)
}
