package admin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type postgresRepository struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

// ── Platform Stats ────────────────────────────────────────────────────────────

func (r *postgresRepository) GetPlatformStats(ctx context.Context) (*PlatformStats, error) {
	stats := &PlatformStats{}
	queries := []struct {
		dest  interface{}
		query string
	}{
		{&stats.TotalUsers, "SELECT COUNT(*) FROM users"},
		{&stats.TotalVendors, "SELECT COUNT(*) FROM vendors"},
		{&stats.TotalStores, "SELECT COUNT(*) FROM stores"},
		{&stats.TotalOrders, "SELECT COUNT(*) FROM orders"},
		{&stats.TotalRevenue, "SELECT COALESCE(SUM(total_amount),0) FROM orders WHERE status NOT IN ('CANCELLED')"},
		{&stats.ActiveSubscriptions, "SELECT COUNT(*) FROM vendor_subscriptions WHERE status IN ('TRIAL','ACTIVE')"},
		{&stats.PendingOrders, "SELECT COUNT(*) FROM orders WHERE status = 'PENDING'"},
		{&stats.ProductionJobs, "SELECT COUNT(*) FROM production_jobs WHERE status IN ('QUEUED','IN_PROGRESS')"},
	}
	for _, q := range queries {
		if err := r.db.QueryRowContext(ctx, q.query).Scan(q.dest); err != nil {
			return nil, fmt.Errorf("stats query failed: %w", err)
		}
	}
	return stats, nil
}

// ── User Management ───────────────────────────────────────────────────────────

func (r *postgresRepository) ListUsers(ctx context.Context, role, search string, limit, offset int) ([]AdminUser, int, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	idx := 1

	if role != "" {
		where = append(where, fmt.Sprintf("role = $%d", idx))
		args = append(args, role)
		idx++
	}
	if search != "" {
		where = append(where, fmt.Sprintf("(email ILIKE $%d OR first_name ILIKE $%d OR last_name ILIKE $%d)", idx, idx, idx))
		args = append(args, "%"+search+"%")
		idx++
	}

	whereClause := strings.Join(where, " AND ")
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users WHERE %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT id, email, first_name, last_name, role, is_active, COALESCE(phone,''), created_at
		FROM users WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, whereClause, idx, idx+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []AdminUser
	for rows.Next() {
		var u AdminUser
		if err := rows.Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.Role, &u.IsActive, &u.Phone, &u.CreatedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, nil
}

func (r *postgresRepository) GetUser(ctx context.Context, id string) (*AdminUser, error) {
	var u AdminUser
	err := r.db.QueryRowContext(ctx, `
		SELECT id, email, first_name, last_name, role, is_active, COALESCE(phone,''), created_at
		FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.Role, &u.IsActive, &u.Phone, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found: %s", id)
	}
	return &u, err
}

func (r *postgresRepository) UpdateUser(ctx context.Context, id string, req UpdateUserRequest) (*AdminUser, error) {
	sets := []string{"updated_at = NOW()"}
	args := []interface{}{}
	idx := 1

	if req.Role != "" {
		sets = append(sets, fmt.Sprintf("role = $%d", idx))
		args = append(args, req.Role)
		idx++
	}
	if req.IsActive != nil {
		sets = append(sets, fmt.Sprintf("is_active = $%d", idx))
		args = append(args, *req.IsActive)
		idx++
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d", strings.Join(sets, ", "), idx)
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return nil, err
	}
	return r.GetUser(ctx, id)
}

func (r *postgresRepository) DeactivateUser(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET is_active = false, updated_at = NOW() WHERE id = $1", id)
	return err
}

func (r *postgresRepository) CreateAdministrator(ctx context.Context, request CreateAdministratorRequest, passwordHash string) (*AdminUser, error) {
	id := uuid.New().String()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, first_name, last_name, role, is_active)
		VALUES ($1, $2, $3, $4, $5, 'ADMIN', true)`,
		id, request.Email, passwordHash, request.FirstName, request.LastName)
	if err != nil {
		return nil, err
	}
	return r.GetUser(ctx, id)
}

func (r *postgresRepository) CountActiveAdministrators(ctx context.Context) (int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE role = 'ADMIN' AND is_active = true").Scan(&total)
	return total, err
}

// ── Vendor Management ─────────────────────────────────────────────────────────

func (r *postgresRepository) ListVendors(ctx context.Context, status, search string, limit, offset int) ([]AdminVendor, int, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	idx := 1

	if status != "" {
		where = append(where, fmt.Sprintf("v.status = $%d", idx))
		args = append(args, status)
		idx++
	}
	if search != "" {
		where = append(where, fmt.Sprintf("(v.business_name ILIKE $%d OR v.email ILIKE $%d)", idx, idx))
		args = append(args, "%"+search+"%")
		idx++
	}

	whereClause := strings.Join(where, " AND ")
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM vendors v WHERE %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT v.id, v.business_name, v.email, COALESCE(v.phone,''), v.status,
		       COALESCE(vt.name,'CORE'), v.created_at
		FROM vendors v
		LEFT JOIN vendor_tiers vt ON v.tier_id = vt.id
		WHERE %s
		ORDER BY v.created_at DESC
		LIMIT $%d OFFSET $%d`, whereClause, idx, idx+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var vendors []AdminVendor
	for rows.Next() {
		var v AdminVendor
		if err := rows.Scan(&v.ID, &v.BusinessName, &v.Email, &v.Phone, &v.Status, &v.TierName, &v.CreatedAt); err != nil {
			return nil, 0, err
		}
		vendors = append(vendors, v)
	}
	return vendors, total, nil
}

func (r *postgresRepository) GetVendor(ctx context.Context, id string) (*AdminVendor, error) {
	var v AdminVendor
	err := r.db.QueryRowContext(ctx, `
		SELECT v.id, v.business_name, v.email, COALESCE(v.phone,''), v.status,
		       COALESCE(vt.name,'CORE'), v.created_at
		FROM vendors v
		LEFT JOIN vendor_tiers vt ON v.tier_id = vt.id
		WHERE v.id = $1`, id).
		Scan(&v.ID, &v.BusinessName, &v.Email, &v.Phone, &v.Status, &v.TierName, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("vendor not found: %s", id)
	}
	return &v, err
}

func (r *postgresRepository) UpdateVendorStatus(ctx context.Context, id, status string) (*AdminVendor, error) {
	_, err := r.db.ExecContext(ctx, "UPDATE vendors SET status = $1, updated_at = NOW() WHERE id = $2", status, id)
	if err != nil {
		return nil, err
	}
	return r.GetVendor(ctx, id)
}

// ── Order Management ─────────────────────────────────────────────────────────

func (r *postgresRepository) ListOrders(ctx context.Context, status string, limit, offset int) ([]AdminOrder, int, error) {
	where := "1=1"
	args := []interface{}{}
	idx := 1

	if status != "" {
		where = fmt.Sprintf("o.status = $%d", idx)
		args = append(args, status)
		idx++
	}

	var total int
	if err := r.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM orders o WHERE %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT o.id, o.order_number, o.store_id, COALESCE(s.name,''), COALESCE(o.customer_id::text,''),
		       o.status, o.total_amount, o.created_at
		FROM orders o
		LEFT JOIN stores s ON o.store_id = s.id
		WHERE %s
		ORDER BY o.created_at DESC
		LIMIT $%d OFFSET $%d`, where, idx, idx+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var orders []AdminOrder
	for rows.Next() {
		var o AdminOrder
		if err := rows.Scan(&o.ID, &o.OrderNumber, &o.StoreID, &o.StoreName, &o.CustomerID, &o.Status, &o.TotalAmount, &o.CreatedAt); err != nil {
			return nil, 0, err
		}
		orders = append(orders, o)
	}
	return orders, total, nil
}

func (r *postgresRepository) GetOrder(ctx context.Context, id string) (*AdminOrder, error) {
	var o AdminOrder
	err := r.db.QueryRowContext(ctx, `
		SELECT o.id, o.order_number, o.store_id, COALESCE(s.name,''), COALESCE(o.customer_id::text,''),
		       o.status, o.total_amount, o.created_at
		FROM orders o
		LEFT JOIN stores s ON o.store_id = s.id
		WHERE o.id = $1`, id).
		Scan(&o.ID, &o.OrderNumber, &o.StoreID, &o.StoreName, &o.CustomerID, &o.Status, &o.TotalAmount, &o.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("order not found: %s", id)
	}
	return &o, err
}

// ── Subscription Management ───────────────────────────────────────────────────

func (r *postgresRepository) ListSubscriptions(ctx context.Context, status string, limit, offset int) ([]AdminSubscription, int, error) {
	where := "1=1"
	args := []interface{}{}
	idx := 1

	if status != "" {
		where = fmt.Sprintf("vs.status = $%d", idx)
		args = append(args, status)
		idx++
	}

	var total int
	if err := r.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM vendor_subscriptions vs WHERE %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT vs.id, vs.vendor_id, COALESCE(v.business_name,''), COALESCE(vt.name,''),
		       vs.status, vs.start_date, vs.end_date, vs.auto_renew
		FROM vendor_subscriptions vs
		LEFT JOIN vendors v ON vs.vendor_id = v.id
		LEFT JOIN vendor_tiers vt ON vs.tier_id = vt.id
		WHERE %s
		ORDER BY vs.created_at DESC
		LIMIT $%d OFFSET $%d`, where, idx, idx+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var subs []AdminSubscription
	for rows.Next() {
		var s AdminSubscription
		if err := rows.Scan(&s.ID, &s.VendorID, &s.VendorName, &s.TierName, &s.Status, &s.StartDate, &s.EndDate, &s.AutoRenew); err != nil {
			return nil, 0, err
		}
		subs = append(subs, s)
	}
	return subs, total, nil
}

// ── Audit Log ─────────────────────────────────────────────────────────────────

func (r *postgresRepository) CreateAuditLog(ctx context.Context, log AuditLog) error {
	log.ID = uuid.New().String()
	log.CreatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO admin_audit_logs (id, admin_id, action, target_type, target_id, details, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		log.ID, log.AdminID, log.Action, log.TargetType, log.TargetID, log.Details, log.CreatedAt)
	return err
}

func (r *postgresRepository) ListAuditLogs(ctx context.Context, adminID, targetType string, limit, offset int) ([]AuditLog, int, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	idx := 1

	if adminID != "" {
		where = append(where, fmt.Sprintf("admin_id = $%d", idx))
		args = append(args, adminID)
		idx++
	}
	if targetType != "" {
		where = append(where, fmt.Sprintf("target_type = $%d", idx))
		args = append(args, targetType)
		idx++
	}

	whereClause := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM admin_audit_logs WHERE %s", whereClause), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT id, admin_id, action, target_type, target_id, details, created_at
		FROM admin_audit_logs WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, whereClause, idx, idx+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		if err := rows.Scan(&l.ID, &l.AdminID, &l.Action, &l.TargetType, &l.TargetID, &l.Details, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	return logs, total, nil
}
