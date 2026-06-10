package postgres

import (
	"context"
	"database/sql"
)

// WithAdvisoryLock runs fn only when this process wins the named advisory
// lock, so scheduler jobs stay single-flight across replicas. It returns
// (false, nil) when another holder has the lock.
func WithAdvisoryLock(ctx context.Context, db *sql.DB, name string, fn func(context.Context) error) (bool, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	var acquired bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(hashtext($1))`, name).Scan(&acquired); err != nil {
		return false, err
	}
	if !acquired {
		return false, nil
	}
	defer func() {
		_, _ = conn.ExecContext(ctx, `SELECT pg_advisory_unlock(hashtext($1))`, name)
	}()
	return true, fn(ctx)
}

// ListTenants returns known tenant IDs for per-tenant maintenance jobs.
func ListTenants(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT id FROM tenants ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tenants []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		tenants = append(tenants, id)
	}
	return tenants, rows.Err()
}
