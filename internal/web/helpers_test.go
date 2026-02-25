package web

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	pomodb "github.com/Soeky/pomo/internal/db"
)

func TestIsReadOnlySQL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		query string
		want  bool
	}{
		{query: "SELECT * FROM sessions", want: true},
		{query: " with cte as (select 1) select * from cte", want: true},
		{query: "PRAGMA table_info(sessions)", want: true},
		{query: "UPDATE sessions SET topic='x'", want: false},
		{query: "SELECT * FROM sessions; DELETE FROM sessions", want: false},
		{query: "DROP TABLE sessions", want: false},
	}

	for _, tt := range tests {
		got := isReadOnlySQL(tt.query)
		if got != tt.want {
			t.Fatalf("isReadOnlySQL(%q): got=%v want=%v", tt.query, got, tt.want)
		}
	}
}

func TestParsePrefixedID(t *testing.T) {
	t.Parallel()

	kind, id, err := parsePrefixedID("p-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != "p" || id != 42 {
		t.Fatalf("unexpected parse result: %s %d", kind, id)
	}

	if _, _, err := parsePrefixedID("bad"); err == nil {
		t.Fatalf("expected error for invalid prefixed id")
	}
	if _, _, err := parsePrefixedID("x-1"); err == nil {
		t.Fatalf("expected error for unsupported prefix")
	}
	if _, _, err := parsePrefixedID("p-a"); err == nil {
		t.Fatalf("expected error for invalid numeric id")
	}
}

func TestParseAnyTimeAndDefault(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"2026-02-25T10:00:00Z",
		"2026-02-25T10:00",
		"2026-02-25 10:00:00",
		"2026-02-25 10:00",
	}
	for _, in := range inputs {
		if _, err := parseAnyTime(in); err != nil {
			t.Fatalf("expected parseAnyTime to parse %q, got error: %v", in, err)
		}
	}
	if _, err := parseAnyTime(""); err == nil {
		t.Fatalf("expected empty parse error")
	}
	if _, err := parseAnyTime("invalid"); err == nil {
		t.Fatalf("expected invalid parse error")
	}

	fallback := time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)
	got := parseTimeOrDefault("invalid", fallback)
	if !got.Equal(fallback) {
		t.Fatalf("expected fallback time, got %v", got)
	}
}

func TestListTables(t *testing.T) {
	t.Parallel()

	if _, err := listTables(context.Background(), nil); err == nil {
		t.Fatalf("expected error for nil db")
	}

	opened, err := pomodb.Open(filepath.Join(t.TempDir(), "pomo.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer opened.Close()

	tables, err := listTables(context.Background(), opened)
	if err != nil {
		t.Fatalf("listTables failed: %v", err)
	}
	if len(tables) == 0 {
		t.Fatalf("expected non-empty table list")
	}
}
