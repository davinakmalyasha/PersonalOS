package db

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/pressly/goose/v3"
	_ "github.com/mattn/go-sqlite3"
)

func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?cache=shared&_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000&_synchronous=NORMAL", filepath.ToSlash(path))
	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if _, err := sqlDB.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("pragma journal_mode: %w", err)
	}
	if _, err := sqlDB.Exec(`PRAGMA foreign_keys=ON;`); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("pragma foreign_keys: %w", err)
	}
	return sqlDB, nil
}

// Migrate runs goose up from an on-disk migrations directory.
// migrationsDir is a filesystem path (e.g., "services/api/migrations").
func Migrate(sqlDB *sql.DB, migrationsDir string) error {
	goose.SetBaseFS(nil)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
