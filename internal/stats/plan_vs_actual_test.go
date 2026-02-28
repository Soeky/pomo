package stats

import (
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestQueryPlanVsActualMetricsWithSeededFixture(t *testing.T) {
	opened := openStatsDB(t)
	defer opened.Close()

	base := time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)
	end := base.AddDate(0, 0, 7)
	seedPlanVsActualFixture(t, opened, base)

	metrics, err := QueryPlanVsActualMetrics(base, end, DefaultAdherenceToleranceMinutes)
	if err != nil {
		t.Fatalf("QueryPlanVsActualMetrics failed: %v", err)
	}

	if metrics.OnTimeAdherence.PlannedCount != 3 {
		t.Fatalf("unexpected adherence planned count: %d", metrics.OnTimeAdherence.PlannedCount)
	}
	if metrics.OnTimeAdherence.OnTimeCount != 2 {
		t.Fatalf("unexpected adherence on-time count: %d", metrics.OnTimeAdherence.OnTimeCount)
	}
	if !approxFloat(metrics.OnTimeAdherence.Percent, 66.6667, 0.01) {
		t.Fatalf("unexpected adherence percent: %.4f", metrics.OnTimeAdherence.Percent)
	}

	if metrics.PlanCompletion.PlannedCount != 3 {
		t.Fatalf("unexpected completion planned count: %d", metrics.PlanCompletion.PlannedCount)
	}
	if metrics.PlanCompletion.DoneCount != 2 {
		t.Fatalf("unexpected completion done count: %d", metrics.PlanCompletion.DoneCount)
	}
	if !approxFloat(metrics.PlanCompletion.Percent, 66.6667, 0.01) {
		t.Fatalf("unexpected completion percent: %.4f", metrics.PlanCompletion.Percent)
	}

	if metrics.ScheduledMinutes != 210 || metrics.ActualMinutes != 180 || metrics.DriftMinutes != -30 {
		t.Fatalf("unexpected totals: scheduled=%d actual=%d drift=%d", metrics.ScheduledMinutes, metrics.ActualMinutes, metrics.DriftMinutes)
	}
	if len(metrics.DriftByDomain) != 2 {
		t.Fatalf("unexpected drift row count: %d", len(metrics.DriftByDomain))
	}
	if metrics.DriftByDomain[0].Domain != "Math" || metrics.DriftByDomain[0].DriftMinutes != -30 {
		t.Fatalf("unexpected top drift row: %+v", metrics.DriftByDomain[0])
	}
	if !approxFloat(metrics.WeeklyBalance.ScorePercent, 83.3333, 0.01) {
		t.Fatalf("unexpected weekly balance score: %.4f", metrics.WeeklyBalance.ScorePercent)
	}
	if metrics.WeeklyBalance.ActiveDays != 3 {
		t.Fatalf("unexpected active day count: %d", metrics.WeeklyBalance.ActiveDays)
	}
}

func TestBuildPlanVsActualAndAdherenceReportsParityWithQuery(t *testing.T) {
	opened := openStatsDB(t)
	defer opened.Close()

	base := time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)
	end := base.AddDate(0, 0, 7)
	seedPlanVsActualFixture(t, opened, base)

	queryMetrics, err := QueryPlanVsActualMetrics(base, end, DefaultAdherenceToleranceMinutes)
	if err != nil {
		t.Fatalf("QueryPlanVsActualMetrics failed: %v", err)
	}

	planReport, err := BuildPlanVsActualReport([]string{"2026-02-23", "2026-03-01"}, base.Add(10*time.Hour))
	if err != nil {
		t.Fatalf("BuildPlanVsActualReport failed: %v", err)
	}
	if planReport.Metrics.OnTimeAdherence.PlannedCount != queryMetrics.OnTimeAdherence.PlannedCount ||
		planReport.Metrics.OnTimeAdherence.OnTimeCount != queryMetrics.OnTimeAdherence.OnTimeCount ||
		!approxFloat(planReport.Metrics.OnTimeAdherence.Percent, queryMetrics.OnTimeAdherence.Percent, 0.01) {
		t.Fatalf("plan-vs-actual report parity mismatch: report=%+v query=%+v", planReport.Metrics.OnTimeAdherence, queryMetrics.OnTimeAdherence)
	}
	if planReport.Metrics.PlanCompletion.PlannedCount != queryMetrics.PlanCompletion.PlannedCount ||
		planReport.Metrics.PlanCompletion.DoneCount != queryMetrics.PlanCompletion.DoneCount ||
		!approxFloat(planReport.Metrics.PlanCompletion.Percent, queryMetrics.PlanCompletion.Percent, 0.01) {
		t.Fatalf("completion parity mismatch: report=%+v query=%+v", planReport.Metrics.PlanCompletion, queryMetrics.PlanCompletion)
	}
	if planReport.Metrics.ScheduledMinutes != queryMetrics.ScheduledMinutes ||
		planReport.Metrics.ActualMinutes != queryMetrics.ActualMinutes ||
		planReport.Metrics.DriftMinutes != queryMetrics.DriftMinutes {
		t.Fatalf("total parity mismatch: report=%+v query=%+v", planReport.Metrics, queryMetrics)
	}
	if !approxFloat(planReport.Metrics.WeeklyBalance.ScorePercent, queryMetrics.WeeklyBalance.ScorePercent, 0.01) {
		t.Fatalf("weekly balance parity mismatch: report=%.4f query=%.4f", planReport.Metrics.WeeklyBalance.ScorePercent, queryMetrics.WeeklyBalance.ScorePercent)
	}

	adherenceReport, err := BuildAdherenceReport([]string{"2026-02-23", "2026-03-01"}, base.Add(10*time.Hour))
	if err != nil {
		t.Fatalf("BuildAdherenceReport failed: %v", err)
	}
	if adherenceReport.Summary.PlannedCount != queryMetrics.OnTimeAdherence.PlannedCount ||
		adherenceReport.Summary.OnTimeCount != queryMetrics.OnTimeAdherence.OnTimeCount ||
		!approxFloat(adherenceReport.Summary.Percent, queryMetrics.OnTimeAdherence.Percent, 0.01) {
		t.Fatalf("adherence report parity mismatch: report=%+v query=%+v", adherenceReport.Summary, queryMetrics.OnTimeAdherence)
	}
}

func TestRenderPlanVsActualAndAdherenceReports(t *testing.T) {
	plan := RenderPlanVsActualReport(PlanVsActualReport{
		Label: "2026-02-23 – 2026-03-01",
		Metrics: PlanVsActualMetrics{
			OnTimeAdherence:  OnTimeAdherenceSummary{PlannedCount: 3, OnTimeCount: 2, Percent: 66.7, ToleranceMinutes: 10},
			PlanCompletion:   PlanCompletionSummary{PlannedCount: 3, DoneCount: 2, Percent: 66.7},
			WeeklyBalance:    WeeklyBalanceSummary{ScorePercent: 83.3, ActiveDays: 3},
			ScheduledMinutes: 210,
			ActualMinutes:    180,
			DriftMinutes:     -30,
			DriftByDomain:    []DomainDriftRow{{Domain: "Math", ScheduledMinutes: 120, ActualMinutes: 90, DriftMinutes: -30}},
		},
	})
	for _, needle := range []string{"Plan vs Actual", "On-time adherence", "Plan completion", "Weekly balance", "Drift by domain"} {
		if !strings.Contains(plan, needle) {
			t.Fatalf("plan-vs-actual render missing %q", needle)
		}
	}

	adherence := RenderAdherenceReport(AdherenceReport{
		Label:   "2026-02-23 – 2026-03-01",
		Summary: OnTimeAdherenceSummary{PlannedCount: 3, OnTimeCount: 2, Percent: 66.7, ToleranceMinutes: 10},
	})
	for _, needle := range []string{"Adherence", "Tolerance", "Planned blocks", "On-time starts"} {
		if !strings.Contains(adherence, needle) {
			t.Fatalf("adherence render missing %q", needle)
		}
	}
}

func seedPlanVsActualFixture(t *testing.T, opened *sql.DB, base time.Time) {
	t.Helper()

	mustExecStats(t, opened, `INSERT INTO planned_events(title, domain, subtopic, description, start_time, end_time, status, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Math::Algebra", "Math", "Algebra", "", base.Add(9*time.Hour), base.Add(10*time.Hour), "done", "manual", base, base)
	mustExecStats(t, opened, `INSERT INTO planned_events(title, domain, subtopic, description, start_time, end_time, status, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Math::Geometry", "Math", "Geometry", "", base.AddDate(0, 0, 1).Add(9*time.Hour), base.AddDate(0, 0, 1).Add(10*time.Hour), "planned", "manual", base, base)
	mustExecStats(t, opened, `INSERT INTO planned_events(title, domain, subtopic, description, start_time, end_time, status, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Science::Lab", "Science", "Lab", "", base.AddDate(0, 0, 2).Add(14*time.Hour), base.AddDate(0, 0, 2).Add(15*time.Hour+30*time.Minute), "done", "manual", base, base)
	mustExecStats(t, opened, `INSERT INTO planned_events(title, domain, subtopic, description, start_time, end_time, status, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Ops::Admin", "Ops", "Admin", "", base.AddDate(0, 0, 3).Add(10*time.Hour), base.AddDate(0, 0, 3).Add(10*time.Hour+30*time.Minute), "canceled", "manual", base, base)

	mustExecStats(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "Math::Algebra", base.Add(9*time.Hour+5*time.Minute), base.Add(10*time.Hour+5*time.Minute), int((60 * time.Minute).Seconds()), base, base)
	mustExecStats(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "Math::Geometry", base.AddDate(0, 0, 1).Add(9*time.Hour+20*time.Minute), base.AddDate(0, 0, 1).Add(9*time.Hour+50*time.Minute), int((30 * time.Minute).Seconds()), base, base)
	mustExecStats(t, opened, `INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "Science::Lab", base.AddDate(0, 0, 2).Add(13*time.Hour+55*time.Minute), base.AddDate(0, 0, 2).Add(15*time.Hour+25*time.Minute), int((90 * time.Minute).Seconds()), base, base)
}

func approxFloat(left, right, epsilon float64) bool {
	if left > right {
		return left-right <= epsilon
	}
	return right-left <= epsilon
}
