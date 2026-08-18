package operatinghours

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type postgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) List(ctx context.Context, storeID string) ([]OperatingHour, error) {
	parsedStoreID, err := uuid.Parse(storeID)
	if err != nil {
		return nil, fmt.Errorf("invalid store id")
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT day_of_week, is_open, COALESCE(to_char(opens_at, 'HH24:MI'), ''), COALESCE(to_char(closes_at, 'HH24:MI'), '')
		FROM store_operating_hours
		WHERE store_id = $1
		ORDER BY day_of_week ASC
	`, parsedStoreID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hours := make([]OperatingHour, 0)
	for rows.Next() {
		var hour OperatingHour
		if err := rows.Scan(&hour.DayOfWeek, &hour.IsOpen, &hour.OpensAt, &hour.ClosesAt); err != nil {
			return nil, err
		}
		hours = append(hours, hour)
	}
	return hours, rows.Err()
}

func (r *postgresRepository) Replace(ctx context.Context, storeID string, hours []OperatingHour) error {
	parsedStoreID, err := uuid.Parse(storeID)
	if err != nil {
		return fmt.Errorf("invalid store id")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM store_operating_hours WHERE store_id = $1`, parsedStoreID); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO store_operating_hours (store_id, day_of_week, is_open, opens_at, closes_at)
		VALUES ($1, $2, $3, NULLIF($4, '')::time, NULLIF($5, '')::time)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, hour := range hours {
		if _, err := stmt.ExecContext(ctx, parsedStoreID, hour.DayOfWeek, hour.IsOpen, hour.OpensAt, hour.ClosesAt); err != nil {
			return err
		}
	}

	return tx.Commit()
}
