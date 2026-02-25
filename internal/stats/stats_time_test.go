package stats

import (
	"testing"
	"time"
)

func TestFormatMinutesToHM(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   int
		want string
	}{
		{in: 0, want: "00:00"},
		{in: 5, want: "00:05"},
		{in: 65, want: "01:05"},
		{in: 150, want: "02:30"},
	}

	for _, tt := range tests {
		got := FormatMinutesToHM(tt.in)
		if got != tt.want {
			t.Fatalf("FormatMinutesToHM(%d): got=%q want=%q", tt.in, got, tt.want)
		}
	}
}

func TestFormatRangeName(t *testing.T) {
	t.Parallel()

	if got := FormatRangeName("all"); got != "All Time" {
		t.Fatalf("unexpected all label: %q", got)
	}
	if got := FormatRangeName("custom"); got != "custom" {
		t.Fatalf("unexpected passthrough label: %q", got)
	}
}

func TestGetTimeRangeAtBranches(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 2, 25, 12, 30, 0, 0, time.UTC) // Wednesday
	tests := []struct {
		view      string
		wantStart string
	}{
		{view: "day", wantStart: "2026-02-25"},
		{view: "week", wantStart: "2026-02-23"},
		{view: "month", wantStart: "2026-02-01"},
		{view: "year", wantStart: "2026-01-01"},
		{view: "all", wantStart: "2000-01-01"},
		{view: "unknown", wantStart: "2026-02-25"},
	}
	for _, tt := range tests {
		start, end := getTimeRangeAt(tt.view, now)
		if start.Format("2006-01-02") != tt.wantStart {
			t.Fatalf("%s start: got=%s want=%s", tt.view, start.Format("2006-01-02"), tt.wantStart)
		}
		if !end.Equal(now) {
			t.Fatalf("%s end: got=%v want=%v", tt.view, end, now)
		}
	}
}

func TestFormatRangeNameAtBranches(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 2, 25, 12, 30, 0, 0, time.UTC)
	if got := formatRangeNameAt("day", now); got != "2026-02-25" {
		t.Fatalf("unexpected day label: %q", got)
	}
	if got := formatRangeNameAt("month", now); got != "2026-02" {
		t.Fatalf("unexpected month label: %q", got)
	}
	if got := formatRangeNameAt("year", now); got != "2026" {
		t.Fatalf("unexpected year label: %q", got)
	}
	if got := formatRangeNameAt("all", now); got != "All Time" {
		t.Fatalf("unexpected all label: %q", got)
	}
	if got := formatRangeNameAt("week", now); got != "2026-02-23 – 2026-03-01" {
		t.Fatalf("unexpected week label: %q", got)
	}
}
