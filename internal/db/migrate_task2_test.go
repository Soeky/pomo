package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

type task2LegacySeed struct {
	focusSessionID     int64
	breakSessionID     int64
	runningSessionID   int64
	manualPlannedID    int64
	schedulerPlannedID int64
	canceledPlannedID  int64
	rangeFrom          string
	rangeTo            string
}

func TestTask2BackfillReconciliationAndParity(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "task2-backfill.db")
	opened, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer opened.Close()

	seed := seedLegacySchemaForTask2(t, opened)

	if err := RunMigrations(context.Background(), opened); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	var sessionsCount, plannedCount int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM sessions`).Scan(&sessionsCount); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if err := opened.QueryRow(`SELECT COUNT(1) FROM planned_events`).Scan(&plannedCount); err != nil {
		t.Fatalf("count planned_events: %v", err)
	}

	var sessionEvents, plannedEvents int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM events WHERE legacy_source = 'sessions'`).Scan(&sessionEvents); err != nil {
		t.Fatalf("count session events: %v", err)
	}
	if err := opened.QueryRow(`SELECT COUNT(1) FROM events WHERE legacy_source = 'planned_events'`).Scan(&plannedEvents); err != nil {
		t.Fatalf("count planned events: %v", err)
	}
	if sessionsCount != sessionEvents {
		t.Fatalf("session reconciliation mismatch: sessions=%d events=%d", sessionsCount, sessionEvents)
	}
	if plannedCount != plannedEvents {
		t.Fatalf("planned reconciliation mismatch: planned_events=%d events=%d", plannedCount, plannedEvents)
	}

	var legacyInRange int
	if err := opened.QueryRow(`
		SELECT
			(SELECT COUNT(1)
			 FROM sessions
			 WHERE start_time < ?
			   AND COALESCE(end_time, datetime(start_time, '+' || COALESCE(duration, 0) || ' seconds')) > ?)
			+
			(SELECT COUNT(1)
			 FROM planned_events
			 WHERE start_time < ?
			   AND end_time > ?)`,
		seed.rangeTo, seed.rangeFrom, seed.rangeTo, seed.rangeFrom,
	).Scan(&legacyInRange); err != nil {
		t.Fatalf("legacy range count query failed: %v", err)
	}

	var eventInRange int
	if err := opened.QueryRow(`
		SELECT COUNT(1)
		FROM events
		WHERE start_time < ?
		  AND end_time > ?
		  AND legacy_source IN ('sessions', 'planned_events')`,
		seed.rangeTo, seed.rangeFrom,
	).Scan(&eventInRange); err != nil {
		t.Fatalf("events range count query failed: %v", err)
	}
	if legacyInRange != eventInRange {
		t.Fatalf("range parity mismatch: legacy=%d events=%d", legacyInRange, eventInRange)
	}

	var domain, subtopic, title, status, source string
	if err := opened.QueryRow(`
		SELECT domain, subtopic, title, status, source
		FROM events
		WHERE legacy_source = 'sessions' AND legacy_id = ?`,
		seed.focusSessionID,
	).Scan(&domain, &subtopic, &title, &status, &source); err != nil {
		t.Fatalf("query focus session event failed: %v", err)
	}
	if domain != "Math::Discrete Probability" || subtopic != "General" {
		t.Fatalf("unexpected focus topic mapping: domain=%s subtopic=%s", domain, subtopic)
	}
	if title != "Math::Discrete Probability" {
		t.Fatalf("unexpected focus title mapping: %s", title)
	}
	if status != "done" || source != "tracked" {
		t.Fatalf("unexpected focus status/source mapping: status=%s source=%s", status, source)
	}

	if err := opened.QueryRow(`
		SELECT status
		FROM events
		WHERE legacy_source = 'sessions' AND legacy_id = ?`,
		seed.runningSessionID,
	).Scan(&status); err != nil {
		t.Fatalf("query running session event failed: %v", err)
	}
	if status != "in_progress" {
		t.Fatalf("expected running session to map to in_progress, got %s", status)
	}

	if err := opened.QueryRow(`
		SELECT domain, subtopic, source
		FROM events
		WHERE legacy_source = 'planned_events' AND legacy_id = ?`,
		seed.manualPlannedID,
	).Scan(&domain, &subtopic, &source); err != nil {
		t.Fatalf("query manual planned event failed: %v", err)
	}
	if domain != "Manual Planning Block" || subtopic != "General" {
		t.Fatalf("unexpected planned topic mapping: domain=%s subtopic=%s", domain, subtopic)
	}
	if source != "manual" {
		t.Fatalf("expected planned source manual, got %s", source)
	}
}

func TestTask2MigrationReapplyIsIdempotent(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "task2-idempotent.db")
	opened, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer opened.Close()

	seedLegacySchemaForTask2(t, opened)
	if err := RunMigrations(context.Background(), opened); err != nil {
		t.Fatalf("initial RunMigrations failed: %v", err)
	}

	var before int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM events`).Scan(&before); err != nil {
		t.Fatalf("count events before reapply: %v", err)
	}

	if _, err := opened.Exec(`DELETE FROM schema_migrations WHERE name = '008_unified_events_backfill_and_sync'`); err != nil {
		t.Fatalf("delete 008 migration row: %v", err)
	}
	if err := RunMigrations(context.Background(), opened); err != nil {
		t.Fatalf("reapply RunMigrations failed: %v", err)
	}

	var after int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM events`).Scan(&after); err != nil {
		t.Fatalf("count events after reapply: %v", err)
	}
	if before != after {
		t.Fatalf("expected idempotent reapply without duplicate rows: before=%d after=%d", before, after)
	}
}

func TestTask2IndexesAndCompatibilityTriggers(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "task2-indexes.db")
	opened, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer opened.Close()

	requiredIndexes := []string{
		"idx_events_legacy_unique",
		"idx_events_source_time",
		"idx_events_status_time",
		"idx_events_kind_time",
		"idx_recurrence_window",
		"idx_workload_targets_active",
		"idx_schedule_runs_status_started",
		"idx_schedule_run_events_event_action",
	}
	for _, idx := range requiredIndexes {
		if !sqliteObjectExists(t, opened, "index", idx) {
			t.Fatalf("expected index %q to exist", idx)
		}
	}

	requiredTriggers := []string{
		"trg_sessions_to_events_insert",
		"trg_sessions_to_events_update",
		"trg_sessions_to_events_delete",
		"trg_planned_events_to_events_insert",
		"trg_planned_events_to_events_update",
		"trg_planned_events_to_events_delete",
	}
	for _, trig := range requiredTriggers {
		if !sqliteObjectExists(t, opened, "trigger", trig) {
			t.Fatalf("expected trigger %q to exist", trig)
		}
	}

	if !columnExists(t, opened, "events", "timezone") {
		t.Fatalf("expected events.timezone column to exist")
	}
}

func TestTask2LegacyMutationTriggersKeepEventsSynced(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "task2-triggers.db")
	opened, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer opened.Close()

	start := "2026-02-27 09:00:00"
	if _, err := opened.Exec(`
		INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "Math", start, nil, 1800, start, start,
	); err != nil {
		t.Fatalf("insert session failed: %v", err)
	}

	var sessionID int64
	if err := opened.QueryRow(`SELECT id FROM sessions ORDER BY id DESC LIMIT 1`).Scan(&sessionID); err != nil {
		t.Fatalf("read inserted session id failed: %v", err)
	}

	var status string
	if err := opened.QueryRow(`
		SELECT status
		FROM events
		WHERE legacy_source = 'sessions' AND legacy_id = ?`, sessionID).Scan(&status); err != nil {
		t.Fatalf("query session-backed event failed: %v", err)
	}
	if status != "in_progress" {
		t.Fatalf("expected in_progress status for running session, got %s", status)
	}

	if _, err := opened.Exec(`
		UPDATE sessions
		SET topic = ?, end_time = ?, duration = ?, updated_at = ?
		WHERE id = ?`,
		"Math::Linear Algebra", "2026-02-27 09:45:00", 2700, "2026-02-27 09:45:00", sessionID,
	); err != nil {
		t.Fatalf("update session failed: %v", err)
	}

	var domain, subtopic string
	if err := opened.QueryRow(`
		SELECT domain, subtopic, status
		FROM events
		WHERE legacy_source = 'sessions' AND legacy_id = ?`, sessionID).Scan(&domain, &subtopic, &status); err != nil {
		t.Fatalf("query updated session-backed event failed: %v", err)
	}
	if domain != "Math::Linear Algebra" || subtopic != "General" {
		t.Fatalf("unexpected updated session topic mapping: domain=%s subtopic=%s", domain, subtopic)
	}
	if status != "done" {
		t.Fatalf("expected done status after ending session, got %s", status)
	}

	if _, err := opened.Exec(`DELETE FROM sessions WHERE id = ?`, sessionID); err != nil {
		t.Fatalf("delete session failed: %v", err)
	}

	var sessionEvents int
	if err := opened.QueryRow(`
		SELECT COUNT(1)
		FROM events
		WHERE legacy_source = 'sessions' AND legacy_id = ?`, sessionID).Scan(&sessionEvents); err != nil {
		t.Fatalf("count session-backed events after delete failed: %v", err)
	}
	if sessionEvents != 0 {
		t.Fatalf("expected session-backed event to be deleted, remaining=%d", sessionEvents)
	}

	if _, err := opened.Exec(`
		INSERT INTO planned_events(title, description, start_time, end_time, status, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"Planner Item", "initial", "2026-02-27 11:00:00", "2026-02-27 12:00:00", "planned", "manual", "2026-02-27 11:00:00", "2026-02-27 11:00:00",
	); err != nil {
		t.Fatalf("insert planned event failed: %v", err)
	}

	var plannedID int64
	if err := opened.QueryRow(`SELECT id FROM planned_events ORDER BY id DESC LIMIT 1`).Scan(&plannedID); err != nil {
		t.Fatalf("read inserted planned event id failed: %v", err)
	}

	if err := opened.QueryRow(`
		SELECT source, status
		FROM events
		WHERE legacy_source = 'planned_events' AND legacy_id = ?`, plannedID).Scan(&domain, &status); err != nil {
		t.Fatalf("query planned-backed event failed: %v", err)
	}
	if domain != "manual" || status != "planned" {
		t.Fatalf("unexpected planned-backed event mapping: source=%s status=%s", domain, status)
	}

	if _, err := opened.Exec(`
		UPDATE planned_events
		SET status = ?, source = ?, updated_at = ?
		WHERE id = ?`,
		"done", "scheduler", "2026-02-27 12:00:00", plannedID,
	); err != nil {
		t.Fatalf("update planned event failed: %v", err)
	}

	if err := opened.QueryRow(`
		SELECT source, status
		FROM events
		WHERE legacy_source = 'planned_events' AND legacy_id = ?`, plannedID).Scan(&domain, &status); err != nil {
		t.Fatalf("query updated planned-backed event failed: %v", err)
	}
	if domain != "scheduler" || status != "done" {
		t.Fatalf("unexpected updated planned-backed mapping: source=%s status=%s", domain, status)
	}

	if _, err := opened.Exec(`DELETE FROM planned_events WHERE id = ?`, plannedID); err != nil {
		t.Fatalf("delete planned event failed: %v", err)
	}

	var plannedEvents int
	if err := opened.QueryRow(`
		SELECT COUNT(1)
		FROM events
		WHERE legacy_source = 'planned_events' AND legacy_id = ?`, plannedID).Scan(&plannedEvents); err != nil {
		t.Fatalf("count planned-backed events after delete failed: %v", err)
	}
	if plannedEvents != 0 {
		t.Fatalf("expected planned-backed event to be deleted, remaining=%d", plannedEvents)
	}
}

func seedLegacySchemaForTask2(t *testing.T, opened *sql.DB) task2LegacySeed {
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

	now := time.Date(2026, 2, 27, 12, 0, 0, 0, time.UTC)
	legacyMigrations := []string{
		"001_base_sessions",
		"002_sessions_metadata",
		"003_planned_events",
		"004_audit_log",
		"005_sessions_indexes",
		"006_sessions_timestamps_backfill",
	}
	for _, name := range legacyMigrations {
		if _, err := opened.Exec(`INSERT INTO schema_migrations(name, checksum, applied_at) VALUES (?, 'legacy', ?)`, name, now); err != nil {
			t.Fatalf("insert legacy migration row %s failed: %v", name, err)
		}
	}

	base := time.Date(2026, 2, 25, 9, 0, 0, 0, time.UTC)
	focusEnd := base.Add(50 * time.Minute)
	breakStart := focusEnd
	breakEnd := breakStart.Add(10 * time.Minute)
	runningStart := base.Add(2 * time.Hour)
	formatTS := func(v time.Time) string { return v.Format("2006-01-02 15:04:05") }

	res, err := opened.Exec(`
		INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "Math::Discrete Probability", formatTS(base), formatTS(focusEnd), int((50 * time.Minute).Seconds()), formatTS(base), formatTS(focusEnd),
	)
	if err != nil {
		t.Fatalf("insert focus session failed: %v", err)
	}
	focusID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("focus session last insert id failed: %v", err)
	}

	res, err = opened.Exec(`
		INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"break", "", formatTS(breakStart), formatTS(breakEnd), int((10 * time.Minute).Seconds()), formatTS(breakStart), formatTS(breakEnd),
	)
	if err != nil {
		t.Fatalf("insert break session failed: %v", err)
	}
	breakID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("break session last insert id failed: %v", err)
	}

	res, err = opened.Exec(`
		INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "Physics", formatTS(runningStart), nil, int((30 * time.Minute).Seconds()), formatTS(runningStart), formatTS(runningStart),
	)
	if err != nil {
		t.Fatalf("insert running session failed: %v", err)
	}
	runningID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("running session last insert id failed: %v", err)
	}

	manualStart := base.Add(4 * time.Hour)
	manualEnd := manualStart.Add(90 * time.Minute)
	res, err = opened.Exec(`
		INSERT INTO planned_events(title, description, start_time, end_time, status, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"Manual Planning Block", "read chapter 3", formatTS(manualStart), formatTS(manualEnd), "planned", "manual", formatTS(manualStart), formatTS(manualStart),
	)
	if err != nil {
		t.Fatalf("insert manual planned event failed: %v", err)
	}
	manualID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("manual planned event last insert id failed: %v", err)
	}

	schedulerStart := manualEnd.Add(30 * time.Minute)
	schedulerEnd := schedulerStart.Add(45 * time.Minute)
	res, err = opened.Exec(`
		INSERT INTO planned_events(title, description, start_time, end_time, status, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"Scheduler Block", "generated", formatTS(schedulerStart), formatTS(schedulerEnd), "done", "scheduler", formatTS(schedulerStart), formatTS(schedulerEnd),
	)
	if err != nil {
		t.Fatalf("insert scheduler planned event failed: %v", err)
	}
	schedulerID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("scheduler planned event last insert id failed: %v", err)
	}

	canceledStart := schedulerEnd.Add(30 * time.Minute)
	canceledEnd := canceledStart.Add(20 * time.Minute)
	res, err = opened.Exec(`
		INSERT INTO planned_events(title, description, start_time, end_time, status, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"Canceled Block", "skip this", formatTS(canceledStart), formatTS(canceledEnd), "canceled", "manual", formatTS(canceledStart), formatTS(canceledEnd),
	)
	if err != nil {
		t.Fatalf("insert canceled planned event failed: %v", err)
	}
	canceledID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("canceled planned event last insert id failed: %v", err)
	}

	return task2LegacySeed{
		focusSessionID:     focusID,
		breakSessionID:     breakID,
		runningSessionID:   runningID,
		manualPlannedID:    manualID,
		schedulerPlannedID: schedulerID,
		canceledPlannedID:  canceledID,
		rangeFrom:          formatTS(base.Add(-time.Hour)),
		rangeTo:            formatTS(canceledEnd.Add(time.Hour)),
	}
}

func sqliteObjectExists(t *testing.T, opened *sql.DB, objType, name string) bool {
	t.Helper()

	var count int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM sqlite_master WHERE type = ? AND name = ?`, objType, name).Scan(&count); err != nil {
		t.Fatalf("sqlite object exists query failed: %v", err)
	}
	return count == 1
}

func columnExists(t *testing.T, opened *sql.DB, table, column string) bool {
	t.Helper()

	rows, err := opened.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info failed: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			t.Fatalf("scan table_info row failed: %v", err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table_info rows failed: %v", err)
	}
	return false
}
