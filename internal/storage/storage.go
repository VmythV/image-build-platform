package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/VmythV/image-build-platform/internal/config"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type Store struct {
	DB         *sql.DB
	DriverName string
}

func Open(ctx context.Context, cfg config.Config) (*Store, error) {
	if err := ensureStorageDirs(cfg.Storage); err != nil {
		return nil, err
	}

	driverName, dsn, err := databaseDriver(cfg.Database)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if driverName == "sqlite" {
		if err := configureSQLite(ctx, db); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := RunMigrations(ctx, db, driverName); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{DB: db, DriverName: driverName}, nil
}

func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

func ensureStorageDirs(cfg config.StorageConfig) error {
	dirs := []string{
		cfg.DataDir,
		cfg.LogDir,
		cfg.ContextDir,
		cfg.TmpDir,
		cfg.BackupDir,
	}
	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create directory %q: %w", dir, err)
		}
	}
	return nil
}

func databaseDriver(cfg config.DatabaseConfig) (string, string, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	switch driver {
	case "sqlite", "sqlite3":
		if err := os.MkdirAll(filepath.Dir(cfg.DSN), 0o750); err != nil && filepath.Dir(cfg.DSN) != "." {
			return "", "", fmt.Errorf("create sqlite database directory: %w", err)
		}
		return "sqlite", cfg.DSN, nil
	case "postgres", "postgresql", "pgx":
		return "pgx", cfg.DSN, nil
	default:
		return "", "", fmt.Errorf("unsupported database driver %q", cfg.Driver)
	}
}

func configureSQLite(ctx context.Context, db *sql.DB) error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	}
	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("configure sqlite %q: %w", pragma, err)
		}
	}
	return nil
}
