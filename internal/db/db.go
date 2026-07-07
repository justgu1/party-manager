package db

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Connect opens a pgx connection pool and pings the database.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// Migrate applies all up migrations embedded in the binary. It is safe to call
// on every boot: already-applied migrations are skipped.
func Migrate(databaseURL string) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}
	// golang-migrate's pgx driver wants the "pgx5://" style URL scheme.
	m, err := migrate.NewWithSourceInstance("iofs", src, pgxURL(databaseURL))
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// pgxURL rewrites a postgres:// URL to the scheme expected by the migrate pgx/v5 driver.
func pgxURL(databaseURL string) string {
	// migrate registers the driver under "pgx5".
	if len(databaseURL) > 11 && databaseURL[:11] == "postgres://" {
		return "pgx5://" + databaseURL[11:]
	}
	if len(databaseURL) > 13 && databaseURL[:13] == "postgresql://" {
		return "pgx5://" + databaseURL[13:]
	}
	return databaseURL
}

// ensure the pgx driver is registered.
var _ = migratepgx.Postgres{}
