package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestTask6DependencyBlockingSchemaAndIndexes(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "task6-schema.db")
	opened, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer opened.Close()

	if !columnExists(t, opened, "events", "blocked_reason") {
		t.Fatalf("expected events.blocked_reason column to exist")
	}
	if !columnExists(t, opened, "events", "blocked_override") {
		t.Fatalf("expected events.blocked_override column to exist")
	}

	requiredIndexes := []string{
		"idx_event_dependencies_required",
		"idx_event_dependencies_dep_required",
		"idx_events_status_override_time",
	}
	for _, idx := range requiredIndexes {
		if !sqliteObjectExists(t, opened, "index", idx) {
			t.Fatalf("expected index %q to exist", idx)
		}
	}
}

func TestTask6MigrationReapplyPreservesParityAndNormalization(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "task6-reapply.db")
	opened, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer opened.Close()

	seed := seedLegacySchemaForTask2(t, opened)
	if err := RunMigrations(context.Background(), opened); err != nil {
		t.Fatalf("initial RunMigrations failed: %v", err)
	}

	assertLegacyParity := func(prefix string) {
		t.Helper()
		var sessionsCount, plannedCount, sessionEvents, plannedEvents int
		if err := opened.QueryRow(`SELECT COUNT(1) FROM sessions`).Scan(&sessionsCount); err != nil {
			t.Fatalf("%s: count sessions failed: %v", prefix, err)
		}
		if err := opened.QueryRow(`SELECT COUNT(1) FROM planned_events`).Scan(&plannedCount); err != nil {
			t.Fatalf("%s: count planned_events failed: %v", prefix, err)
		}
		if err := opened.QueryRow(`SELECT COUNT(1) FROM events WHERE legacy_source = 'sessions'`).Scan(&sessionEvents); err != nil {
			t.Fatalf("%s: count session-backed events failed: %v", prefix, err)
		}
		if err := opened.QueryRow(`SELECT COUNT(1) FROM events WHERE legacy_source = 'planned_events'`).Scan(&plannedEvents); err != nil {
			t.Fatalf("%s: count planned-backed events failed: %v", prefix, err)
		}
		if sessionsCount != sessionEvents {
			t.Fatalf("%s: session parity mismatch: sessions=%d events=%d", prefix, sessionsCount, sessionEvents)
		}
		if plannedCount != plannedEvents {
			t.Fatalf("%s: planned parity mismatch: planned=%d events=%d", prefix, plannedCount, plannedEvents)
		}
	}

	assertLegacyParity("post-initial")

	var beforeCount int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM events`).Scan(&beforeCount); err != nil {
		t.Fatalf("count events before reapply failed: %v", err)
	}

	if _, err := opened.Exec(`
		UPDATE events
		SET status = 'planned',
		    blocked_reason = 'stale-blocking-reason',
		    blocked_override = 1
		WHERE legacy_source = 'planned_events'
		  AND legacy_id = ?`, seed.manualPlannedID); err != nil {
		t.Fatalf("drift blocked fields failed: %v", err)
	}

	if _, err := opened.Exec(`DELETE FROM schema_migrations WHERE name = '013_task6_dependency_blocking_hardening'`); err != nil {
		t.Fatalf("delete 013 migration row failed: %v", err)
	}
	if err := RunMigrations(context.Background(), opened); err != nil {
		t.Fatalf("reapply RunMigrations failed: %v", err)
	}

	assertLegacyParity("post-reapply")

	var afterCount int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM events`).Scan(&afterCount); err != nil {
		t.Fatalf("count events after reapply failed: %v", err)
	}
	if beforeCount != afterCount {
		t.Fatalf("expected idempotent event row count: before=%d after=%d", beforeCount, afterCount)
	}

	var blockedReason sql.NullString
	if err := opened.QueryRow(`
		SELECT blocked_reason
		FROM events
		WHERE legacy_source = 'planned_events'
		  AND legacy_id = ?`, seed.manualPlannedID).Scan(&blockedReason); err != nil {
		t.Fatalf("query blocked_reason after reapply failed: %v", err)
	}
	if blockedReason.Valid {
		t.Fatalf("expected non-blocked row to have cleared blocked_reason after reapply, got %q", blockedReason.String)
	}

	var duplicateMappings int
	if err := opened.QueryRow(`
		SELECT COUNT(1)
		FROM (
			SELECT legacy_source, legacy_id, COUNT(1) AS c
			FROM events
			WHERE legacy_source IN ('sessions', 'planned_events')
			  AND legacy_id IS NOT NULL
			GROUP BY legacy_source, legacy_id
			HAVING c > 1
		)`).Scan(&duplicateMappings); err != nil {
		t.Fatalf("duplicate mapping query failed: %v", err)
	}
	if duplicateMappings != 0 {
		t.Fatalf("expected no duplicate legacy mappings, found %d", duplicateMappings)
	}
}
