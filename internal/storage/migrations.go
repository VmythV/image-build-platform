package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Migration struct {
	Version int
	Name    string
	SQL     string
}

func RunMigrations(ctx context.Context, db *sql.DB, driverName string) error {
	if err := ensureMigrationTable(ctx, db); err != nil {
		return err
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		applied, dirty, err := migrationStatus(ctx, db, driverName, migration.Version)
		if err != nil {
			return err
		}
		if dirty {
			return fmt.Errorf("migration %d is dirty; restore from backup or repair the migration state", migration.Version)
		}
		if applied {
			continue
		}
		if err := applyMigration(ctx, db, driverName, migration); err != nil {
			return err
		}
	}

	return nil
}

func ensureMigrationTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY,
	dirty INTEGER NOT NULL DEFAULT 0,
	applied_at TEXT NOT NULL
);`)
	if err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}
	return nil
}

func loadMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}

	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}

		version, err := parseMigrationVersion(entry.Name())
		if err != nil {
			return nil, err
		}

		path := filepath.ToSlash(filepath.Join("migrations", entry.Name()))
		data, err := migrationFiles.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    entry.Name(),
			SQL:     string(data),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func parseMigrationVersion(name string) (int, error) {
	parts := strings.SplitN(name, "_", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid migration filename %q", name)
	}
	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid migration version in %q: %w", name, err)
	}
	return version, nil
}

func migrationStatus(ctx context.Context, db *sql.DB, driverName string, version int) (applied bool, dirty bool, err error) {
	row := db.QueryRowContext(ctx, "SELECT dirty FROM schema_migrations WHERE version = "+placeholder(driverName, 1), version)

	var dirtyValue int
	if err := row.Scan(&dirtyValue); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("query migration status %d: %w", version, err)
	}

	return true, dirtyValue != 0, nil
}

func applyMigration(ctx context.Context, db *sql.DB, driverName string, migration Migration) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (version, dirty, applied_at) VALUES ("+placeholder(driverName, 1)+", 1, "+placeholder(driverName, 2)+")", migration.Version, now); err != nil {
		return fmt.Errorf("mark migration %s dirty: %w", migration.Name, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", migration.Name, err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, migration.SQL); err != nil {
		return fmt.Errorf("apply migration %s: %w", migration.Name, err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", migration.Name, err)
	}

	if _, err = db.ExecContext(ctx, "UPDATE schema_migrations SET dirty = 0, applied_at = "+placeholder(driverName, 1)+" WHERE version = "+placeholder(driverName, 2), now, migration.Version); err != nil {
		return fmt.Errorf("mark migration %s clean: %w", migration.Name, err)
	}

	return nil
}

func placeholder(driverName string, position int) string {
	if driverName == "pgx" {
		return "$" + strconv.Itoa(position)
	}
	return "?"
}
