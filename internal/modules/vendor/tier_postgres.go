package vendor

import (
	"context"
	"database/sql"
)

type tierPostgresRepository struct {
	db *sql.DB
}

// NewTierPostgresRepository creates a new PostgreSQL vendor tier repository.
func NewTierPostgresRepository(db *sql.DB) TierRepository {
	return &tierPostgresRepository{db: db}
}

func (r *tierPostgresRepository) GetTiers(ctx context.Context) ([]*Tier, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, monthly_price, features, created_at, updated_at FROM vendor_tiers")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tiers []*Tier
	for rows.Next() {
		tier := &Tier{}
		if err := rows.Scan(&tier.ID, &tier.Name, &tier.MonthlyPrice, &tier.Features, &tier.CreatedAt, &tier.UpdatedAt); err != nil {
			return nil, err
		}
		tiers = append(tiers, tier)
	}

	return tiers, nil
}

func (r *tierPostgresRepository) GetTierByName(ctx context.Context, name string) (*Tier, error) {
	tier := &Tier{}
	query := `
		SELECT id, name, monthly_price, features, created_at, updated_at
		FROM vendor_tiers
		WHERE name = $1
	`
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&tier.ID,
		&tier.Name,
		&tier.MonthlyPrice,
		&tier.Features,
		&tier.CreatedAt,
		&tier.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return tier, nil
}
