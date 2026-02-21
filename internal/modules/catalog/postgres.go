package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type postgresRepo struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) Repository { return &postgresRepo{db: db} }

func (r *postgresRepo) Create(ctx context.Context, p *PlatformProduct) error {
	var attrs interface{}
	if p.Attributes != nil {
		attrs = p.Attributes
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO platform_products
		  (id, name, description, category, base_price, currency, sku, image_url, is_active, attributes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		p.ID, p.Name, p.Description, p.Category, p.BasePrice,
		p.Currency, p.SKU, p.ImageURL, p.IsActive, attrs)
	return err
}

func scanProduct(scan func(...interface{}) error) (*PlatformProduct, error) {
	p := &PlatformProduct{}
	var attrs []byte
	err := scan(&p.ID, &p.Name, &p.Description, &p.Category, &p.BasePrice,
		&p.Currency, &p.SKU, &p.ImageURL, &p.IsActive, &attrs,
		&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if attrs != nil {
		p.Attributes = json.RawMessage(attrs)
	}
	return p, nil
}

func (r *postgresRepo) GetByID(ctx context.Context, id string) (*PlatformProduct, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id,name,description,category,base_price,currency,sku,image_url,is_active,attributes,created_at,updated_at
		FROM platform_products WHERE id=$1`, uid)
	return scanProduct(row.Scan)
}

func (r *postgresRepo) List(ctx context.Context, category string, activeOnly bool) ([]*PlatformProduct, error) {
	query := `SELECT id,name,description,category,base_price,currency,sku,image_url,is_active,attributes,created_at,updated_at
	          FROM platform_products WHERE 1=1`
	args := []interface{}{}
	n := 1
	if category != "" {
		query += fmt.Sprintf(` AND category=$%d`, n)
		args = append(args, category)
		n++
	}
	if activeOnly {
		query += ` AND is_active=true`
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []*PlatformProduct
	for rows.Next() {
		p, err := scanProduct(rows.Scan)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

func (r *postgresRepo) Update(ctx context.Context, p *PlatformProduct) error {
	var attrs interface{}
	if p.Attributes != nil {
		attrs = p.Attributes
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE platform_products
		SET name=$1, description=$2, category=$3, base_price=$4, currency=$5,
		    sku=$6, image_url=$7, is_active=$8, attributes=$9, updated_at=NOW()
		WHERE id=$10`,
		p.Name, p.Description, p.Category, p.BasePrice, p.Currency,
		p.SKU, p.ImageURL, p.IsActive, attrs, p.ID)
	return err
}
