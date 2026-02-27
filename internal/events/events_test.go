package events

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/db"
)

func TestCreateAndListInRange(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	start := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	end := start.Add(90 * time.Minute)

	id, err := Create(context.Background(), Event{
		Kind:      "task",
		Title:     "Study Block",
		Domain:    "Math",
		Subtopic:  "Discrete Probability",
		StartTime: start,
		EndTime:   end,
		Layer:     "planned",
		Status:    "planned",
		Source:    "manual",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}

	rows, err := ListInRange(context.Background(), start.Add(-time.Minute), end.Add(time.Minute))
	if err != nil {
		t.Fatalf("ListInRange failed: %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("expected at least one event")
	}
	if rows[0].Domain != "Math" || rows[0].Subtopic != "Discrete Probability" {
		t.Fatalf("unexpected topic path: %+v", rows[0])
	}
}

func TestCreateUsesSubtopicDefault(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	start := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)

	id, err := Create(context.Background(), Event{
		Title:     "Math Session",
		Domain:    "Math",
		StartTime: start,
		EndTime:   end,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var domain, subtopic string
	if err := opened.QueryRow(`SELECT domain, subtopic FROM events WHERE id = ?`, id).Scan(&domain, &subtopic); err != nil {
		t.Fatalf("query event failed: %v", err)
	}
	if domain != "Math" || subtopic != "General" {
		t.Fatalf("unexpected topic defaults: domain=%s subtopic=%s", domain, subtopic)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	opened, err := db.Open(filepath.Join(t.TempDir(), "pomo.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	prev := db.DB
	db.DB = opened
	t.Cleanup(func() { db.DB = prev })
	return opened
}
