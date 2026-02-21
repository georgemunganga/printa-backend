package production

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type postgresRepo struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) Repository { return &postgresRepo{db: db} }

func (r *postgresRepo) Create(ctx context.Context, job *ProductionJob) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO production_jobs (id, order_id, store_id, assigned_to, status, priority, notes, due_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		job.ID, job.OrderID, job.StoreID, job.AssignedTo,
		job.Status, job.Priority, job.Notes, job.DueAt)
	return err
}

func (r *postgresRepo) GetByID(ctx context.Context, id string) (*ProductionJob, error) {
	return r.scan(r.db.QueryRowContext(ctx, `
		SELECT id,order_id,store_id,assigned_to,status,priority,notes,
		       started_at,completed_at,due_at,created_at,updated_at
		FROM production_jobs WHERE id=$1`, id))
}

func (r *postgresRepo) GetByOrderID(ctx context.Context, orderID string) (*ProductionJob, error) {
	return r.scan(r.db.QueryRowContext(ctx, `
		SELECT id,order_id,store_id,assigned_to,status,priority,notes,
		       started_at,completed_at,due_at,created_at,updated_at
		FROM production_jobs WHERE order_id=$1 ORDER BY created_at DESC LIMIT 1`, orderID))
}

func (r *postgresRepo) ListByStore(ctx context.Context, storeID string, status string) ([]*ProductionJob, error) {
	query := `SELECT id,order_id,store_id,assigned_to,status,priority,notes,
	                 started_at,completed_at,due_at,created_at,updated_at
	          FROM production_jobs WHERE store_id=$1`
	args := []interface{}{storeID}
	if status != "" {
		query += " AND status=$2"
		args = append(args, status)
	}
	query += " ORDER BY priority ASC, created_at ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []*ProductionJob
	for rows.Next() {
		j, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (r *postgresRepo) ListByAssignee(ctx context.Context, userID string) ([]*ProductionJob, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id,order_id,store_id,assigned_to,status,priority,notes,
		       started_at,completed_at,due_at,created_at,updated_at
		FROM production_jobs WHERE assigned_to=$1
		ORDER BY priority ASC, created_at ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []*ProductionJob
	for rows.Next() {
		j, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (r *postgresRepo) UpdateStatus(ctx context.Context, id string, status JobStatus, notes string) error {
	now := time.Now()
	var startedAt, completedAt interface{}
	if status == JobInProgress {
		startedAt = now
	}
	if status == JobCompleted {
		completedAt = now
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE production_jobs
		SET status=$1, notes=COALESCE(NULLIF($2,''), notes),
		    started_at=COALESCE($3, started_at),
		    completed_at=COALESCE($4, completed_at),
		    updated_at=$5
		WHERE id=$6`,
		status, notes, startedAt, completedAt, now, id)
	return err
}

func (r *postgresRepo) UpdateAssignee(ctx context.Context, id string, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `UPDATE production_jobs SET assigned_to=$1, updated_at=$2 WHERE id=$3`,
		uid, time.Now(), id)
	return err
}

func (r *postgresRepo) CountActiveByStore(ctx context.Context, storeID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM production_jobs
		WHERE store_id=$1 AND status IN ('QUEUED','IN_PROGRESS')`, storeID).Scan(&count)
	return count, err
}

// ── scanner ───────────────────────────────────────────────────────────────────

type rowScanner interface{ Scan(dest ...interface{}) error }

func (r *postgresRepo) scan(row rowScanner) (*ProductionJob, error) {
	j := &ProductionJob{}
	var assignedTo sql.NullString
	var startedAt, completedAt, dueAt sql.NullTime
	err := row.Scan(&j.ID, &j.OrderID, &j.StoreID, &assignedTo, &j.Status,
		&j.Priority, &j.Notes, &startedAt, &completedAt, &dueAt,
		&j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if assignedTo.Valid {
		uid, _ := uuid.Parse(assignedTo.String)
		j.AssignedTo = &uid
	}
	if startedAt.Valid {
		j.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		j.CompletedAt = &completedAt.Time
	}
	if dueAt.Valid {
		j.DueAt = &dueAt.Time
	}
	return j, nil
}
