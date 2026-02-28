package events

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/db"
)

func TestExpandRecurrenceRuleInWindowDSTSafe(t *testing.T) {
	opened := openRecurrenceDB(t)
	defer opened.Close()

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("timezone not available: %v", err)
	}
	start := time.Date(2026, 3, 1, 9, 0, 0, 0, loc)
	rrule, err := BuildRRule(RecurrenceSpec{
		Freq:     "weekly",
		Interval: 1,
		ByDays:   []time.Weekday{time.Sunday},
	})
	if err != nil {
		t.Fatalf("BuildRRule failed: %v", err)
	}

	ruleID, err := CreateRecurrenceRule(context.Background(), RecurrenceRule{
		Title:       "Weekly Review",
		Domain:      "Planning",
		Subtopic:    "General",
		Kind:        "task",
		DurationSec: 3600,
		RRule:       rrule,
		Timezone:    "America/New_York",
		StartDate:   start,
		Active:      true,
	})
	if err != nil {
		t.Fatalf("CreateRecurrenceRule failed: %v", err)
	}

	rule, err := GetRecurrenceRuleByID(context.Background(), ruleID)
	if err != nil {
		t.Fatalf("GetRecurrenceRuleByID failed: %v", err)
	}
	out, err := ExpandRecurrenceRuleInWindow(rule, start.Add(-time.Hour), start.AddDate(0, 0, 21))
	if err != nil {
		t.Fatalf("ExpandRecurrenceRuleInWindow failed: %v", err)
	}
	if len(out) < 3 {
		t.Fatalf("expected at least 3 weekly occurrences, got %d", len(out))
	}
	for _, occ := range out[:3] {
		if got := occ.StartTime.In(loc).Hour(); got != 9 {
			t.Fatalf("expected 09:00 local recurrence time, got hour=%d at %s", got, occ.StartTime.In(loc).Format(time.RFC3339))
		}
	}
}

func TestExpandRecurrenceRuleInWindowWeeklyPatterns(t *testing.T) {
	opened := openRecurrenceDB(t)
	defer opened.Close()

	loc := time.Local
	start := time.Date(2026, 2, 2, 18, 30, 0, 0, loc) // Monday
	rrule, err := BuildRRule(RecurrenceSpec{
		Freq:     "weekly",
		Interval: 1,
		ByDays:   []time.Weekday{time.Monday, time.Wednesday},
	})
	if err != nil {
		t.Fatalf("BuildRRule failed: %v", err)
	}

	id, err := CreateRecurrenceRule(context.Background(), RecurrenceRule{
		Title:       "Evening Study",
		Domain:      "Math",
		Subtopic:    "General",
		Kind:        "task",
		DurationSec: 2700,
		RRule:       rrule,
		Timezone:    "Local",
		StartDate:   start,
		Active:      true,
	})
	if err != nil {
		t.Fatalf("CreateRecurrenceRule failed: %v", err)
	}
	rule, err := GetRecurrenceRuleByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetRecurrenceRuleByID failed: %v", err)
	}
	out, err := ExpandRecurrenceRuleInWindow(rule, start.Add(-time.Hour), start.AddDate(0, 0, 14))
	if err != nil {
		t.Fatalf("ExpandRecurrenceRuleInWindow failed: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("expected 4 occurrences in 2 weeks, got %d", len(out))
	}
	if out[0].StartTime.Weekday() != time.Monday || out[1].StartTime.Weekday() != time.Wednesday {
		t.Fatalf("unexpected weekly pattern order: %s, %s", out[0].StartTime.Weekday(), out[1].StartTime.Weekday())
	}
}

func TestExpandRecurrenceRuleInWindowMonthlyEdgeDate(t *testing.T) {
	opened := openRecurrenceDB(t)
	defer opened.Close()

	start := time.Date(2026, 1, 31, 8, 0, 0, 0, time.Local)
	rrule, err := BuildRRule(RecurrenceSpec{
		Freq:       "monthly",
		Interval:   1,
		ByMonthDay: 31,
	})
	if err != nil {
		t.Fatalf("BuildRRule failed: %v", err)
	}

	id, err := CreateRecurrenceRule(context.Background(), RecurrenceRule{
		Title:       "Month End Review",
		Domain:      "Planning",
		Subtopic:    "General",
		Kind:        "task",
		DurationSec: 1800,
		RRule:       rrule,
		Timezone:    "Local",
		StartDate:   start,
		Active:      true,
	})
	if err != nil {
		t.Fatalf("CreateRecurrenceRule failed: %v", err)
	}
	rule, err := GetRecurrenceRuleByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetRecurrenceRuleByID failed: %v", err)
	}
	out, err := ExpandRecurrenceRuleInWindow(rule, time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local), time.Date(2026, 5, 1, 0, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("ExpandRecurrenceRuleInWindow failed: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("expected 4 monthly occurrences Jan-Apr, got %d", len(out))
	}
	wantDays := []int{31, 28, 31, 30}
	for i, want := range wantDays {
		if got := out[i].StartTime.Day(); got != want {
			t.Fatalf("occurrence %d day mismatch: got=%d want=%d (%s)", i, got, want, out[i].StartTime.Format(time.RFC3339))
		}
	}
}

func TestRecurrenceRuleCRUDAndGenerationIdempotent(t *testing.T) {
	opened := openRecurrenceDB(t)
	defer opened.Close()

	start := time.Date(2026, 2, 25, 9, 0, 0, 0, time.Local)
	rrule, err := BuildRRule(RecurrenceSpec{
		Freq:     "daily",
		Interval: 1,
	})
	if err != nil {
		t.Fatalf("BuildRRule failed: %v", err)
	}

	id, err := CreateRecurrenceRule(context.Background(), RecurrenceRule{
		Title:       "Daily Focus Block",
		Domain:      "Math",
		Subtopic:    "Discrete Probability",
		Kind:        "focus",
		DurationSec: 3600,
		RRule:       rrule,
		Timezone:    "Local",
		StartDate:   start,
		Active:      true,
	})
	if err != nil {
		t.Fatalf("CreateRecurrenceRule failed: %v", err)
	}

	rules, err := ListRecurrenceRules(context.Background(), true)
	if err != nil {
		t.Fatalf("ListRecurrenceRules failed: %v", err)
	}
	if len(rules) != 1 || rules[0].ID != id {
		t.Fatalf("unexpected active rules list: %+v", rules)
	}

	rule, err := GetRecurrenceRuleByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetRecurrenceRuleByID failed: %v", err)
	}
	rule.Title = "Updated Daily Focus"
	if err := UpdateRecurrenceRule(context.Background(), id, rule); err != nil {
		t.Fatalf("UpdateRecurrenceRule failed: %v", err)
	}

	from := start.Add(-time.Hour)
	to := start.AddDate(0, 0, 3)
	result, err := GenerateRecurringEventsInWindow(context.Background(), from, to, id)
	if err != nil {
		t.Fatalf("GenerateRecurringEventsInWindow failed: %v", err)
	}
	if result.Generated == 0 {
		t.Fatalf("expected generated recurring events")
	}

	result2, err := GenerateRecurringEventsInWindow(context.Background(), from, to, id)
	if err != nil {
		t.Fatalf("GenerateRecurringEventsInWindow second run failed: %v", err)
	}
	if result2.Generated != 0 {
		t.Fatalf("expected second generation run to be idempotent, generated=%d", result2.Generated)
	}

	var count int
	if err := opened.QueryRow(`
		SELECT COUNT(1)
		FROM events
		WHERE recurrence_rule_id = ? AND source = 'recurring'`, id).Scan(&count); err != nil {
		t.Fatalf("count recurring events failed: %v", err)
	}
	if count == 0 {
		t.Fatalf("expected recurring events with recurrence_rule_id provenance")
	}

	if err := DeleteRecurrenceRule(context.Background(), id); err != nil {
		t.Fatalf("DeleteRecurrenceRule failed: %v", err)
	}
	if _, err := GetRecurrenceRuleByID(context.Background(), id); err == nil {
		t.Fatalf("expected recurrence rule to be deleted")
	}
}

func openRecurrenceDB(t *testing.T) *sql.DB {
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
