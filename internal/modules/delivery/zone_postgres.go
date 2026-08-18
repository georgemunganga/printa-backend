package delivery

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type zonePostgresRepository struct {
	db *sql.DB
}

func NewZonePostgresRepository(db *sql.DB) ZoneRepository {
	return &zonePostgresRepository{db: db}
}

const zoneColumns = `id, store_id, name, city, country, is_active, created_at, updated_at`

func (r *zonePostgresRepository) ListByStore(ctx context.Context, storeID string) ([]*Zone, error) {
	storeUUID, err := uuid.Parse(storeID)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+zoneColumns+`
		FROM store_delivery_zones WHERE store_id=$1 ORDER BY is_active DESC, city ASC, name ASC`, storeUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	zones := make([]*Zone, 0)
	for rows.Next() {
		zone := &Zone{}
		if err := scanZone(rows, zone); err != nil {
			return nil, err
		}
		zones = append(zones, zone)
	}
	return zones, rows.Err()
}

func (r *zonePostgresRepository) GetByStore(ctx context.Context, id, storeID string) (*Zone, error) {
	zoneUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	storeUUID, err := uuid.Parse(storeID)
	if err != nil {
		return nil, err
	}
	zone := &Zone{}
	err = scanZone(r.db.QueryRowContext(ctx, `SELECT `+zoneColumns+` FROM store_delivery_zones WHERE id=$1 AND store_id=$2`, zoneUUID, storeUUID), zone)
	if err != nil {
		return nil, err
	}
	return zone, nil
}

func (r *zonePostgresRepository) Create(ctx context.Context, zone *Zone) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO store_delivery_zones (id, store_id, name, city, country, is_active)
		VALUES ($1,$2,$3,$4,$5,$6)`, zone.ID, zone.StoreID, zone.Name, zone.City, zone.Country, zone.IsActive)
	return err
}

func (r *zonePostgresRepository) Update(ctx context.Context, zone *Zone) error {
	result, err := r.db.ExecContext(ctx, `UPDATE store_delivery_zones
		SET name=$3, city=$4, country=$5, is_active=$6, updated_at=NOW()
		WHERE id=$1 AND store_id=$2`, zone.ID, zone.StoreID, zone.Name, zone.City, zone.Country, zone.IsActive)
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

func (r *zonePostgresRepository) Delete(ctx context.Context, id, storeID string) error {
	zoneUUID, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	storeUUID, err := uuid.Parse(storeID)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM store_delivery_zones WHERE id=$1 AND store_id=$2`, zoneUUID, storeUUID)
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

func (r *zonePostgresRepository) FindActiveByStoreCity(ctx context.Context, storeID, city, country string) (*Zone, error) {
	storeUUID, err := uuid.Parse(storeID)
	if err != nil {
		return nil, err
	}
	zone := &Zone{}
	err = scanZone(r.db.QueryRowContext(ctx, `SELECT `+zoneColumns+`
		FROM store_delivery_zones
		WHERE store_id=$1 AND is_active=true AND LOWER(city)=LOWER($2) AND LOWER(country)=LOWER($3)
		LIMIT 1`, storeUUID, city, country), zone)
	if err != nil {
		return nil, err
	}
	return zone, nil
}

func (r *zonePostgresRepository) HasAnyForStore(ctx context.Context, storeID string) (bool, error) {
	storeUUID, err := uuid.Parse(storeID)
	if err != nil {
		return false, err
	}
	var exists bool
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM store_delivery_zones WHERE store_id=$1)`, storeUUID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

type zoneScanner interface {
	Scan(dest ...interface{}) error
}

func scanZone(scanner zoneScanner, zone *Zone) error {
	return scanner.Scan(&zone.ID, &zone.StoreID, &zone.Name, &zone.City, &zone.Country, &zone.IsActive, &zone.CreatedAt, &zone.UpdatedAt)
}
