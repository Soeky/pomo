package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenRunsMigrationsOnFreshDatabase(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "pomo.db")
	opened, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer opened.Close()

	tables := []string{"sessions", "planned_events", "audit_log", "schema_migrations"}
	for _, tbl := range tables {
		if !tableExists(t, opened, tbl) {
			t.Fatalf("expected table %q to exist", tbl)
		}
	}

	var count int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count < 6 {
		t.Fatalf("expected at least 6 migrations, got %d", count)
	}
}

func TestRunMigrationsSupportsLegacySchemaMigrationsTable(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	legacyDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	defer legacyDB.Close()

	if _, err := legacyDB.Exec(`
		CREATE TABLE schema_migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			applied_at DATETIME NOT NULL
		);
		CREATE TABLE sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT NOT NULL CHECK(type IN ('focus', 'break')),
			topic TEXT,
			start_time DATETIME NOT NULL,
			end_time DATETIME,
			duration INTEGER,
			planned_event_id INTEGER,
			created_at DATETIME,
			updated_at DATETIME
		);
	`); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}

	now := time.Now()
	if _, err := legacyDB.Exec(`INSERT INTO schema_migrations(name, applied_at) VALUES (?, ?), (?, ?)`, "001_base_sessions", now, "002_sessions_metadata", now); err != nil {
		t.Fatalf("seed legacy migration rows: %v", err)
	}

	if err := RunMigrations(context.Background(), legacyDB); err != nil {
		t.Fatalf("RunMigrations failed for legacy schema: %v", err)
	}

	var checksum string
	if err := legacyDB.QueryRow(`SELECT checksum FROM schema_migrations WHERE name = ?`, "001_base_sessions").Scan(&checksum); err != nil {
		t.Fatalf("read upgraded checksum: %v", err)
	}
	if checksum == "" || checksum == "legacy" {
		t.Fatalf("expected upgraded checksum for legacy row, got %q", checksum)
	}
}

func TestRunMigrationsChecksumMismatch(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "mismatch.db")
	opened, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer opened.Close()

	if _, err := opened.Exec(`
		CREATE TABLE schema_migrations (
			name TEXT PRIMARY KEY,
			checksum TEXT NOT NULL,
			applied_at DATETIME NOT NULL
		);
		INSERT INTO schema_migrations(name, checksum, applied_at) VALUES ('001_base_sessions', 'wrong-checksum', CURRENT_TIMESTAMP);
	`); err != nil {
		t.Fatalf("seed mismatch table failed: %v", err)
	}

	if err := RunMigrations(context.Background(), opened); err == nil {
		t.Fatalf("expected checksum mismatch error")
	}
}

func TestIsDuplicateColumnErr(t *testing.T) {
	t.Parallel()

	if isDuplicateColumnErr(nil) {
		t.Fatalf("nil error must not be duplicate column")
	}
	if isDuplicateColumnErr(sql.ErrNoRows) {
		t.Fatalf("sql.ErrNoRows must not match duplicate column")
	}
	if !isDuplicateColumnErr(assertErr("duplicate column name: x")) {
		t.Fatalf("expected duplicate column detection")
	}
}

type textErr string

func (e textErr) Error() string { return string(e) }

func assertErr(s string) error { return textErr(s) }

func tableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()

	var count int
	if err := db.QueryRow(`SELECT COUNT(1) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil {
		t.Fatalf("tableExists query failed: %v", err)
	}
	return count == 1
}
