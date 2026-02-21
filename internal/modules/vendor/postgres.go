package vendor

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type postgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository creates a new PostgreSQL vendor repository.
func NewPostgresRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) CreateVendor(ctx context.Context, vendor *Vendor) error {
	query := `
		INSERT INTO vendors (id, owner_id, tier_id, business_name, tax_id)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.ExecContext(ctx, query, vendor.ID, vendor.OwnerID, vendor.TierID, vendor.BusinessName, vendor.TaxID)
	return err
}

func (r *postgresRepository) GetVendorByOwnerID(ctx context.Context, ownerID string) (*Vendor, error) {
	vendor := &Vendor{}
	query := `
		SELECT id, owner_id, tier_id, business_name, tax_id, created_at, updated_at
		FROM vendors
		WHERE owner_id = $1
	`
	parsedID, err := uuid.Parse(ownerID)
	if err != nil {
		return nil, err
	}

	err = r.db.QueryRowContext(ctx, query, parsedID).Scan(
		&vendor.ID,
		&vendor.OwnerID,
		&vendor.TierID,
		&vendor.BusinessName,
		&vendor.TaxID,
		&vendor.CreatedAt,
		&vendor.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return vendor, nil
}
