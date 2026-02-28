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
	if _, err := InsertSessionAt("focus", "DeepWork", start, 25*time.Minute); err != nil {
		t.Fatalf("insert seed tracked event: %v", err)
	}

	current, err := GetCurrentSession()
	if err != nil {
		t.Fatalf("GetCurrentSession failed: %v", err)
	}
	if current == nil {
		t.Fatalf("expected running session")
	}
	if current.Topic != "DeepWork::General" {
		t.Fatalf("unexpected topic: %s", current.Topic)
	}

	end := start.Add(5 * time.Minute)
	if err := StopCurrentSessionAt(end); err != nil {
		t.Fatalf("StopCurrentSessionAt failed: %v", err)
	}

	var gotDuration int
	var gotEnd time.Time
	if err := DB.QueryRow(`SELECT duration, end_time FROM events WHERE id = ?`, current.ID).Scan(&gotDuration, &gotEnd); err != nil {
		t.Fatalf("read stopped event: %v", err)
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
