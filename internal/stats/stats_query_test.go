package stats

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
)

func TestQueryStatsAndBlocks(t *testing.T) {
	opened, err := db.Open(filepath.Join(t.TempDir(), "pomo.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer opened.Close()

	prevDB := db.DB
	db.DB = opened
	defer func() { db.DB = prevDB }()

	base := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	mustExec(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "A", base, base.Add(25*time.Minute), int((25 * time.Minute).Seconds()), base, base)
	mustExec(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "A", base.Add(30*time.Minute), base.Add(55*time.Minute), int((25 * time.Minute).Seconds()), base, base)
	mustExec(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"break", "", base.Add(55*time.Minute), base.Add(65*time.Minute), int((10 * time.Minute).Seconds()), base, base)
	mustExec(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "B", base.Add(70*time.Minute), base.Add(95*time.Minute), int((25 * time.Minute).Seconds()), base, base)

	from := base.Add(-time.Minute)
	to := base.Add(3 * time.Hour)

	focus, brk, err := QueryStats(from, to)
	if err != nil {
		t.Fatalf("QueryStats failed: %v", err)
	}
	if len(focus) != 2 {
		t.Fatalf("expected 2 focus groups, got %d", len(focus))
	}
	if brk.Count != 1 || brk.TotalMinutes != 10 {
		t.Fatalf("unexpected break stats: %+v", brk)
	}

	blocks, err := QuerySessionBlocks(from, to)
	if err != nil {
		t.Fatalf("QuerySessionBlocks failed: %v", err)
	}
	if len(blocks) != 3 {
		t.Fatalf("expected 3 merged blocks, got %d", len(blocks))
	}
	if blocks[0].Type != "focus" || blocks[0].Duration != int((50*time.Minute).Seconds()) {
		t.Fatalf("unexpected first merged block: %+v", blocks[0])
	}
}

func TestGetTimeRangeSemUsesConfiguredDate(t *testing.T) {
	prev := config.AppConfig
	defer func() { config.AppConfig = prev }()
	config.AppConfig.SemesterStart = "2025-10-15"

	start, end := GetTimeRange("sem")
	if start.Format("2006-01-02") != "2025-10-15" {
		t.Fatalf("unexpected sem start: %s", start.Format("2006-01-02"))
	}
	if !end.After(start) {
		t.Fatalf("expected end after start")
	}
}

func TestQueryTopicHierarchyStats(t *testing.T) {
	opened, err := db.Open(filepath.Join(t.TempDir(), "pomo.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer opened.Close()

	prevDB := db.DB
	db.DB = opened
	defer func() { db.DB = prevDB }()

	base := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	mustExec(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "Math::Discrete Probability", base, base.Add(30*time.Minute), int((30 * time.Minute).Seconds()), base, base)
	mustExec(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "Math::Linear Algebra", base.Add(35*time.Minute), base.Add(65*time.Minute), int((30 * time.Minute).Seconds()), base, base)
	mustExec(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "LegacyTopicOnly", base.Add(70*time.Minute), base.Add(95*time.Minute), int((25 * time.Minute).Seconds()), base, base)
	mustExec(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"break", "", base.Add(100*time.Minute), base.Add(110*time.Minute), int((10 * time.Minute).Seconds()), base, base)

	from := base.Add(-time.Minute)
	to := base.Add(3 * time.Hour)
	domains, subtopics, err := QueryTopicHierarchyStats(from, to)
	if err != nil {
		t.Fatalf("QueryTopicHierarchyStats failed: %v", err)
	}
	if len(domains) < 2 {
		t.Fatalf("expected at least 2 domain groups, got %d", len(domains))
	}
	if domains[0].Name != "Math" || domains[0].TotalMinutes != 60 {
		t.Fatalf("unexpected top domain aggregate: %+v", domains[0])
	}

	foundGeneral := false
	for _, s := range subtopics {
		if s.Name == "General" {
			foundGeneral = true
			if s.TotalMinutes != 25 {
				t.Fatalf("unexpected General aggregate minutes: %+v", s)
			}
		}
	}
	if !foundGeneral {
		t.Fatalf("expected General subtopic aggregate to include legacy topic rows")
	}
}

func mustExec(t *testing.T, opened *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := opened.Exec(query, args...); err != nil {
		t.Fatalf("exec failed: %v", err)
	}
}
