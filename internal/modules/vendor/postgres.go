package vendor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type postgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository creates the PostgreSQL implementation for vendor data.
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

// EnsureVendorWithFirstStore creates a vendor and its first storefront in one
// database transaction. It is idempotent for retried onboarding requests: if a
// profile and first store already exist for the authenticated owner, those
// persisted records are returned without creating duplicates.
func (r *postgresRepository) EnsureVendorWithFirstStore(ctx context.Context, candidate *Vendor, firstStore FirstStoreInput) (*Vendor, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Serialise retries for one owner even though the historical schema does not
	// yet enforce a unique vendors.owner_id constraint.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, candidate.OwnerID.String()); err != nil {
		return nil, err
	}

	vendorRecord := &Vendor{}
	err = tx.QueryRowContext(ctx, `
		SELECT id, owner_id, tier_id, business_name, tax_id, created_at, updated_at
		FROM vendors
		WHERE owner_id = $1
	`, candidate.OwnerID).Scan(
		&vendorRecord.ID,
		&vendorRecord.OwnerID,
		&vendorRecord.TierID,
		&vendorRecord.BusinessName,
		&vendorRecord.TaxID,
		&vendorRecord.CreatedAt,
		&vendorRecord.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		vendorRecord = &Vendor{
			ID:           candidate.ID,
			OwnerID:      candidate.OwnerID,
			TierID:       candidate.TierID,
			BusinessName: candidate.BusinessName,
			TaxID:        candidate.TaxID,
		}
		err = tx.QueryRowContext(ctx, `
			INSERT INTO vendors (id, owner_id, tier_id, business_name, tax_id)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING created_at, updated_at
		`, vendorRecord.ID, vendorRecord.OwnerID, vendorRecord.TierID, vendorRecord.BusinessName, vendorRecord.TaxID).Scan(
			&vendorRecord.CreatedAt,
			&vendorRecord.UpdatedAt,
		)
	}
	if err != nil {
		return nil, err
	}

	store := &FirstStore{}
	err = tx.QueryRowContext(ctx, `
		SELECT id, vendor_id, name, address, city, country, latitude, longitude, is_active, created_at, updated_at
		FROM stores
		WHERE vendor_id = $1
		ORDER BY created_at ASC
		LIMIT 1
	`, vendorRecord.ID).Scan(
		&store.ID,
		&store.VendorID,
		&store.Name,
		&store.Address,
		&store.City,
		&store.Country,
		&store.Latitude,
		&store.Longitude,
		&store.IsActive,
		&store.CreatedAt,
		&store.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		store = &FirstStore{
			ID:        uuid.New(),
			VendorID:  vendorRecord.ID,
			Name:      firstStore.Name,
			Address:   firstStore.Address,
			City:      firstStore.City,
			Country:   firstStore.Country,
			Latitude:  firstStore.Latitude,
			Longitude: firstStore.Longitude,
			IsActive:  true,
		}
		err = tx.QueryRowContext(ctx, `
			INSERT INTO stores (id, vendor_id, name, address, city, country, latitude, longitude, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING created_at, updated_at
		`, store.ID, store.VendorID, store.Name, store.Address, store.City, store.Country, store.Latitude, store.Longitude, store.IsActive).Scan(
			&store.CreatedAt,
			&store.UpdatedAt,
		)
	}
	if err != nil {
		return nil, err
	}

	if firstStore.OwnerPINHash != "" {
		result, err := tx.ExecContext(ctx, `
			UPDATE store_staff
			SET pin_hash = $3, pin_updated_at = NOW(), updated_at = NOW()
			WHERE store_id = $1 AND user_id = $2`, store.ID, vendorRecord.OwnerID, firstStore.OwnerPINHash)
		if err != nil {
			return nil, err
		}
		updated, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if updated != 1 {
			return nil, fmt.Errorf("owner staff assignment was not created for the first store")
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	vendorRecord.FirstStore = store
	return vendorRecord, nil
}
