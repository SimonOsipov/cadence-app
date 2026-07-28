// Package database owns the Postgres connection pools and the migration
// runner. It knows nothing about the domain: the bounded contexts receive a
// pool and write their own queries.
package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a connection pool and verifies it can reach the database, so
// that an unreachable Postgres fails the process at startup rather than on the
// first request.
//
// The URL passed here is the request-path role: low-privilege, unable to
// bypass RLS. The service role gets its own pool and is reserved for system
// jobs.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return pool, nil
}

// HealthCheck reports whether the pool can still serve a trivial query. It is
// the probe behind /healthz.
func HealthCheck(ctx context.Context, pool *pgxpool.Pool) error {
	var one int
	if err := pool.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
		return fmt.Errorf("health check query: %w", err)
	}

	return nil
}
