package order

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type postgresRepo struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) Repository { return &postgresRepo{db: db} }

// CreateOrder inserts the order and all its items inside a single transaction.
func (r *postgresRepo) CreateOrder(ctx context.Context, o *Order) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders
		  (id, store_id, customer_id, order_number, status, channel,
		   subtotal, discount, tax, total, currency, notes, delivery_address, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		o.ID, o.StoreID, o.CustomerID, o.OrderNumber, o.Status, o.Channel,
		o.Subtotal, o.Discount, o.Tax, o.Total, o.Currency, o.Notes,
		nullableJSON(o.DeliveryAddress), nullableJSON(o.Metadata))
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}

	for _, item := range o.Items {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO order_items
			  (id, order_id, vendor_store_product_id, quantity, unit_price, line_total, customisation)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			item.ID, o.ID, item.VendorStoreProductID,
			item.Quantity, item.UnitPrice, item.LineTotal,
			nullableJSON(item.Customisation))
		if err != nil {
			return fmt.Errorf("insert order_item: %w", err)
		}
	}

	return tx.Commit()
}

func (r *postgresRepo) GetOrderByID(ctx context.Context, id string) (*Order, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	o, err := r.scanOrder(r.db.QueryRowContext(ctx, `
		SELECT id,store_id,customer_id,order_number,status,channel,
		       subtotal,discount,tax,total,currency,notes,delivery_address,metadata,created_at,updated_at
		FROM orders WHERE id=$1`, uid))
	if err != nil {
		return nil, err
	}
	o.Items, err = r.listItems(ctx, o.ID.String())
	return o, err
}

func (r *postgresRepo) GetOrderByNumber(ctx context.Context, orderNumber string) (*Order, error) {
	o, err := r.scanOrder(r.db.QueryRowContext(ctx, `
		SELECT id,store_id,customer_id,order_number,status,channel,
		       subtotal,discount,tax,total,currency,notes,delivery_address,metadata,created_at,updated_at
		FROM orders WHERE order_number=$1`, orderNumber))
	if err != nil {
		return nil, err
	}
	o.Items, err = r.listItems(ctx, o.ID.String())
	return o, err
}

func (r *postgresRepo) ListOrdersByStore(ctx context.Context, storeID string, status string) ([]*Order, error) {
	query := `SELECT id,store_id,customer_id,order_number,status,channel,
	                 subtotal,discount,tax,total,currency,notes,delivery_address,metadata,created_at,updated_at
	          FROM orders WHERE store_id=$1`
	args := []interface{}{storeID}
	if status != "" {
		query += ` AND status=$2`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`
	return r.queryOrders(ctx, query, args...)
}

func (r *postgresRepo) ListOrdersByCustomer(ctx context.Context, customerID string) ([]*Order, error) {
	return r.queryOrders(ctx, `
		SELECT id,store_id,customer_id,order_number,status,channel,
		       subtotal,discount,tax,total,currency,notes,delivery_address,metadata,created_at,updated_at
		FROM orders WHERE customer_id=$1 ORDER BY created_at DESC`, customerID)
}

func (r *postgresRepo) UpdateStatus(ctx context.Context, id string, status OrderStatus) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE orders SET status=$1, updated_at=$2 WHERE id=$3`,
		status, time.Now(), id)
	return err
}

func (r *postgresRepo) GetProductPrice(ctx context.Context, vendorStoreProductID string) (float64, bool, error) {
	var price float64
	var available bool
	err := r.db.QueryRowContext(ctx,
		`SELECT vendor_price, is_available FROM vendor_store_products WHERE id=$1`,
		vendorStoreProductID).Scan(&price, &available)
	return price, available, err
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (r *postgresRepo) scanOrder(row *sql.Row) (*Order, error) {
	o := &Order{}
	var customerID sql.NullString
	var deliveryAddr, metadata []byte
	err := row.Scan(
		&o.ID, &o.StoreID, &customerID, &o.OrderNumber, &o.Status, &o.Channel,
		&o.Subtotal, &o.Discount, &o.Tax, &o.Total, &o.Currency, &o.Notes,
		&deliveryAddr, &metadata, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if customerID.Valid {
		uid, _ := uuid.Parse(customerID.String)
		o.CustomerID = &uid
	}
	o.DeliveryAddress = deliveryAddr
	o.Metadata = metadata
	return o, nil
}

func (r *postgresRepo) queryOrders(ctx context.Context, query string, args ...interface{}) ([]*Order, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orders []*Order
	for rows.Next() {
		o := &Order{}
		var customerID sql.NullString
		var deliveryAddr, metadata []byte
		if err := rows.Scan(
			&o.ID, &o.StoreID, &customerID, &o.OrderNumber, &o.Status, &o.Channel,
			&o.Subtotal, &o.Discount, &o.Tax, &o.Total, &o.Currency, &o.Notes,
			&deliveryAddr, &metadata, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		if customerID.Valid {
			uid, _ := uuid.Parse(customerID.String)
			o.CustomerID = &uid
		}
		o.DeliveryAddress = deliveryAddr
		o.Metadata = metadata
		orders = append(orders, o)
	}
	return orders, nil
}

func (r *postgresRepo) listItems(ctx context.Context, orderID string) ([]*OrderItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, order_id, vendor_store_product_id, quantity, unit_price, line_total, customisation, created_at, updated_at
		FROM order_items WHERE order_id=$1 ORDER BY created_at ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*OrderItem
	for rows.Next() {
		item := &OrderItem{}
		var customisation []byte
		if err := rows.Scan(&item.ID, &item.OrderID, &item.VendorStoreProductID,
			&item.Quantity, &item.UnitPrice, &item.LineTotal,
			&customisation, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Customisation = customisation
		items = append(items, item)
	}
	return items, nil
}

func nullableJSON(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	return b
}
