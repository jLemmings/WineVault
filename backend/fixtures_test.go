package main

import (
	"context"
	_ "embed"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/seed.sql
var initialSeed string

// Existing integration scenarios explicitly request example racks and inventory.
// Fresh production migration is covered separately in TestFreshInstanceIsEmpty.
func migrateFixture(ctx context.Context, pool *pgxpool.Pool, legacyPath string) error {
	if legacyPath != "" {
		return migrate(ctx, pool, legacyPath)
	}
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('schema_migrations') IS NOT NULL`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, initialSchema+"DELETE FROM cellars;"+legacyLayout+initialSeed+`
		CREATE TABLE schema_migrations (
		    version integer PRIMARY KEY,
		    applied_at timestamptz NOT NULL DEFAULT now()
		);

		INSERT INTO schema_migrations (version)
		    VALUES (1);
	`); err != nil {
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
	}
	return migrate(ctx, pool, legacyPath)
}
