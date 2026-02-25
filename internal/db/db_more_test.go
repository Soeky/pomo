package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGetDBPathAndInitDB(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	p := GetDBPath()
	if !strings.Contains(p, filepath.Join(".local", "share", "pomo", "pomo.db")) {
		t.Fatalf("unexpected db path: %s", p)
	}

	prev := DB
	defer func() { DB = prev }()

	if err := InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if DB == nil {
		t.Fatalf("expected DB to be initialized")
	}
	_ = DB.Close()
}

func TestInsertSessionAndGetCurrentSession(t *testing.T) {
	opened, err := Open(filepath.Join(t.TempDir(), "pomo.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer opened.Close()

	prev := DB
	DB = opened
	defer func() { DB = prev }()

	if _, err := InsertSession("focus", "Topic1", 25*time.Minute); err != nil {
		t.Fatalf("InsertSession failed: %v", err)
	}
	s, err := GetCurrentSession()
	if err != nil {
		t.Fatalf("GetCurrentSession failed: %v", err)
	}
	if s == nil || s.Topic != "Topic1" {
		t.Fatalf("unexpected current session: %+v", s)
	}
}

func TestOpenFailsOnMigrationChecksumMismatch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mismatch-open.db")
	seed, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("seed open failed: %v", err)
	}
	if _, err := seed.Exec(`
		CREATE TABLE schema_migrations (
			name TEXT PRIMARY KEY,
			checksum TEXT NOT NULL,
			applied_at DATETIME NOT NULL
		);
		INSERT INTO schema_migrations(name, checksum, applied_at)
		VALUES ('001_base_sessions', 'wrong-checksum', CURRENT_TIMESTAMP);
	`); err != nil {
		_ = seed.Close()
		t.Fatalf("seed schema failed: %v", err)
	}
	_ = seed.Close()

	if _, err := Open(dbPath); err == nil {
		t.Fatalf("expected Open to fail on migration checksum mismatch")
	}
}

func TestStopCurrentSessionSuccess(t *testing.T) {
	opened, err := Open(filepath.Join(t.TempDir(), "pomo.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer opened.Close()

	prev := DB
	DB = opened
	defer func() { DB = prev }()

	start := time.Now().UTC().Add(-2 * time.Minute)
	if _, err := DB.Exec(`INSERT INTO sessions(type, topic, start_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"focus", "stop-me", start, 600, start, start); err != nil {
		t.Fatalf("seed session failed: %v", err)
	}
	if err := StopCurrentSession(); err != nil {
		t.Fatalf("StopCurrentSession failed: %v", err)
	}
}
