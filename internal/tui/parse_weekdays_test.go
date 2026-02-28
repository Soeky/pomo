package tui

import (
	"testing"
	"time"
)

func TestParseOptionalHelpers(t *testing.T) {
	if got, err := parseOptionalDateTime(""); err != nil || got != nil {
		t.Fatalf("expected nil optional datetime, got=%v err=%v", got, err)
	}
	if got, err := parseOptionalDateTime("2026-03-01T09:30"); err != nil || got == nil {
		t.Fatalf("expected parsed optional datetime, got=%v err=%v", got, err)
	}
	if _, err := parseOptionalDateTime("not-a-time"); err == nil {
		t.Fatalf("expected optional datetime parse error")
	}

	if got, err := parseDurationSeconds("30m"); err != nil || got != 1800 {
		t.Fatalf("unexpected duration parse result: got=%d err=%v", got, err)
	}
	if _, err := parseDurationSeconds("0m"); err == nil {
		t.Fatalf("expected non-positive duration error")
	}

	if got, err := parseOptionalInt("", 7); err != nil || got != 7 {
		t.Fatalf("expected default optional int, got=%d err=%v", got, err)
	}
	if got, err := parseOptionalInt("42", 7); err != nil || got != 42 {
		t.Fatalf("expected parsed optional int, got=%d err=%v", got, err)
	}
	if _, err := parseOptionalInt("xx", 7); err == nil {
		t.Fatalf("expected optional int parse error")
	}
}

func TestParseRecurrenceWeekdays(t *testing.T) {
	if got, err := parseRecurrenceWeekdays(""); err != nil || got != nil {
		t.Fatalf("expected nil weekdays for empty input, got=%v err=%v", got, err)
	}

	got, err := parseRecurrenceWeekdays("MO,WE,MO")
	if err != nil {
		t.Fatalf("parseRecurrenceWeekdays failed: %v", err)
	}
	if len(got) != 2 || got[0] != time.Monday || got[1] != time.Wednesday {
		t.Fatalf("unexpected weekday parse result: %+v", got)
	}

	if _, err := parseRecurrenceWeekdays("MO,XX"); err == nil {
		t.Fatalf("expected invalid weekday token error")
	}
}
