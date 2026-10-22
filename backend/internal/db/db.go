// Package db owns the PostgreSQL connection pool. The API is the only
// database client (ADR-02).
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open creates a connection pool. Connections are made lazily, so the API can
// start and report "not ready" while the database is still waking up.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		// pgx error text can include the connection string; do not wrap it.
		return nil, fmt.Errorf("parse DATABASE_URL: invalid connection string")
	}
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	return pool, nil
}
