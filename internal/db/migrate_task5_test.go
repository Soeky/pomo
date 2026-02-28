package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestTask5SchedulerIndexesAndPlannedTopicColumns(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "task5-schema.db")
	opened, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer opened.Close()

	if !columnExists(t, opened, "planned_events", "domain") {
		t.Fatalf("expected planned_events.domain column to exist")
	}
	if !columnExists(t, opened, "planned_events", "subtopic") {
		t.Fatalf("expected planned_events.subtopic column to exist")
	}

	requiredIndexes := []string{
		"idx_planned_events_topic",
		"idx_events_workload_target_time",
		"idx_events_source_status_time",
		"idx_workload_targets_active_cadence_topic",
		"idx_schedule_constraints_updated_at",
		"idx_schedule_run_events_run_action",
	}
	for _, idx := range requiredIndexes {
		if !sqliteObjectExists(t, opened, "index", idx) {
			t.Fatalf("expected index %q to exist", idx)
		}
	}
}

func TestTask5PlannedTopicBackfillReconciliationAndIdempotency(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "task5-backfill.db")
	opened, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer opened.Close()

	seedLegacySchemaForTask2(t, opened)

	res, err := opened.Exec(`
		INSERT INTO planned_events(title, description, start_time, end_time, status, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"Math::Linear Algebra", "hierarchical legacy title", "2026-02-27 15:00:00", "2026-02-27 16:00:00", "planned", "manual", "2026-02-27 15:00:00", "2026-02-27 15:00:00",
	)
	if err != nil {
		t.Fatalf("insert hierarchical planned event failed: %v", err)
	}
	hierarchicalPlannedID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("hierarchical planned event id failed: %v", err)
	}

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

	var domain, subtopic string
	if err := opened.QueryRow(`
		SELECT domain, subtopic
		FROM planned_events
		WHERE id = ?`, hierarchicalPlannedID).Scan(&domain, &subtopic); err != nil {
		t.Fatalf("query migrated planned event topic fields failed: %v", err)
	}
	if domain != "Math" || subtopic != "Linear Algebra" {
		t.Fatalf("unexpected migrated planned topic: domain=%s subtopic=%s", domain, subtopic)
	}

	if err := opened.QueryRow(`
		SELECT domain, subtopic
		FROM events
		WHERE legacy_source = 'planned_events' AND legacy_id = ?`, hierarchicalPlannedID).Scan(&domain, &subtopic); err != nil {
		t.Fatalf("query mapped event topic fields failed: %v", err)
	}
	if domain != "Math" || subtopic != "Linear Algebra" {
		t.Fatalf("unexpected mapped event topic: domain=%s subtopic=%s", domain, subtopic)
	}

	if _, err := opened.Exec(`
		UPDATE planned_events
		SET title = ?, domain = ?, subtopic = ?, updated_at = ?
		WHERE id = ?`,
		"Physics::Quantum", "stale-domain", "stale-subtopic", "2026-02-27 17:00:00", hierarchicalPlannedID,
	); err != nil {
		t.Fatalf("drift planned event topic fields failed: %v", err)
	}
	if _, err := opened.Exec(`
		UPDATE events
		SET domain = ?, subtopic = ?, recurrence_rule_id = 9, workload_target_id = 9, metadata_json = '{"drift":true}'
		WHERE legacy_source = 'planned_events' AND legacy_id = ?`,
		"wrong-domain", "wrong-subtopic", hierarchicalPlannedID,
	); err != nil {
		t.Fatalf("drift mapped event fields failed: %v", err)
	}

	if _, err := opened.Exec(`DELETE FROM schema_migrations WHERE name = '012_task5_scheduler_topic_backfill'`); err != nil {
		t.Fatalf("delete 012 migration row failed: %v", err)
	}
	if err := RunMigrations(context.Background(), opened); err != nil {
		t.Fatalf("reapply RunMigrations failed: %v", err)
	}

	assertLegacyParity("post-reapply")

	if err := opened.QueryRow(`
		SELECT domain, subtopic
		FROM planned_events
		WHERE id = ?`, hierarchicalPlannedID).Scan(&domain, &subtopic); err != nil {
		t.Fatalf("query planned topics after reapply failed: %v", err)
	}
	if domain != "Physics" || subtopic != "Quantum" {
		t.Fatalf("unexpected planned topic after reapply: domain=%s subtopic=%s", domain, subtopic)
	}

	var recurrenceRuleID, workloadTargetID sql.NullInt64
	var metadataJSON sql.NullString
	if err := opened.QueryRow(`
		SELECT domain, subtopic, recurrence_rule_id, workload_target_id, metadata_json
		FROM events
		WHERE legacy_source = 'planned_events' AND legacy_id = ?`, hierarchicalPlannedID).
		Scan(&domain, &subtopic, &recurrenceRuleID, &workloadTargetID, &metadataJSON); err != nil {
		t.Fatalf("query mapped event after reapply failed: %v", err)
	}
	if domain != "Physics" || subtopic != "Quantum" {
		t.Fatalf("unexpected mapped event topic after reapply: domain=%s subtopic=%s", domain, subtopic)
	}
	if recurrenceRuleID.Valid || workloadTargetID.Valid || metadataJSON.Valid {
		t.Fatalf("expected mapped legacy scheduler linkage fields to be cleared after reapply")
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
