package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/store"

	_ "modernc.org/sqlite"
)

func TestTask12LegacyFixtureMigrationAndEventCutoverParity(t *testing.T) {
	opened, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "task12-cutover.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer opened.Close()

	seed := seedTask12LegacyFixture(t, opened)

	if err := db.RunMigrations(context.Background(), opened); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	result, err := db.FinalizeV2Cutover(context.Background(), opened)
	if err != nil {
		t.Fatalf("FinalizeV2Cutover failed: %v", err)
	}
	if result.AlreadyFinalized {
		t.Fatalf("expected first finalization run to execute")
	}
	if result.DroppedCompatibilitySync == 0 {
		t.Fatalf("expected compatibility triggers to be dropped during cutover")
	}

	assertLegacyParity(t, opened)

	prev := db.DB
	db.DB = opened
	t.Cleanup(func() { db.DB = prev })

	var sessionsBefore int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM sessions`).Scan(&sessionsBefore); err != nil {
		t.Fatalf("count sessions before canonical writes failed: %v", err)
	}
	var plannedBefore int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM planned_events`).Scan(&plannedBefore); err != nil {
		t.Fatalf("count planned_events before canonical writes failed: %v", err)
	}

	trackedID, err := db.InsertSession("focus", "Task12", 25*time.Minute)
	if err != nil {
		t.Fatalf("InsertSession (canonical tracked write) failed: %v", err)
	}
	if err := db.StopCurrentSession(); err != nil {
		t.Fatalf("StopCurrentSession failed: %v", err)
	}

	var (
		trackedStatus       string
		trackedSource       string
		trackedLayer        string
		trackedLegacySource sql.NullString
		trackedDomain       string
		trackedSubtopic     string
	)
	if err := opened.QueryRow(`
		SELECT status, source, layer, legacy_source, domain, subtopic
		FROM events
		WHERE id = ?`, trackedID).Scan(&trackedStatus, &trackedSource, &trackedLayer, &trackedLegacySource, &trackedDomain, &trackedSubtopic); err != nil {
		t.Fatalf("query canonical tracked row failed: %v", err)
	}
	if trackedSource != "tracked" || trackedLayer != "done" {
		t.Fatalf("unexpected tracked row source/layer: source=%s layer=%s", trackedSource, trackedLayer)
	}
	if trackedStatus != "done" {
		t.Fatalf("expected stopped tracked row status=done, got %s", trackedStatus)
	}
	if trackedLegacySource.Valid {
		t.Fatalf("expected canonical tracked write to have no legacy_source, got %q", trackedLegacySource.String)
	}
	if trackedDomain != "Task12" || trackedSubtopic != "General" {
		t.Fatalf("unexpected canonical tracked topic mapping: %s::%s", trackedDomain, trackedSubtopic)
	}

	var sessionsAfter int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM sessions`).Scan(&sessionsAfter); err != nil {
		t.Fatalf("count sessions after canonical writes failed: %v", err)
	}
	if sessionsAfter != sessionsBefore {
		t.Fatalf("expected legacy sessions table to remain unchanged post-cutover: before=%d after=%d", sessionsBefore, sessionsAfter)
	}

	planStart := seed.anchor.Add(8 * time.Hour)
	planEnd := planStart.Add(90 * time.Minute)
	plannedID, err := store.CreatePlannedEvent(context.Background(), store.PlannedEvent{
		Title:       "Task12::Review",
		Description: "post-cutover create",
		StartTime:   planStart,
		EndTime:     planEnd,
	}, "task12-test")
	if err != nil {
		t.Fatalf("CreatePlannedEvent (canonical planned write) failed: %v", err)
	}

	var (
		plannedLayer        string
		plannedSource       string
		plannedLegacySource sql.NullString
	)
	if err := opened.QueryRow(`SELECT layer, source, legacy_source FROM events WHERE id = ?`, plannedID).Scan(&plannedLayer, &plannedSource, &plannedLegacySource); err != nil {
		t.Fatalf("query canonical planned row failed: %v", err)
	}
	if plannedLayer != "planned" || plannedSource != "manual" {
		t.Fatalf("unexpected canonical planned row layer/source: layer=%s source=%s", plannedLayer, plannedSource)
	}
	if plannedLegacySource.Valid {
		t.Fatalf("expected canonical planned write to have no legacy_source, got %q", plannedLegacySource.String)
	}

	var plannedAfter int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM planned_events`).Scan(&plannedAfter); err != nil {
		t.Fatalf("count planned_events after canonical writes failed: %v", err)
	}
	if plannedAfter != plannedBefore {
		t.Fatalf("expected legacy planned_events table to remain unchanged post-cutover: before=%d after=%d", plannedBefore, plannedAfter)
	}

	now := time.Now()
	sessions, err := store.SessionsInRange(context.Background(), now.Add(-2*time.Hour), now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("SessionsInRange failed: %v", err)
	}
	if !containsSessionID(sessions, int(trackedID)) {
		t.Fatalf("expected SessionsInRange to include canonical tracked id=%d", trackedID)
	}

	planned, err := store.PlannedEventsInRange(context.Background(), seed.anchor.Add(-24*time.Hour), seed.anchor.Add(48*time.Hour))
	if err != nil {
		t.Fatalf("PlannedEventsInRange failed: %v", err)
	}
	if !containsPlannedID(planned, int(plannedID)) {
		t.Fatalf("expected PlannedEventsInRange to include canonical planned id=%d", plannedID)
	}
}

type task12Seed struct {
	anchor time.Time
}

func seedTask12LegacyFixture(t *testing.T, opened *sql.DB) task12Seed {
	t.Helper()

	if _, err := opened.Exec(`
		CREATE TABLE schema_migrations (
			name TEXT PRIMARY KEY,
			checksum TEXT NOT NULL,
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

		CREATE TABLE planned_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT,
			start_time DATETIME NOT NULL,
			end_time DATETIME NOT NULL,
			status TEXT NOT NULL DEFAULT 'planned' CHECK(status IN ('planned','done','canceled')),
			source TEXT NOT NULL DEFAULT 'manual' CHECK(source IN ('manual','scheduler')),
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
	`); err != nil {
		t.Fatalf("seed legacy schema failed: %v", err)
	}

	appliedAt := time.Date(2026, 2, 27, 12, 0, 0, 0, time.UTC)
	for _, name := range []string{
		"001_base_sessions",
		"002_sessions_metadata",
		"003_planned_events",
		"004_audit_log",
		"005_sessions_indexes",
		"006_sessions_timestamps_backfill",
	} {
		if _, err := opened.Exec(`INSERT INTO schema_migrations(name, checksum, applied_at) VALUES (?, 'legacy', ?)`, name, appliedAt); err != nil {
			t.Fatalf("insert legacy migration row %s failed: %v", name, err)
		}
	}

	anchor := time.Date(2026, 2, 25, 9, 0, 0, 0, time.UTC)
	formatTS := func(v time.Time) string { return v.Format("2006-01-02 15:04:05") }
	if _, err := opened.Exec(`
		INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "Legacy::Math", formatTS(anchor), formatTS(anchor.Add(50*time.Minute)), int((50 * time.Minute).Seconds()), formatTS(anchor), formatTS(anchor.Add(50*time.Minute))); err != nil {
		t.Fatalf("insert legacy focus session failed: %v", err)
	}
	if _, err := opened.Exec(`
		INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"break", "", formatTS(anchor.Add(50*time.Minute)), formatTS(anchor.Add(60*time.Minute)), int((10 * time.Minute).Seconds()), formatTS(anchor.Add(50*time.Minute)), formatTS(anchor.Add(60*time.Minute))); err != nil {
		t.Fatalf("insert legacy break session failed: %v", err)
	}
	if _, err := opened.Exec(`
		INSERT INTO planned_events(title, description, start_time, end_time, status, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"Legacy Planning", "legacy fixture", formatTS(anchor.Add(4*time.Hour)), formatTS(anchor.Add(5*time.Hour)), "planned", "manual", formatTS(anchor.Add(4*time.Hour)), formatTS(anchor.Add(4*time.Hour))); err != nil {
		t.Fatalf("insert legacy planned event failed: %v", err)
	}

	return task12Seed{anchor: anchor}
}

func assertLegacyParity(t *testing.T, opened *sql.DB) {
	t.Helper()

	var sessionsCount, plannedCount int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM sessions`).Scan(&sessionsCount); err != nil {
		t.Fatalf("count sessions failed: %v", err)
	}
	if err := opened.QueryRow(`SELECT COUNT(1) FROM planned_events`).Scan(&plannedCount); err != nil {
		t.Fatalf("count planned_events failed: %v", err)
	}

	var mappedSessions, mappedPlanned int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM events WHERE legacy_source = 'sessions'`).Scan(&mappedSessions); err != nil {
		t.Fatalf("count mapped sessions failed: %v", err)
	}
	if err := opened.QueryRow(`SELECT COUNT(1) FROM events WHERE legacy_source = 'planned_events'`).Scan(&mappedPlanned); err != nil {
		t.Fatalf("count mapped planned_events failed: %v", err)
	}
	if mappedSessions != sessionsCount {
		t.Fatalf("legacy session parity mismatch: sessions=%d mapped=%d", sessionsCount, mappedSessions)
	}
	if mappedPlanned != plannedCount {
		t.Fatalf("legacy planned parity mismatch: planned=%d mapped=%d", plannedCount, mappedPlanned)
	}
}

func containsSessionID(rows []store.Session, want int) bool {
	for _, row := range rows {
		if row.ID == want {
			return true
		}
	}
	return false
}

func containsPlannedID(rows []store.PlannedEvent, want int) bool {
	for _, row := range rows {
		if row.ID == want {
			return true
		}
	}
	return false
}
