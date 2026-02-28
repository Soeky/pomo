package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestTask4UnifiedEventsSchedulerSchemaAndIndexes(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "task4-schema.db")
	opened, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer opened.Close()

	requiredTables := []string{
		"events",
		"event_dependencies",
		"recurrence_rules",
		"workload_targets",
		"schedule_constraints",
		"schedule_runs",
		"schedule_run_events",
	}
	for _, table := range requiredTables {
		if !tableExists(t, opened, table) {
			t.Fatalf("expected table %q to exist", table)
		}
	}

	requiredIndexes := []string{
		"idx_events_time",
		"idx_events_layer_status",
		"idx_events_topic",
		"idx_events_legacy",
		"idx_events_legacy_unique",
		"idx_events_source_time",
		"idx_events_status_time",
		"idx_events_kind_time",
		"idx_events_recurrence_occurrence_unique",
		"idx_events_recurrence_rule_time",
		"idx_event_dependencies_event",
		"idx_event_dependencies_depends",
		"idx_recurrence_active",
		"idx_recurrence_window",
		"idx_workload_targets_topic",
		"idx_workload_targets_active",
		"idx_schedule_runs_status_started",
		"idx_schedule_run_events_run",
		"idx_schedule_run_events_event",
		"idx_schedule_run_events_event_action",
	}
	for _, idx := range requiredIndexes {
		if !sqliteObjectExists(t, opened, "index", idx) {
			t.Fatalf("expected index %q to exist", idx)
		}
	}
}

func TestTask4BackfillParityIdempotencyAndLegacyAdapterHardening(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "task4-backfill.db")
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

	assertNoDuplicateLegacyMappings := func(prefix string) {
		t.Helper()
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
			t.Fatalf("%s: duplicate mapping query failed: %v", prefix, err)
		}
		if duplicateMappings != 0 {
			t.Fatalf("%s: expected no duplicate legacy mappings, found %d", prefix, duplicateMappings)
		}
	}

	assertLegacyParity("post-initial")
	assertNoDuplicateLegacyMappings("post-initial")

	if _, err := opened.Exec(`
		UPDATE events
		SET recurrence_rule_id = 44, workload_target_id = 55, metadata_json = '{"drift":true}'
		WHERE legacy_source = 'sessions' AND legacy_id = ?`, seed.focusSessionID); err != nil {
		t.Fatalf("drift session-backed event failed: %v", err)
	}
	if _, err := opened.Exec(`
		UPDATE events
		SET recurrence_rule_id = 66, workload_target_id = 77, metadata_json = '{"drift":true}'
		WHERE legacy_source = 'planned_events' AND legacy_id = ?`, seed.manualPlannedID); err != nil {
		t.Fatalf("drift planned-backed event failed: %v", err)
	}

	if _, err := opened.Exec(`
		UPDATE sessions
		SET topic = ?, updated_at = ?
		WHERE id = ?`,
		"Math::Combinatorics", "2026-02-27 13:00:00", seed.focusSessionID,
	); err != nil {
		t.Fatalf("update legacy session failed: %v", err)
	}
	if _, err := opened.Exec(`
		UPDATE planned_events
		SET description = ?, updated_at = ?
		WHERE id = ?`,
		"reconciled by trigger", "2026-02-27 13:00:00", seed.manualPlannedID,
	); err != nil {
		t.Fatalf("update legacy planned event failed: %v", err)
	}

	var recurrenceRuleID, workloadTargetID sql.NullInt64
	var metadataJSON sql.NullString
	if err := opened.QueryRow(`
		SELECT recurrence_rule_id, workload_target_id, metadata_json
		FROM events
		WHERE legacy_source = 'sessions' AND legacy_id = ?`, seed.focusSessionID).Scan(&recurrenceRuleID, &workloadTargetID, &metadataJSON); err != nil {
		t.Fatalf("query session-backed linkage fields failed: %v", err)
	}
	if recurrenceRuleID.Valid || workloadTargetID.Valid || metadataJSON.Valid {
		t.Fatalf("expected session-backed legacy linkage fields to be cleared by adapter trigger")
	}

	if err := opened.QueryRow(`
		SELECT recurrence_rule_id, workload_target_id, metadata_json
		FROM events
		WHERE legacy_source = 'planned_events' AND legacy_id = ?`, seed.manualPlannedID).Scan(&recurrenceRuleID, &workloadTargetID, &metadataJSON); err != nil {
		t.Fatalf("query planned-backed linkage fields failed: %v", err)
	}
	if recurrenceRuleID.Valid || workloadTargetID.Valid || metadataJSON.Valid {
		t.Fatalf("expected planned-backed legacy linkage fields to be cleared by adapter trigger")
	}

	if _, err := opened.Exec(`DELETE FROM schema_migrations WHERE name IN ('008_unified_events_backfill_and_sync', '009_unified_events_reconcile_legacy_rows', '010_unified_events_legacy_trigger_hardening', '011_recurring_events_occurrence_indexes')`); err != nil {
		t.Fatalf("delete migration rows failed: %v", err)
	}
	if err := RunMigrations(context.Background(), opened); err != nil {
		t.Fatalf("reapply RunMigrations failed: %v", err)
	}

	assertLegacyParity("post-reapply")
	assertNoDuplicateLegacyMappings("post-reapply")
}
