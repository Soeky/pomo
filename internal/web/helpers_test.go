package web

import "testing"

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
}
