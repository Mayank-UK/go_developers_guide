package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// connectTimeout bounds pool creation + first contact: an unreachable
// PostgreSQL is a boot failure by design (it is part of the stack), and it
// must fail in seconds, not in TCP-stack-default minutes.
const connectTimeout = 10 * time.Second

// NewPool dials PostgreSQL with the configured DSN and proves the
// connection with a ping before returning the pool. The pool is the
// process-wide connection owner: acquire/release discipline, health
// checking, and max-connection bounds live here once, not in every caller.
// A DSN that parses but cannot be reached fails here — fail-fast at boot
// beats first-request surprises.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Migrate runs the embedded goose migrations to the latest version and
// returns the number of migrations applied by THIS call. On a fresh
// database that is the whole set; on an already-migrated one it is zero —
// goose's version table (goose_db_version, created in-band) makes the
// second call a no-op, which is what makes re-deploys safe. The pool rides
// through database/sql: goose's runner is built on it, and pgx's
// OpenDBFromPool adapts the SAME pool (no second connection pool, and
// closing the adapter still leaves the pool alive) to the interface goose
// needs.
func Migrate(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, Migrations())
	if err != nil {
		return 0, fmt.Errorf("build migration provider: %w", err)
	}
	applied, err := provider.Up(ctx)
	if err != nil {
		return 0, fmt.Errorf("apply migrations: %w", err)
	}
	return len(applied), nil
}
