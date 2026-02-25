package web

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/store"
)

func TestStoreAdapterMethods(t *testing.T) {
	opened := openWebDB(t)
	defer opened.Close()

	a := storeAdapter{}
	ctx := context.Background()
	start := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	end := start.Add(20 * time.Minute)

	sid, err := a.CreateSession(ctx, store.Session{
		Type:      "focus",
		Topic:     "A",
		StartTime: start,
		EndTime:   &end,
	}, "test")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if _, err := a.ListSessions(ctx, store.SessionFilter{}); err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if _, err := a.GetSessionByID(ctx, int(sid)); err != nil {
		t.Fatalf("GetSessionByID failed: %v", err)
	}
	if err := a.UpdateSession(ctx, int(sid), store.Session{
		Type:      "break",
		Topic:     "B",
		StartTime: start,
		EndTime:   &end,
	}, "test"); err != nil {
		t.Fatalf("UpdateSession failed: %v", err)
	}
	if _, err := a.SessionsInRange(ctx, start.Add(-time.Hour), end.Add(time.Hour)); err != nil {
		t.Fatalf("SessionsInRange failed: %v", err)
	}

	pid, err := a.CreatePlannedEvent(ctx, store.PlannedEvent{
		Title:       "P1",
		Description: "d",
		StartTime:   start,
		EndTime:     end,
	}, "test")
	if err != nil {
		t.Fatalf("CreatePlannedEvent failed: %v", err)
	}
	if _, err := a.GetPlannedEventByID(ctx, int(pid)); err != nil {
		t.Fatalf("GetPlannedEventByID failed: %v", err)
	}
	if err := a.UpdatePlannedEvent(ctx, int(pid), store.PlannedEvent{
		Title:       "P2",
		Description: "d",
		StartTime:   start,
		EndTime:     end,
		Status:      "planned",
		Source:      "manual",
	}, "test"); err != nil {
		t.Fatalf("UpdatePlannedEvent failed: %v", err)
	}
	if _, err := a.PlannedEventsInRange(ctx, start.Add(-time.Hour), end.Add(time.Hour)); err != nil {
		t.Fatalf("PlannedEventsInRange failed: %v", err)
	}
	if err := a.InsertAuditLog(ctx, "x", 1, "act", map[string]any{"a": 1}, map[string]any{"a": 2}, "test"); err != nil {
		t.Fatalf("InsertAuditLog failed: %v", err)
	}
	if err := a.DeletePlannedEvent(ctx, int(pid), "test"); err != nil {
		t.Fatalf("DeletePlannedEvent failed: %v", err)
	}
	if err := a.DeleteSession(ctx, int(sid), "test"); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}
}

func openWebDB(t *testing.T) *sql.DB {
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
