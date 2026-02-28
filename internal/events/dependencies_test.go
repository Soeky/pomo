package events

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAddDependencyCycleDetection(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	aID := mustCreateEvent(t, "Lecture", "Math", "Discrete", "planned", "manual", time.Date(2026, 3, 3, 9, 0, 0, 0, time.UTC), 60*time.Minute)
	bID := mustCreateEvent(t, "Tutorial", "Math", "Discrete", "planned", "manual", time.Date(2026, 3, 3, 11, 0, 0, 0, time.UTC), 60*time.Minute)
	cID := mustCreateEvent(t, "Homework", "Math", "Discrete", "planned", "manual", time.Date(2026, 3, 3, 13, 0, 0, 0, time.UTC), 60*time.Minute)

	if err := AddDependency(context.Background(), bID, aID, true); err != nil {
		t.Fatalf("AddDependency b->a failed: %v", err)
	}
	if err := AddDependency(context.Background(), cID, bID, true); err != nil {
		t.Fatalf("AddDependency c->b failed: %v", err)
	}
	err := AddDependency(context.Background(), aID, cID, true)
	if !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("expected cycle detection error, got %v", err)
	}
}

func TestDependencyBlockedUnblockedTransitions(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	lectureID := mustCreateEvent(t, "Lecture", "Math", "Discrete", "planned", "manual", time.Date(2026, 3, 4, 9, 0, 0, 0, time.UTC), 60*time.Minute)
	tutorialID := mustCreateEvent(t, "Tutorial", "Math", "Discrete", "planned", "manual", time.Date(2026, 3, 4, 11, 0, 0, 0, time.UTC), 60*time.Minute)

	if err := AddDependency(context.Background(), tutorialID, lectureID, true); err != nil {
		t.Fatalf("AddDependency tutorial->lecture failed: %v", err)
	}

	tutorial, err := GetByID(context.Background(), tutorialID)
	if err != nil {
		t.Fatalf("GetByID tutorial failed: %v", err)
	}
	if tutorial.Status != "blocked" {
		t.Fatalf("expected tutorial to be blocked, got status=%s", tutorial.Status)
	}
	if !strings.Contains(strings.ToLower(tutorial.BlockedReason), "lecture") {
		t.Fatalf("expected blocked reason to mention lecture, got %q", tutorial.BlockedReason)
	}

	lecture, err := GetByID(context.Background(), lectureID)
	if err != nil {
		t.Fatalf("GetByID lecture failed: %v", err)
	}
	lecture.Status = "done"
	if err := Update(context.Background(), lectureID, lecture); err != nil {
		t.Fatalf("Update lecture->done failed: %v", err)
	}

	tutorial, err = GetByID(context.Background(), tutorialID)
	if err != nil {
		t.Fatalf("GetByID tutorial after done failed: %v", err)
	}
	if tutorial.Status != "planned" {
		t.Fatalf("expected tutorial to unblock to planned, got status=%s", tutorial.Status)
	}
	if strings.TrimSpace(tutorial.BlockedReason) != "" {
		t.Fatalf("expected cleared blocked reason after unblocking, got %q", tutorial.BlockedReason)
	}

	lecture.Status = "planned"
	if err := Update(context.Background(), lectureID, lecture); err != nil {
		t.Fatalf("Update lecture->planned failed: %v", err)
	}
	tutorial, err = GetByID(context.Background(), tutorialID)
	if err != nil {
		t.Fatalf("GetByID tutorial after reprime failed: %v", err)
	}
	if tutorial.Status != "blocked" {
		t.Fatalf("expected tutorial to re-block after prerequisite reverted, got status=%s", tutorial.Status)
	}
}

func TestDependencyOverrideRequiresAdminAndWritesAuditLog(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	lectureID := mustCreateEvent(t, "Lecture", "Math", "Discrete", "planned", "manual", time.Date(2026, 3, 5, 9, 0, 0, 0, time.UTC), 60*time.Minute)
	tutorialID := mustCreateEvent(t, "Tutorial", "Math", "Discrete", "planned", "manual", time.Date(2026, 3, 5, 11, 0, 0, 0, time.UTC), 60*time.Minute)
	if err := AddDependency(context.Background(), tutorialID, lectureID, true); err != nil {
		t.Fatalf("AddDependency tutorial->lecture failed: %v", err)
	}

	if err := SetDependencyOverride(context.Background(), tutorialID, true, false, "operator request", "test"); !errors.Is(err, ErrOverrideNotAllowed) {
		t.Fatalf("expected admin-required override error, got %v", err)
	}

	if err := SetDependencyOverride(context.Background(), tutorialID, true, true, "operator request", "test"); err != nil {
		t.Fatalf("SetDependencyOverride enable failed: %v", err)
	}

	tutorial, err := GetByID(context.Background(), tutorialID)
	if err != nil {
		t.Fatalf("GetByID tutorial after override failed: %v", err)
	}
	if tutorial.Status != "planned" || !tutorial.BlockedOverride {
		t.Fatalf("expected planned+override=true after enable, got status=%s override=%v", tutorial.Status, tutorial.BlockedOverride)
	}

	var enableAuditCount int
	if err := opened.QueryRow(`
		SELECT COUNT(1)
		FROM audit_log
		WHERE entity_type = 'event'
		  AND entity_id = ?
		  AND action = 'dependency_override_enable'`, tutorialID).Scan(&enableAuditCount); err != nil {
		t.Fatalf("count enable override audit rows failed: %v", err)
	}
	if enableAuditCount == 0 {
		t.Fatalf("expected dependency_override_enable audit row")
	}

	if err := SetDependencyOverride(context.Background(), tutorialID, false, true, "clear", "test"); err != nil {
		t.Fatalf("SetDependencyOverride clear failed: %v", err)
	}
	tutorial, err = GetByID(context.Background(), tutorialID)
	if err != nil {
		t.Fatalf("GetByID tutorial after clear failed: %v", err)
	}
	if tutorial.Status != "blocked" || tutorial.BlockedOverride {
		t.Fatalf("expected blocked+override=false after clear, got status=%s override=%v", tutorial.Status, tutorial.BlockedOverride)
	}

	var disableAuditCount int
	if err := opened.QueryRow(`
		SELECT COUNT(1)
		FROM audit_log
		WHERE entity_type = 'event'
		  AND entity_id = ?
		  AND action = 'dependency_override_disable'`, tutorialID).Scan(&disableAuditCount); err != nil {
		t.Fatalf("count disable override audit rows failed: %v", err)
	}
	if disableAuditCount == 0 {
		t.Fatalf("expected dependency_override_disable audit row")
	}
}

func mustCreateEvent(t *testing.T, title, domain, subtopic, status, source string, start time.Time, dur time.Duration) int64 {
	t.Helper()
	id, err := Create(context.Background(), Event{
		Kind:      "task",
		Title:     title,
		Domain:    domain,
		Subtopic:  subtopic,
		StartTime: start,
		EndTime:   start.Add(dur),
		Layer:     "planned",
		Status:    status,
		Source:    source,
	})
	if err != nil {
		t.Fatalf("Create event %s failed: %v", title, err)
	}
	return id
}
