package stats

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
)

func TestBuildReport(t *testing.T) {
	opened := openStatsDB(t)
	defer opened.Close()

	prevConfig := config.AppConfig
	defer func() { config.AppConfig = prevConfig }()
	config.AppConfig.BreakCreditThresholdMinutes = 10

	base := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	mustExecStats(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "A", base, base.Add(25*time.Minute), int((25 * time.Minute).Seconds()), base, base)
	mustExecStats(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"break", "", base.Add(25*time.Minute), base.Add(35*time.Minute), int((10 * time.Minute).Seconds()), base, base)

	report, err := BuildReport([]string{"2026-02-25"}, base.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}
	if report.Label != "2026-02-25" {
		t.Fatalf("unexpected label: %s", report.Label)
	}
	if report.WorkTotalMin == 0 {
		t.Fatalf("expected non-zero work total")
	}
	if report.WorkEffectiveMin != report.WorkTotalMin {
		t.Fatalf("expected no break credit without trailing focus: raw=%d effective=%d", report.WorkTotalMin, report.WorkEffectiveMin)
	}
}

func TestBuildReportSemesterIncludesHierarchyBreakdown(t *testing.T) {
	opened := openStatsDB(t)
	defer opened.Close()

	prevConfig := config.AppConfig
	defer func() { config.AppConfig = prevConfig }()
	config.AppConfig.SemesterStart = "2026-02-01"

	base := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	mustExecStats(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "Math::Discrete Probability", base, base.Add(25*time.Minute), int((25 * time.Minute).Seconds()), base, base)
	mustExecStats(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "Math::Linear Algebra", base.Add(30*time.Minute), base.Add(55*time.Minute), int((25 * time.Minute).Seconds()), base, base)
	mustExecStats(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "LegacyTopicOnly", base.Add(60*time.Minute), base.Add(85*time.Minute), int((25 * time.Minute).Seconds()), base, base)

	report, err := BuildReport([]string{"sem"}, base.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("BuildReport sem failed: %v", err)
	}
	if len(report.TopDomains) == 0 || len(report.TopSubtopics) == 0 {
		t.Fatalf("expected semester report to include hierarchy aggregates")
	}

	rendered := RenderReport(report)
	if !strings.Contains(rendered, "Top domains:") || !strings.Contains(rendered, "Top subtopics:") {
		t.Fatalf("expected rendered semester report hierarchy sections, got: %s", rendered)
	}
}

func TestBuildReportRawVsEffectiveTotals(t *testing.T) {
	opened := openStatsDB(t)
	defer opened.Close()

	prevConfig := config.AppConfig
	defer func() { config.AppConfig = prevConfig }()
	config.AppConfig.BreakCreditThresholdMinutes = 10

	base := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	mustExecStats(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "Math::A", base, base.Add(25*time.Minute), int((25 * time.Minute).Seconds()), base, base)
	mustExecStats(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"break", "", base.Add(25*time.Minute), base.Add(35*time.Minute), int((10 * time.Minute).Seconds()), base, base)
	mustExecStats(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "Math::B", base.Add(35*time.Minute), base.Add(60*time.Minute), int((25 * time.Minute).Seconds()), base, base)

	report, err := BuildReport([]string{"2026-02-25"}, base.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}
	if report.WorkTotalMin != 50 {
		t.Fatalf("unexpected raw work total: %d", report.WorkTotalMin)
	}
	if report.BreakCreditMin != 10 {
		t.Fatalf("unexpected break credit total: %d", report.BreakCreditMin)
	}
	if report.WorkEffectiveMin != 60 {
		t.Fatalf("unexpected effective work total: %d", report.WorkEffectiveMin)
	}

	rendered := RenderReport(report)
	if !strings.Contains(rendered, "Worktime (raw):       00:50 h") {
		t.Fatalf("expected raw total in output, got: %s", rendered)
	}
	if !strings.Contains(rendered, "Worktime (effective): 01:00 h") {
		t.Fatalf("expected effective total in output, got: %s", rendered)
	}
}

func openStatsDB(t *testing.T) *sql.DB {
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

func mustExecStats(t *testing.T, opened *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := opened.Exec(q, args...); err != nil {
		t.Fatalf("exec failed: %v", err)
	}
}
