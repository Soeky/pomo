package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestTask3ReconciliationRepairsDriftedLegacyMappings(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "task3-reconcile.db")
	opened, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer opened.Close()

	seed := seedLegacySchemaForTask2(t, opened)
	if err := RunMigrations(context.Background(), opened); err != nil {
		t.Fatalf("initial RunMigrations failed: %v", err)
	}

	if _, err := opened.Exec(`
		UPDATE events
		SET title = ?, domain = ?, subtopic = ?, description = ?, status = ?, source = ?, timezone = ?,
		    recurrence_rule_id = 7, workload_target_id = 9, metadata_json = '{"drift":true}'
		WHERE legacy_source = 'sessions' AND legacy_id = ?`,
		"drifted-session", "WrongDomain", "WrongSubtopic", "bad", "planned", "manual", "UTC", seed.focusSessionID,
	); err != nil {
		t.Fatalf("drift session-backed event failed: %v", err)
	}

	if _, err := opened.Exec(`
		UPDATE events
		SET title = ?, domain = ?, subtopic = ?, description = ?, status = ?, source = ?, timezone = ?,
		    recurrence_rule_id = 3, workload_target_id = 4, metadata_json = '{"drift":true}'
		WHERE legacy_source = 'planned_events' AND legacy_id = ?`,
		"drifted-planned", "WrongDomain", "WrongSubtopic", "bad", "blocked", "tracked", "UTC", seed.manualPlannedID,
	); err != nil {
		t.Fatalf("drift planned-backed event failed: %v", err)
	}

	if _, err := opened.Exec(`
		INSERT INTO events (
			kind, title, domain, subtopic, description,
			start_time, end_time, duration, timezone,
			layer, status, source,
			legacy_source, legacy_id,
			created_at, updated_at
		)
		VALUES (
			'task', 'orphan', 'orphan', 'General', 'orphan',
			'2026-02-01 10:00:00', '2026-02-01 11:00:00', 3600, 'Local',
			'planned', 'planned', 'manual',
			'sessions', 999999,
			'2026-02-01 10:00:00', '2026-02-01 10:00:00'
		)`); err != nil {
		t.Fatalf("insert orphan event failed: %v", err)
	}

	if _, err := opened.Exec(`DELETE FROM schema_migrations WHERE name = '009_unified_events_reconcile_legacy_rows'`); err != nil {
		t.Fatalf("delete 009 migration row failed: %v", err)
	}
	if err := RunMigrations(context.Background(), opened); err != nil {
		t.Fatalf("reapply RunMigrations failed: %v", err)
	}

	var title, domain, subtopic, status, source, timezone string
	var recurrenceRuleID, workloadTargetID sql.NullInt64
	var metadataJSON sql.NullString
	if err := opened.QueryRow(`
		SELECT title, domain, subtopic, status, source, timezone, recurrence_rule_id, workload_target_id, metadata_json
		FROM events
		WHERE legacy_source = 'sessions' AND legacy_id = ?`,
		seed.focusSessionID,
	).Scan(&title, &domain, &subtopic, &status, &source, &timezone, &recurrenceRuleID, &workloadTargetID, &metadataJSON); err != nil {
		t.Fatalf("query reconciled session-backed event failed: %v", err)
	}
	if title != "Math::Discrete Probability" || domain != "Math::Discrete Probability" || subtopic != "General" {
		t.Fatalf("unexpected reconciled session topic mapping: title=%s domain=%s subtopic=%s", title, domain, subtopic)
	}
	if status != "done" || source != "tracked" || timezone != "Local" {
		t.Fatalf("unexpected reconciled session status/source/timezone: status=%s source=%s timezone=%s", status, source, timezone)
	}
	if recurrenceRuleID.Valid || workloadTargetID.Valid || metadataJSON.Valid {
		t.Fatalf("expected reconciled session-backed event to clear scheduler linkage fields")
	}

	if err := opened.QueryRow(`
		SELECT title, domain, subtopic, status, source, timezone, recurrence_rule_id, workload_target_id, metadata_json
		FROM events
		WHERE legacy_source = 'planned_events' AND legacy_id = ?`,
		seed.manualPlannedID,
	).Scan(&title, &domain, &subtopic, &status, &source, &timezone, &recurrenceRuleID, &workloadTargetID, &metadataJSON); err != nil {
		t.Fatalf("query reconciled planned-backed event failed: %v", err)
	}
	if title != "Manual Planning Block" || domain != "Manual Planning Block" || subtopic != "General" {
		t.Fatalf("unexpected reconciled planned mapping: title=%s domain=%s subtopic=%s", title, domain, subtopic)
	}
	if status != "planned" || source != "manual" || timezone != "Local" {
		t.Fatalf("unexpected reconciled planned status/source/timezone: status=%s source=%s timezone=%s", status, source, timezone)
	}
	if recurrenceRuleID.Valid || workloadTargetID.Valid || metadataJSON.Valid {
		t.Fatalf("expected reconciled planned-backed event to clear scheduler linkage fields")
	}

	var orphanCount int
	if err := opened.QueryRow(`
		SELECT COUNT(1)
		FROM events
		WHERE legacy_source = 'sessions' AND legacy_id = 999999`).Scan(&orphanCount); err != nil {
		t.Fatalf("count orphan row failed: %v", err)
	}
	if orphanCount != 0 {
		t.Fatalf("expected orphan legacy mapping cleanup, still found %d rows", orphanCount)
	}

	var sessionsCount, plannedCount, sessionEvents, plannedEvents int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM sessions`).Scan(&sessionsCount); err != nil {
		t.Fatalf("count sessions failed: %v", err)
	}
	if err := opened.QueryRow(`SELECT COUNT(1) FROM planned_events`).Scan(&plannedCount); err != nil {
		t.Fatalf("count planned_events failed: %v", err)
	}
	if err := opened.QueryRow(`SELECT COUNT(1) FROM events WHERE legacy_source = 'sessions'`).Scan(&sessionEvents); err != nil {
		t.Fatalf("count session-backed events failed: %v", err)
	}
	if err := opened.QueryRow(`SELECT COUNT(1) FROM events WHERE legacy_source = 'planned_events'`).Scan(&plannedEvents); err != nil {
		t.Fatalf("count planned-backed events failed: %v", err)
	}
	if sessionsCount != sessionEvents {
		t.Fatalf("session parity mismatch after reconciliation: sessions=%d events=%d", sessionsCount, sessionEvents)
	}
	if plannedCount != plannedEvents {
		t.Fatalf("planned parity mismatch after reconciliation: planned=%d events=%d", plannedCount, plannedEvents)
	}
}

func TestTask3ReconciliationMigrationReapplyIsIdempotent(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "task3-idempotent.db")
	opened, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer opened.Close()

	seedLegacySchemaForTask2(t, opened)
	if err := RunMigrations(context.Background(), opened); err != nil {
		t.Fatalf("initial RunMigrations failed: %v", err)
	}

	var beforeCount, beforeChecksum int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM events`).Scan(&beforeCount); err != nil {
		t.Fatalf("count events before reapply failed: %v", err)
	}
	if err := opened.QueryRow(`
		SELECT COALESCE(SUM(duration), 0)
		FROM events
		WHERE legacy_source IN ('sessions', 'planned_events')`).Scan(&beforeChecksum); err != nil {
		t.Fatalf("legacy duration checksum before reapply failed: %v", err)
	}

	if _, err := opened.Exec(`DELETE FROM schema_migrations WHERE name = '009_unified_events_reconcile_legacy_rows'`); err != nil {
		t.Fatalf("delete 009 migration row failed: %v", err)
	}
	if err := RunMigrations(context.Background(), opened); err != nil {
		t.Fatalf("reapply RunMigrations failed: %v", err)
	}

	var afterCount, afterChecksum int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM events`).Scan(&afterCount); err != nil {
		t.Fatalf("count events after reapply failed: %v", err)
	}
	if err := opened.QueryRow(`
		SELECT COALESCE(SUM(duration), 0)
		FROM events
		WHERE legacy_source IN ('sessions', 'planned_events')`).Scan(&afterChecksum); err != nil {
		t.Fatalf("legacy duration checksum after reapply failed: %v", err)
	}
	if beforeCount != afterCount {
		t.Fatalf("expected idempotent reapply row count, before=%d after=%d", beforeCount, afterCount)
	}
	if beforeChecksum != afterChecksum {
		t.Fatalf("expected idempotent reapply checksum, before=%d after=%d", beforeChecksum, afterChecksum)
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
		t.Fatalf("query duplicate mapping count failed: %v", err)
	}
	if duplicateMappings != 0 {
		t.Fatalf("expected no duplicate legacy mappings, found %d", duplicateMappings)
	}
}
