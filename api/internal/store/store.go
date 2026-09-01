// Package store adapts the internal/store/db data access layer to the
// domain-level Store interfaces internal/job and internal/auth depend on,
// translating pgx.ErrNoRows into each domain's own ErrNotFound.
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a pgx connection pool and verifies connectivity with a
// ping, so a misconfigured DATABASE_URL fails at startup rather than on
// the first request.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("store: create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping database: %w", err)
	}
	return pool, nil
}
