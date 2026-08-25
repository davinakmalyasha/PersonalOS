package db

import (
	"path/filepath"
	"testing"
)

func TestOpenAndMigrate(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	sqlDB, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := Migrate(sqlDB, "../../migrations"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	// Verify app_meta exists
	var val string
	if err := sqlDB.QueryRow(`SELECT value FROM app_meta WHERE key='schema_version'`).Scan(&val); err != nil {
		t.Fatalf("query app_meta: %v", err)
	}
	if val != "00001" {
		t.Fatalf("expected schema_version 00001, got %q", val)
	}
}
