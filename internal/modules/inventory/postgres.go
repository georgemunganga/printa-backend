package inventory

import (
"context"
"database/sql"
"fmt"

"github.com/google/uuid"
)

// ---- Store ----

type storePostgres struct{ db *sql.DB }

func NewStorePostgresRepository(db *sql.DB) StoreRepository { return &storePostgres{db: db} }

func (r *storePostgres) CreateStore(ctx context.Context, s *Store) error {
_, err := r.db.ExecContext(ctx, `
INSERT INTO stores (id,vendor_id,name,description,address,city,country,phone,email,is_active)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
s.ID, s.VendorID, s.Name, s.Description, s.Address,
s.City, s.Country, s.Phone, s.Email, s.IsActive)
return err
}

func (r *storePostgres) GetStoreByID(ctx context.Context, id string) (*Store, error) {
uid, err := uuid.Parse(id)
if err != nil {
return nil, err
}
s := &Store{}
err = r.db.QueryRowContext(ctx, `
SELECT id,vendor_id,name,description,address,city,country,phone,email,is_active,created_at,updated_at
FROM stores WHERE id=$1`, uid).
Scan(&s.ID, &s.VendorID, &s.Name, &s.Description, &s.Address,
&s.City, &s.Country, &s.Phone, &s.Email, &s.IsActive,
&s.CreatedAt, &s.UpdatedAt)
return s, err
}

func (r *storePostgres) ListStoresByVendor(ctx context.Context, vendorID string) ([]*Store, error) {
uid, err := uuid.Parse(vendorID)
if err != nil {
return nil, err
}
rows, err := r.db.QueryContext(ctx, `
SELECT id,vendor_id,name,description,address,city,country,phone,email,is_active,created_at,updated_at
FROM stores WHERE vendor_id=$1 ORDER BY created_at DESC`, uid)
if err != nil {
return nil, err
}
defer rows.Close()
var stores []*Store
for rows.Next() {
s := &Store{}
if err := rows.Scan(&s.ID, &s.VendorID, &s.Name, &s.Description, &s.Address,
&s.City, &s.Country, &s.Phone, &s.Email, &s.IsActive,
&s.CreatedAt, &s.UpdatedAt); err != nil {
return nil, err
}
stores = append(stores, s)
}
return stores, nil
}

// ---- StoreStaff ----

type staffPostgres struct{ db *sql.DB }

func NewStoreStaffPostgresRepository(db *sql.DB) StoreStaffRepository { return &staffPostgres{db: db} }

func (r *staffPostgres) AddStaff(ctx context.Context, staff *StoreStaff) error {
_, err := r.db.ExecContext(ctx, `
INSERT INTO store_staff (id,store_id,user_id,role) VALUES ($1,$2,$3,$4)`,
staff.ID, staff.StoreID, staff.UserID, staff.Role)
return err
}

func (r *staffPostgres) ListStaff(ctx context.Context, storeID string) ([]*StoreStaff, error) {
uid, err := uuid.Parse(storeID)
if err != nil {
return nil, err
}
rows, err := r.db.QueryContext(ctx, `
SELECT id,store_id,user_id,role,created_at,updated_at FROM store_staff WHERE store_id=$1`, uid)
if err != nil {
return nil, err
}
defer rows.Close()
var staff []*StoreStaff
for rows.Next() {
s := &StoreStaff{}
if err := rows.Scan(&s.ID, &s.StoreID, &s.UserID, &s.Role, &s.CreatedAt, &s.UpdatedAt); err != nil {
return nil, err
}
staff = append(staff, s)
}
return staff, nil
}

func (r *staffPostgres) RemoveStaff(ctx context.Context, storeID, userID string) error {
sid, err := uuid.Parse(storeID)
if err != nil {
return err
}
uid, err := uuid.Parse(userID)
if err != nil {
return err
}
_, err = r.db.ExecContext(ctx, `DELETE FROM store_staff WHERE store_id=$1 AND user_id=$2`, sid, uid)
return err
}

// ---- VendorStoreProduct ----

type productPostgres struct{ db *sql.DB }

func NewProductPostgresRepository(db *sql.DB) ProductRepository { return &productPostgres{db: db} }

func (r *productPostgres) AddProduct(ctx context.Context, p *VendorStoreProduct) error {
_, err := r.db.ExecContext(ctx, `
INSERT INTO vendor_store_products
  (id,store_id,platform_product_id,vendor_price,currency,stock_quantity,is_available)
VALUES ($1,$2,$3,$4,$5,$6,$7)`,
p.ID, p.StoreID, p.PlatformProductID, p.VendorPrice,
p.Currency, p.StockQuantity, p.IsAvailable)
return err
}

func (r *productPostgres) ListProducts(ctx context.Context, storeID string) ([]*VendorStoreProduct, error) {
uid, err := uuid.Parse(storeID)
if err != nil {
return nil, err
}
rows, err := r.db.QueryContext(ctx, `
SELECT id,store_id,platform_product_id,vendor_price,currency,stock_quantity,is_available,created_at,updated_at
FROM vendor_store_products WHERE store_id=$1 ORDER BY created_at DESC`, uid)
if err != nil {
return nil, err
}
defer rows.Close()
var products []*VendorStoreProduct
for rows.Next() {
p := &VendorStoreProduct{}
if err := rows.Scan(&p.ID, &p.StoreID, &p.PlatformProductID, &p.VendorPrice,
&p.Currency, &p.StockQuantity, &p.IsAvailable, &p.CreatedAt, &p.UpdatedAt); err != nil {
return nil, err
}
products = append(products, p)
}
return products, nil
}

func (r *productPostgres) UpdateStock(ctx context.Context, id string, qty int) error {
uid, err := uuid.Parse(id)
if err != nil {
return err
}
res, err := r.db.ExecContext(ctx,
`UPDATE vendor_store_products SET stock_quantity=$1, updated_at=NOW() WHERE id=$2`, qty, uid)
if err != nil {
return err
}
n, _ := res.RowsAffected()
if n == 0 {
return fmt.Errorf("product %s not found", id)
}
return nil
}

func (r *productPostgres) UpdateAvailability(ctx context.Context, id string, available bool) error {
uid, err := uuid.Parse(id)
if err != nil {
return err
}
_, err = r.db.ExecContext(ctx,
`UPDATE vendor_store_products SET is_available=$1, updated_at=NOW() WHERE id=$2`, available, uid)
return err
}
