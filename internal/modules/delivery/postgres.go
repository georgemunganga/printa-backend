package delivery

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type postgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

const locationColumns = `id, customer_id, label, recipient_name, recipient_phone,
	address_line1, address_line2, city, country, latitude, longitude, is_default, created_at, updated_at`

func (r *postgresRepository) ListByCustomer(ctx context.Context, customerID string) ([]*Location, error) {
	uid, err := uuid.Parse(customerID)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+locationColumns+`
		FROM customer_delivery_locations WHERE customer_id=$1
		ORDER BY is_default DESC, created_at ASC`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	locations := make([]*Location, 0)
	for rows.Next() {
		location := &Location{}
		if err := scanLocation(rows, location); err != nil {
			return nil, err
		}
		locations = append(locations, location)
	}
	return locations, rows.Err()
}

func (r *postgresRepository) GetByCustomer(ctx context.Context, id, customerID string) (*Location, error) {
	locationID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		return nil, err
	}
	location := &Location{}
	err = scanLocation(r.db.QueryRowContext(ctx, `SELECT `+locationColumns+`
		FROM customer_delivery_locations WHERE id=$1 AND customer_id=$2`, locationID, customerUUID), location)
	if err != nil {
		return nil, err
	}
	return location, nil
}

func (r *postgresRepository) Create(ctx context.Context, location *Location) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO customer_delivery_locations (
		id, customer_id, label, recipient_name, recipient_phone, address_line1, address_line2,
		city, country, latitude, longitude, is_default
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		location.ID, location.CustomerID, location.Label, location.RecipientName, location.RecipientPhone,
		location.AddressLine1, location.AddressLine2, location.City, location.Country,
		location.Latitude, location.Longitude, location.IsDefault)
	return err
}

func (r *postgresRepository) Update(ctx context.Context, location *Location) error {
	result, err := r.db.ExecContext(ctx, `UPDATE customer_delivery_locations
		SET label=$3, recipient_name=$4, recipient_phone=$5, address_line1=$6, address_line2=$7,
			city=$8, country=$9, latitude=$10, longitude=$11, updated_at=NOW()
		WHERE id=$1 AND customer_id=$2`,
		location.ID, location.CustomerID, location.Label, location.RecipientName, location.RecipientPhone,
		location.AddressLine1, location.AddressLine2, location.City, location.Country,
		location.Latitude, location.Longitude)
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
	locationID, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM customer_delivery_locations WHERE id=$1 AND customer_id=$2`, locationID, customerUUID)
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
	locationID, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `UPDATE customer_delivery_locations SET is_default=false, updated_at=NOW() WHERE customer_id=$1 AND is_default=true`, customerUUID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE customer_delivery_locations SET is_default=true, updated_at=NOW() WHERE id=$1 AND customer_id=$2`, locationID, customerUUID)
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

func (r *postgresRepository) PromoteEarliestAsDefault(ctx context.Context, customerID string) error {
	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `UPDATE customer_delivery_locations
		SET is_default=true, updated_at=NOW()
		WHERE id=(
			SELECT id FROM customer_delivery_locations
			WHERE customer_id=$1 ORDER BY created_at ASC LIMIT 1
		)`, customerUUID)
	return err
}

type locationScanner interface {
	Scan(dest ...interface{}) error
}

func scanLocation(scanner locationScanner, location *Location) error {
	return scanner.Scan(
		&location.ID, &location.CustomerID, &location.Label, &location.RecipientName, &location.RecipientPhone,
		&location.AddressLine1, &location.AddressLine2, &location.City, &location.Country,
		&location.Latitude, &location.Longitude, &location.IsDefault, &location.CreatedAt, &location.UpdatedAt,
	)
}
