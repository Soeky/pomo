package db

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionLifecycleOperations(t *testing.T) {
	opened, err := Open(filepath.Join(t.TempDir(), "pomo.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer opened.Close()

	prev := DB
	DB = opened
	defer func() { DB = prev }()

	start := time.Now().Add(-10 * time.Minute).UTC()
	if _, err := DB.Exec(`INSERT INTO sessions(type, topic, start_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"focus", "DeepWork", start, 1500, start, start); err != nil {
		t.Fatalf("insert seed session: %v", err)
	}

	current, err := GetCurrentSession()
	if err != nil {
		t.Fatalf("GetCurrentSession failed: %v", err)
	}
	if current == nil {
		t.Fatalf("expected running session")
	}
	if current.Topic != "DeepWork" {
		t.Fatalf("unexpected topic: %s", current.Topic)
	}

	end := start.Add(5 * time.Minute)
	if err := StopCurrentSessionAt(end); err != nil {
		t.Fatalf("StopCurrentSessionAt failed: %v", err)
	}

	var gotDuration int
	var gotEnd time.Time
	if err := DB.QueryRow(`SELECT duration, end_time FROM sessions WHERE id = ?`, current.ID).Scan(&gotDuration, &gotEnd); err != nil {
		t.Fatalf("read stopped session: %v", err)
	}
	if gotDuration != int((5 * time.Minute).Seconds()) {
		t.Fatalf("unexpected duration: got=%d want=%d", gotDuration, int((5 * time.Minute).Seconds()))
	}
	if delta := gotEnd.Sub(end); delta < -time.Second || delta > time.Second {
		t.Fatalf("unexpected end time: got=%v want=%v", gotEnd, end)
	}
}

func TestStopCurrentSessionReturnsErrorWhenMissing(t *testing.T) {
	opened, err := Open(filepath.Join(t.TempDir(), "pomo.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer opened.Close()

	prev := DB
	DB = opened
	defer func() { DB = prev }()

	if err := StopCurrentSession(); !errors.Is(err, ErrNoRunningSession) {
		t.Fatalf("expected ErrNoRunningSession, got %v", err)
	}
}
