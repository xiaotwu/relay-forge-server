package database

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const createMigrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version     TEXT PRIMARY KEY,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

// MigrateUp applies all pending up-migrations in order.
func MigrateUp(pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if _, err := pool.Exec(ctx, createMigrationsTable); err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	files, err := collectMigrations("up")
	if err != nil {
		return err
	}

	for _, mf := range files {
		applied, err := isMigrationApplied(ctx, pool, mf.version)
		if err != nil {
			return err
		}
		if applied {
			log.Debug().Str("version", mf.version).Msg("migration already applied, skipping")
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + mf.filename)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", mf.filename, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("beginning transaction for %s: %w", mf.version, err)
		}

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("executing migration %s: %w", mf.version, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", mf.version); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("recording migration %s: %w", mf.version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("committing migration %s: %w", mf.version, err)
		}

		log.Info().Str("version", mf.version).Msg("migration applied")
	}

	return nil
}

// MigrateDown rolls back the most recently applied migration.
func MigrateDown(pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if _, err := pool.Exec(ctx, createMigrationsTable); err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	var version string
	err := pool.QueryRow(ctx, "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&version)
	if err != nil {
		return fmt.Errorf("finding latest migration: %w", err)
	}

	downFiles, err := collectMigrations("down")
	if err != nil {
		return err
	}

	var target *migrationFile
	for i := range downFiles {
		if downFiles[i].version == version {
			target = &downFiles[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("no down migration found for version %s", version)
	}

	content, err := migrationsFS.ReadFile("migrations/" + target.filename)
	if err != nil {
		return fmt.Errorf("reading migration %s: %w", target.filename, err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction for rollback %s: %w", version, err)
	}

	if _, err := tx.Exec(ctx, string(content)); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("executing down migration %s: %w", version, err)
	}

	if _, err := tx.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", version); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("removing migration record %s: %w", version, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing rollback %s: %w", version, err)
	}

	log.Info().Str("version", version).Msg("migration rolled back")
	return nil
}

type migrationFile struct {
	version  string
	filename string
}

func collectMigrations(direction string) ([]migrationFile, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("reading migrations directory: %w", err)
	}

	suffix := "." + direction + ".sql"
	var files []migrationFile

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, suffix) {
			continue
		}
		// Extract version: everything before the first underscore-separated suffix
		// e.g., "000001_initial_schema.up.sql" -> "000001"
		parts := strings.SplitN(name, "_", 2)
		if len(parts) < 2 {
			continue
		}
		files = append(files, migrationFile{
			version:  parts[0],
			filename: name,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].version < files[j].version
	})

	return files, nil
}

func isMigrationApplied(ctx context.Context, pool *pgxpool.Pool, version string) (bool, error) {
	var count int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = $1", version).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("checking migration status for %s: %w", version, err)
	}
	return count > 0, nil
}
