package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestFinalizeV2CutoverBackfillsAndDisablesLegacyTriggers(t *testing.T) {
	t.Parallel()

	opened, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "finalize-v2.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer opened.Close()

	seed := seedLegacySchemaForTask2(t, opened)
	if err := RunMigrations(context.Background(), opened); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	result, err := FinalizeV2Cutover(context.Background(), opened)
	if err != nil {
		t.Fatalf("FinalizeV2Cutover failed: %v", err)
	}
	if result.AlreadyFinalized {
		t.Fatalf("expected first finalize call to execute, got already-finalized")
	}
	if result.DroppedCompatibilitySync == 0 {
		t.Fatalf("expected compatibility triggers to be dropped")
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
	if sessionEvents != sessionsCount {
		t.Fatalf("session parity mismatch after finalize: sessions=%d events=%d", sessionsCount, sessionEvents)
	}
	if plannedEvents != plannedCount {
		t.Fatalf("planned parity mismatch after finalize: planned=%d events=%d", plannedCount, plannedEvents)
	}

	var finalized string
	if err := opened.QueryRow(`SELECT value FROM app_meta WHERE key = ?`, v2FinalizedMetaKey).Scan(&finalized); err != nil {
		t.Fatalf("query app_meta finalization flag failed: %v", err)
	}
	if finalized != "true" {
		t.Fatalf("unexpected app_meta finalization value: %s", finalized)
	}

	// Verify sync trigger removal by mutating legacy rows after cutover.
	if _, err := opened.Exec(`DELETE FROM sessions WHERE id = ?`, seed.focusSessionID); err != nil {
		t.Fatalf("delete legacy session after finalize failed: %v", err)
	}
	var stillMapped int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM events WHERE legacy_source = 'sessions' AND legacy_id = ?`, seed.focusSessionID).Scan(&stillMapped); err != nil {
		t.Fatalf("count mapped session events failed: %v", err)
	}
	if stillMapped != 1 {
		t.Fatalf("expected legacy mapping to remain unchanged after trigger removal, got %d", stillMapped)
	}

	second, err := FinalizeV2Cutover(context.Background(), opened)
	if err != nil {
		t.Fatalf("second FinalizeV2Cutover failed: %v", err)
	}
	if !second.AlreadyFinalized {
		t.Fatalf("expected second finalize call to be no-op already-finalized")
	}
}

func TestFinalizeV2CutoverRejectsNilDB(t *testing.T) {
	t.Parallel()

	if _, err := FinalizeV2Cutover(context.Background(), nil); err == nil {
		t.Fatalf("expected error for nil db")
	}
}
