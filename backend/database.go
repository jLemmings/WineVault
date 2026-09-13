package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
)

//go:embed migrations/001_initial.sql
var initialSchema string

//go:embed migrations/seed.sql
var initialSeed string

//go:embed migrations/002_editors.sql
var editorSchema string

//go:embed migrations/003_non_vintage.sql
var nonVintageSchema string

//go:embed migrations/004_barcodes.sql
var barcodeSchema string

//go:embed migrations/005_history.sql
var historySchema string

//go:embed migrations/006_auth.sql
var authSchema string

//go:embed migrations/007_wine_information.sql
var informationSchema string

//go:embed migrations/008_information_search_cache.sql
var informationSearchSchema string

//go:embed migrations/009_shared_wine_information.sql
var sharedInformationSchema string

// Schema, seed/import, and migration marker commit together exactly once.
func migrate(ctx context.Context, pool *pgxpool.Pool, legacyPath string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(874193205)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
		return err
	}
	var applied bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=1)").Scan(&applied); err != nil {
		return err
	}
	if applied {
		return migrateEditors(ctx, tx)
	}
	if _, err = tx.Exec(ctx, initialSchema); err != nil {
		return err
	}
	var data []byte
	if legacyPath != "" {
		data, err = os.ReadFile(legacyPath)
	} else {
		err = os.ErrNotExist
	}
	if os.IsNotExist(err) {
		_, err = tx.Exec(ctx, initialSeed)
	} else if err == nil {
		var bottles []Bottle
		if err = json.Unmarshal(data, &bottles); err != nil {
			return fmt.Errorf("read legacy inventory: %w", err)
		}
		for _, b := range bottles {
			if !validBottle(b) {
				return fmt.Errorf("invalid legacy bottle %q; original file unchanged", b.ID)
			}
			if _, err = tx.Exec(ctx, `INSERT INTO bottles(name,vintage,region,wine_type,rack_id,slot) VALUES($1,$2,$3,$4,$5,$6)`, b.Name, b.Vintage, b.Region, b.Type, b.Rack, b.Slot); err != nil {
				return fmt.Errorf("import legacy bottle %q: %w", b.ID, err)
			}
		}
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES(1)"); err != nil {
		return err
	}
	return migrateEditors(ctx, tx)
}

func migrateEditors(ctx context.Context, tx pgx.Tx) error {
	var applied bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=2)").Scan(&applied); err != nil {
		return err
	}
	if !applied {
		if _, err := tx.Exec(ctx, editorSchema); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES(2)"); err != nil {
			return err
		}
	}
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=3)").Scan(&applied); err != nil {
		return err
	}
	if !applied {
		if _, err := tx.Exec(ctx, nonVintageSchema); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES(3)"); err != nil {
			return err
		}
	}
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=4)").Scan(&applied); err != nil {
		return err
	}
	if !applied {
		if _, err := tx.Exec(ctx, barcodeSchema); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES(4)"); err != nil {
			return err
		}
	}
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=5)").Scan(&applied); err != nil {
		return err
	}
	if !applied {
		if _, err := tx.Exec(ctx, historySchema); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES(5)"); err != nil {
			return err
		}
	}
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=6)").Scan(&applied); err != nil {
		return err
	}
	if !applied {
		if _, err := tx.Exec(ctx, authSchema); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES(6)"); err != nil {
			return err
		}
	}
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=7)").Scan(&applied); err != nil {
		return err
	}
	if !applied {
		if _, err := tx.Exec(ctx, informationSchema); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES(7)"); err != nil {
			return err
		}
	}
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=8)").Scan(&applied); err != nil {
		return err
	}
	if !applied {
		if _, err := tx.Exec(ctx, informationSearchSchema); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES(8)"); err != nil {
			return err
		}
	}
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=9)").Scan(&applied); err != nil {
		return err
	}
	if !applied {
		if _, err := tx.Exec(ctx, sharedInformationSchema); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES(9)"); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
