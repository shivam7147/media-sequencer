// Package database owns the Postgres connection pool, schema migrations,
// and seed data for the media sequencer.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a connection pool against databaseURL and pings it once so
// startup fails fast (and loudly) instead of surfacing on the first request.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing DATABASE_URL: %w", err)
	}

	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return pool, nil
}

// schema is applied on every boot. Every statement is idempotent so running
// it against an already-migrated database is a no-op.
const schema = `
CREATE TABLE IF NOT EXISTS media (
	id                        BIGSERIAL PRIMARY KEY,
	label                     TEXT NOT NULL,
	kind                      TEXT NOT NULL CHECK (kind IN ('image', 'video', 'blank')),
	url                       TEXT NOT NULL,
	default_duration_seconds INTEGER NOT NULL CHECK (default_duration_seconds > 0)
);

CREATE TABLE IF NOT EXISTS windows (
	id       BIGSERIAL PRIMARY KEY,
	name     TEXT NOT NULL,
	position INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS window_items (
	id               BIGSERIAL PRIMARY KEY,
	window_id        BIGINT NOT NULL REFERENCES windows(id) ON DELETE CASCADE,
	media_id         BIGINT NOT NULL REFERENCES media(id) ON DELETE RESTRICT,
	position         INTEGER NOT NULL,
	duration_seconds INTEGER NOT NULL CHECK (duration_seconds > 0)
);

CREATE INDEX IF NOT EXISTS idx_window_items_window_position
	ON window_items (window_id, position);

CREATE TABLE IF NOT EXISTS sync_events (
	id               BIGSERIAL PRIMARY KEY,
	media_id         BIGINT NOT NULL REFERENCES media(id) ON DELETE RESTRICT,
	start_at         TIMESTAMPTZ NOT NULL,
	duration_seconds INTEGER NOT NULL CHECK (duration_seconds > 0),
	created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`

// Migrate creates every table and index the app needs if they don't already exist.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	return nil
}
