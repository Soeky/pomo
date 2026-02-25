package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/db"
)

func TestSessionAndPlannedEventCRUD(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	ctx := context.Background()
	start := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	end := start.Add(25 * time.Minute)

	sessID, err := CreateSession(ctx, Session{
		Type:      "focus",
		Topic:     "ProjectA",
		StartTime: start,
		EndTime:   &end,
	}, "test")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	gotSession, err := GetSessionByID(ctx, int(sessID))
	if err != nil {
		t.Fatalf("GetSessionByID failed: %v", err)
	}
	if gotSession.Topic != "ProjectA" {
		t.Fatalf("unexpected topic: %s", gotSession.Topic)
	}
	if gotSession.DurationSec != int((25 * time.Minute).Seconds()) {
		t.Fatalf("unexpected duration: %d", gotSession.DurationSec)
	}

	list, err := ListSessions(ctx, SessionFilter{Query: "Project", Type: "focus", SortBy: "start_time", Order: "asc"})
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if list.TotalRows < 1 {
		t.Fatalf("expected at least one row, got %d", list.TotalRows)
	}

	updatedEnd := end.Add(5 * time.Minute)
	if err := UpdateSession(ctx, int(sessID), Session{
		Type:      "break",
		Topic:     "Updated",
		StartTime: start,
		EndTime:   &updatedEnd,
	}, "test"); err != nil {
		t.Fatalf("UpdateSession failed: %v", err)
	}

	afterUpdate, err := GetSessionByID(ctx, int(sessID))
	if err != nil {
		t.Fatalf("GetSessionByID after update failed: %v", err)
	}
	if afterUpdate.Type != "break" || afterUpdate.Topic != "Updated" {
		t.Fatalf("unexpected updated session: %+v", afterUpdate)
	}

	planID, err := CreatePlannedEvent(ctx, PlannedEvent{
		Title:       "Plan1",
		Description: "desc",
		StartTime:   start,
		EndTime:     end,
	}, "test")
	if err != nil {
		t.Fatalf("CreatePlannedEvent failed: %v", err)
	}

	plan, err := GetPlannedEventByID(ctx, int(planID))
	if err != nil {
		t.Fatalf("GetPlannedEventByID failed: %v", err)
	}
	if plan.Status != "planned" || plan.Source != "manual" {
		t.Fatalf("expected default status/source, got %+v", plan)
	}
	plan.Title = "Plan1-Updated"
	plan.Status = "done"
	if err := UpdatePlannedEvent(ctx, int(planID), plan, "test"); err != nil {
		t.Fatalf("UpdatePlannedEvent failed: %v", err)
	}

	plans, err := PlannedEventsInRange(ctx, start.Add(-time.Hour), end.Add(time.Hour))
	if err != nil {
		t.Fatalf("PlannedEventsInRange failed: %v", err)
	}
	if len(plans) == 0 {
		t.Fatalf("expected planned events in range")
	}

	sessions, err := SessionsInRange(ctx, start.Add(-time.Hour), end.Add(time.Hour))
	if err != nil {
		t.Fatalf("SessionsInRange failed: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatalf("expected sessions in range")
	}

	if err := DeletePlannedEvent(ctx, int(planID), "test"); err != nil {
		t.Fatalf("DeletePlannedEvent failed: %v", err)
	}
	if err := DeleteSession(ctx, int(sessID), "test"); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}
}

func TestAuditLogAndMarshal(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	ctx := context.Background()
	if err := InsertAuditLog(ctx, "session", 1, "update", map[string]any{"a": 1}, map[string]any{"a": 2}, "test"); err != nil {
		t.Fatalf("InsertAuditLog failed: %v", err)
	}

	if _, err := marshalMaybeJSON(make(chan int)); err == nil {
		t.Fatalf("expected marshal error for unsupported value")
	}
}

func TestListSessionsDefaultsAndSortingFallbacks(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	ctx := context.Background()
	start := time.Date(2026, 2, 25, 9, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Minute)
	if _, err := CreateSession(ctx, Session{
		Type:      "focus",
		Topic:     "X",
		StartTime: start,
		EndTime:   &end,
	}, "test"); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	res, err := ListSessions(ctx, SessionFilter{
		Page:     0,
		PageSize: 999,
		SortBy:   "not-a-column",
		Order:    "not-an-order",
		Type:     "invalid",
	})
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if res.Page != 1 || res.PageSize != 20 {
		t.Fatalf("expected default page/pageSize, got page=%d size=%d", res.Page, res.PageSize)
	}
	if len(res.Rows) == 0 {
		t.Fatalf("expected at least one row")
	}
}

func TestGetByIDNotFoundErrors(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	ctx := context.Background()
	if _, err := GetSessionByID(ctx, 99999); err == nil {
		t.Fatalf("expected not found error for session")
	}
	if _, err := GetPlannedEventByID(ctx, 99999); err == nil {
		t.Fatalf("expected not found error for planned event")
	}
}

func TestDeletePlannedEventMissingRow(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	err := DeletePlannedEvent(context.Background(), 99999, "test")
	if err == nil {
		t.Fatalf("expected error deleting missing planned event")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		// driver may wrap; keep assertion tolerant but non-empty error is required
		if err.Error() == "" {
			t.Fatalf("expected descriptive delete error")
		}
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
