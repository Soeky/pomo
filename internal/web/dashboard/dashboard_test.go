package dashboard

import (
	"context"
	"database/sql"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/stats"
)

func TestRegistryByID(t *testing.T) {
	t.Parallel()

	r := NewRegistry(nil)
	if _, ok := r.ByID("totals"); !ok {
		t.Fatalf("expected totals module to be registered")
	}
	if _, ok := r.ByID("missing"); ok {
		t.Fatalf("did not expect missing module to be registered")
	}

	defs := r.Definitions()
	if len(defs) != 7 {
		t.Fatalf("expected 7 dashboard definitions, got %d", len(defs))
	}
}

func TestRegistryAllWithoutDB(t *testing.T) {
	t.Parallel()

	r := NewRegistry(nil)
	_, err := r.All(context.Background(), time.Now().Add(-time.Hour), time.Now())
	if err == nil {
		t.Fatalf("expected error without db")
	}
	if !strings.Contains(err.Error(), "database is not initialized") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryAllWithDB(t *testing.T) {
	opened, err := db.Open(filepath.Join(t.TempDir(), "pomo.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer opened.Close()

	start := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	end := start.Add(25 * time.Minute)
	if err := seedDashboardData(opened, start, end); err != nil {
		t.Fatalf("seedDashboardData failed: %v", err)
	}

	r := NewRegistry(opened)
	defs, err := r.All(context.Background(), start.Add(-time.Hour), end.Add(time.Hour))
	if err != nil {
		t.Fatalf("Registry.All failed: %v", err)
	}
	if len(defs) != 7 {
		t.Fatalf("expected 7 dashboard modules, got %d", len(defs))
	}
}

func TestTotalsModuleIncludesEffectiveFocus(t *testing.T) {
	opened, err := db.Open(filepath.Join(t.TempDir(), "pomo.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer opened.Close()

	prevConfig := config.AppConfig
	defer func() { config.AppConfig = prevConfig }()
	config.AppConfig.BreakCreditThresholdMinutes = 10

	start := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	if _, err := opened.Exec(`INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "Math::A", start, start.Add(25*time.Minute), int((25 * time.Minute).Seconds()), start, start); err != nil {
		t.Fatalf("insert focus #1 failed: %v", err)
	}
	if _, err := opened.Exec(`INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"break", "", start.Add(25*time.Minute), start.Add(35*time.Minute), int((10 * time.Minute).Seconds()), start, start); err != nil {
		t.Fatalf("insert break failed: %v", err)
	}
	if _, err := opened.Exec(`INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "Math::B", start.Add(35*time.Minute), start.Add(60*time.Minute), int((25 * time.Minute).Seconds()), start, start); err != nil {
		t.Fatalf("insert focus #2 failed: %v", err)
	}

	r := NewRegistry(opened)
	module, ok := r.ByID("totals")
	if !ok {
		t.Fatalf("totals module missing")
	}

	dataAny, err := module.Load(context.Background(), start.Add(-time.Minute), start.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("totals load failed: %v", err)
	}
	data, ok := dataAny.(TotalsData)
	if !ok {
		t.Fatalf("unexpected totals data type: %T", dataAny)
	}
	if data.FocusMinutes != 50 {
		t.Fatalf("unexpected raw focus minutes: %d", data.FocusMinutes)
	}
	if data.EffectiveFocusMinutes != 60 {
		t.Fatalf("unexpected effective focus minutes: %d", data.EffectiveFocusMinutes)
	}
	if data.BreakCreditMinutes != 10 {
		t.Fatalf("unexpected break credit minutes: %d", data.BreakCreditMinutes)
	}
	if data.BreakMinutes != 10 {
		t.Fatalf("unexpected raw break minutes: %d", data.BreakMinutes)
	}
}

func TestPlanVsActualModulesAcrossDataStates(t *testing.T) {
	base := time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)
	end := base.AddDate(0, 0, 7)

	tests := []struct {
		name                    string
		seed                    func(*testing.T, *sql.DB, time.Time)
		wantPlanned             int
		wantDone                int
		wantOnTime              int
		wantCompletionPercent   float64
		wantAdherencePercent    float64
		wantScheduledMinutes    int
		wantActualMinutes       int
		wantDriftMinutes        int
		wantBalancePercent      float64
		wantBalanceActiveDays   int
		wantDomainDriftRowCount int
	}{
		{
			name:                    "empty",
			seed:                    nil,
			wantPlanned:             0,
			wantDone:                0,
			wantOnTime:              0,
			wantCompletionPercent:   0,
			wantAdherencePercent:    0,
			wantScheduledMinutes:    0,
			wantActualMinutes:       0,
			wantDriftMinutes:        0,
			wantBalancePercent:      0,
			wantBalanceActiveDays:   0,
			wantDomainDriftRowCount: 0,
		},
		{
			name:                    "partial",
			seed:                    seedPlanVsActualPartialData,
			wantPlanned:             2,
			wantDone:                1,
			wantOnTime:              0,
			wantCompletionPercent:   50,
			wantAdherencePercent:    0,
			wantScheduledMinutes:    120,
			wantActualMinutes:       0,
			wantDriftMinutes:        -120,
			wantBalancePercent:      0,
			wantBalanceActiveDays:   2,
			wantDomainDriftRowCount: 2,
		},
		{
			name:                    "full",
			seed:                    seedPlanVsActualFullData,
			wantPlanned:             3,
			wantDone:                2,
			wantOnTime:              2,
			wantCompletionPercent:   66.6667,
			wantAdherencePercent:    66.6667,
			wantScheduledMinutes:    210,
			wantActualMinutes:       180,
			wantDriftMinutes:        -30,
			wantBalancePercent:      83.3333,
			wantBalanceActiveDays:   3,
			wantDomainDriftRowCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opened, err := db.Open(filepath.Join(t.TempDir(), "pomo.db"))
			if err != nil {
				t.Fatalf("db.Open failed: %v", err)
			}
			defer opened.Close()

			if tt.seed != nil {
				tt.seed(t, opened, base)
			}

			r := NewRegistry(opened)

			adherenceAny, err := mustLoadModule(r, "on_time_adherence", base, end)
			if err != nil {
				t.Fatalf("load adherence module failed: %v", err)
			}
			adherence := adherenceAny.(adherenceData)

			completionAny, err := mustLoadModule(r, "planned_completion", base, end)
			if err != nil {
				t.Fatalf("load completion module failed: %v", err)
			}
			completion := completionAny.(CompletionData)

			driftAny, err := mustLoadModule(r, "drift_by_domain", base, end)
			if err != nil {
				t.Fatalf("load drift module failed: %v", err)
			}
			drift := driftAny.(driftByDomainData)

			balanceAny, err := mustLoadModule(r, "weekly_balance", base, end)
			if err != nil {
				t.Fatalf("load weekly balance module failed: %v", err)
			}
			balance := balanceAny.(weeklyBalanceData)

			if adherence.PlannedCount != tt.wantPlanned {
				t.Fatalf("adherence planned count mismatch: got=%d want=%d", adherence.PlannedCount, tt.wantPlanned)
			}
			if adherence.OnTimeCount != tt.wantOnTime {
				t.Fatalf("adherence on-time mismatch: got=%d want=%d", adherence.OnTimeCount, tt.wantOnTime)
			}
			if !floatEquals(adherence.Percent, tt.wantAdherencePercent, 0.01) {
				t.Fatalf("adherence percent mismatch: got=%.4f want=%.4f", adherence.Percent, tt.wantAdherencePercent)
			}

			if completion.PlannedCount != tt.wantPlanned {
				t.Fatalf("completion planned mismatch: got=%d want=%d", completion.PlannedCount, tt.wantPlanned)
			}
			if completion.DoneCount != tt.wantDone {
				t.Fatalf("completion done mismatch: got=%d want=%d", completion.DoneCount, tt.wantDone)
			}
			if !floatEquals(completion.Percent, tt.wantCompletionPercent, 0.01) {
				t.Fatalf("completion percent mismatch: got=%.4f want=%.4f", completion.Percent, tt.wantCompletionPercent)
			}

			if drift.ScheduledMinutes != tt.wantScheduledMinutes {
				t.Fatalf("drift scheduled mismatch: got=%d want=%d", drift.ScheduledMinutes, tt.wantScheduledMinutes)
			}
			if drift.ActualMinutes != tt.wantActualMinutes {
				t.Fatalf("drift actual mismatch: got=%d want=%d", drift.ActualMinutes, tt.wantActualMinutes)
			}
			if drift.DriftMinutes != tt.wantDriftMinutes {
				t.Fatalf("drift total mismatch: got=%d want=%d", drift.DriftMinutes, tt.wantDriftMinutes)
			}
			if len(drift.Rows) != tt.wantDomainDriftRowCount {
				t.Fatalf("drift row count mismatch: got=%d want=%d", len(drift.Rows), tt.wantDomainDriftRowCount)
			}

			if !floatEquals(balance.ScorePercent, tt.wantBalancePercent, 0.01) {
				t.Fatalf("balance score mismatch: got=%.4f want=%.4f", balance.ScorePercent, tt.wantBalancePercent)
			}
			if balance.ActiveDays != tt.wantBalanceActiveDays {
				t.Fatalf("balance active days mismatch: got=%d want=%d", balance.ActiveDays, tt.wantBalanceActiveDays)
			}
		})
	}
}

func TestPlanVsActualModuleParityWithStatsReport(t *testing.T) {
	opened, err := db.Open(filepath.Join(t.TempDir(), "pomo.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer opened.Close()

	base := time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)
	end := base.AddDate(0, 0, 7)
	seedPlanVsActualFullData(t, opened, base)

	r := NewRegistry(opened)
	metrics, err := stats.QueryPlanVsActualMetricsWithQuerier(context.Background(), opened, base, end, stats.DefaultAdherenceToleranceMinutes)
	if err != nil {
		t.Fatalf("QueryPlanVsActualMetricsWithQuerier failed: %v", err)
	}

	adherenceAny, err := mustLoadModule(r, "on_time_adherence", base, end)
	if err != nil {
		t.Fatalf("load adherence module failed: %v", err)
	}
	adherence := adherenceAny.(adherenceData)
	if adherence.PlannedCount != metrics.OnTimeAdherence.PlannedCount || adherence.OnTimeCount != metrics.OnTimeAdherence.OnTimeCount || !floatEquals(adherence.Percent, metrics.OnTimeAdherence.Percent, 0.01) {
		t.Fatalf("adherence module parity mismatch: module=%+v metrics=%+v", adherence, metrics.OnTimeAdherence)
	}

	completionAny, err := mustLoadModule(r, "planned_completion", base, end)
	if err != nil {
		t.Fatalf("load completion module failed: %v", err)
	}
	completion := completionAny.(CompletionData)
	if completion.PlannedCount != metrics.PlanCompletion.PlannedCount || completion.DoneCount != metrics.PlanCompletion.DoneCount || !floatEquals(completion.Percent, metrics.PlanCompletion.Percent, 0.01) {
		t.Fatalf("completion module parity mismatch: module=%+v metrics=%+v", completion, metrics.PlanCompletion)
	}

	driftAny, err := mustLoadModule(r, "drift_by_domain", base, end)
	if err != nil {
		t.Fatalf("load drift module failed: %v", err)
	}
	drift := driftAny.(driftByDomainData)
	if drift.ScheduledMinutes != metrics.ScheduledMinutes || drift.ActualMinutes != metrics.ActualMinutes || drift.DriftMinutes != metrics.DriftMinutes {
		t.Fatalf("drift totals parity mismatch: module=%+v metrics=(scheduled=%d actual=%d drift=%d)", drift, metrics.ScheduledMinutes, metrics.ActualMinutes, metrics.DriftMinutes)
	}
	if len(drift.Rows) != len(metrics.DriftByDomain) {
		t.Fatalf("drift rows parity mismatch: module=%d metrics=%d", len(drift.Rows), len(metrics.DriftByDomain))
	}

	balanceAny, err := mustLoadModule(r, "weekly_balance", base, end)
	if err != nil {
		t.Fatalf("load balance module failed: %v", err)
	}
	balance := balanceAny.(weeklyBalanceData)
	if !floatEquals(balance.ScorePercent, metrics.WeeklyBalance.ScorePercent, 0.01) || balance.ActiveDays != metrics.WeeklyBalance.ActiveDays {
		t.Fatalf("weekly balance module parity mismatch: module=%+v metrics=%+v", balance, metrics.WeeklyBalance)
	}

	prevDB := db.DB
	db.DB = opened
	t.Cleanup(func() { db.DB = prevDB })

	cliReport, err := stats.BuildPlanVsActualReport([]string{"2026-02-23", "2026-03-01"}, base.Add(12*time.Hour))
	if err != nil {
		t.Fatalf("BuildPlanVsActualReport failed: %v", err)
	}
	if !floatEquals(cliReport.Metrics.OnTimeAdherence.Percent, adherence.Percent, 0.01) ||
		cliReport.Metrics.OnTimeAdherence.OnTimeCount != adherence.OnTimeCount ||
		cliReport.Metrics.OnTimeAdherence.PlannedCount != adherence.PlannedCount {
		t.Fatalf("CLI/dashboard adherence parity mismatch: cli=%+v dashboard=%+v", cliReport.Metrics.OnTimeAdherence, adherence)
	}
	if !floatEquals(cliReport.Metrics.PlanCompletion.Percent, completion.Percent, 0.01) ||
		cliReport.Metrics.PlanCompletion.DoneCount != completion.DoneCount ||
		cliReport.Metrics.PlanCompletion.PlannedCount != completion.PlannedCount {
		t.Fatalf("CLI/dashboard completion parity mismatch: cli=%+v dashboard=%+v", cliReport.Metrics.PlanCompletion, completion)
	}
	if !floatEquals(cliReport.Metrics.WeeklyBalance.ScorePercent, balance.ScorePercent, 0.01) ||
		cliReport.Metrics.WeeklyBalance.ActiveDays != balance.ActiveDays {
		t.Fatalf("CLI/dashboard weekly balance parity mismatch: cli=%+v dashboard=%+v", cliReport.Metrics.WeeklyBalance, balance)
	}
	if cliReport.Metrics.ScheduledMinutes != drift.ScheduledMinutes ||
		cliReport.Metrics.ActualMinutes != drift.ActualMinutes ||
		cliReport.Metrics.DriftMinutes != drift.DriftMinutes {
		t.Fatalf("CLI/dashboard drift parity mismatch: cli=(scheduled=%d actual=%d drift=%d) dashboard=%+v",
			cliReport.Metrics.ScheduledMinutes, cliReport.Metrics.ActualMinutes, cliReport.Metrics.DriftMinutes, drift)
	}
}

func mustLoadModule(r *Registry, id string, start, end time.Time) (any, error) {
	module, ok := r.ByID(id)
	if !ok {
		return nil, sql.ErrNoRows
	}
	return module.Load(context.Background(), start, end)
}

func seedDashboardData(opened *sql.DB, start, end time.Time) error {
	if _, err := opened.Exec(`INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "A", start, end, int((25 * time.Minute).Seconds()), start, start); err != nil {
		return err
	}
	if _, err := opened.Exec(`INSERT INTO planned_events(title, domain, subtopic, description, start_time, end_time, status, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"P1", "P1", "General", "d", start, end, "done", "manual", start, start); err != nil {
		return err
	}
	return nil
}

func seedPlanVsActualPartialData(t *testing.T, opened *sql.DB, base time.Time) {
	t.Helper()

	insertPlannedEvent(t, opened, "Math::Algebra", "Math", "Algebra", base.Add(9*time.Hour), 60*time.Minute, "done")
	insertPlannedEvent(t, opened, "Science::Lab", "Science", "Lab", base.AddDate(0, 0, 1).Add(14*time.Hour), 60*time.Minute, "planned")
}

func seedPlanVsActualFullData(t *testing.T, opened *sql.DB, base time.Time) {
	t.Helper()

	insertPlannedEvent(t, opened, "Math::Algebra", "Math", "Algebra", base.Add(9*time.Hour), 60*time.Minute, "done")
	insertPlannedEvent(t, opened, "Math::Geometry", "Math", "Geometry", base.AddDate(0, 0, 1).Add(9*time.Hour), 60*time.Minute, "planned")
	insertPlannedEvent(t, opened, "Science::Lab", "Science", "Lab", base.AddDate(0, 0, 2).Add(14*time.Hour), 90*time.Minute, "done")
	insertPlannedEvent(t, opened, "Ops::Admin", "Ops", "Admin", base.AddDate(0, 0, 3).Add(10*time.Hour), 30*time.Minute, "canceled")

	insertSession(t, opened, "Math::Algebra", base.Add(9*time.Hour+5*time.Minute), 60*time.Minute)
	insertSession(t, opened, "Math::Geometry", base.AddDate(0, 0, 1).Add(9*time.Hour+20*time.Minute), 30*time.Minute)
	insertSession(t, opened, "Science::Lab", base.AddDate(0, 0, 2).Add(13*time.Hour+55*time.Minute), 90*time.Minute)
}

func insertPlannedEvent(t *testing.T, opened *sql.DB, title, domain, subtopic string, start time.Time, duration time.Duration, status string) {
	t.Helper()
	end := start.Add(duration)
	if _, err := opened.Exec(`INSERT INTO planned_events(title, domain, subtopic, description, start_time, end_time, status, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		title, domain, subtopic, "", start, end, status, "manual", start, start); err != nil {
		t.Fatalf("insert planned event failed: %v", err)
	}
}

func insertSession(t *testing.T, opened *sql.DB, topic string, start time.Time, duration time.Duration) {
	t.Helper()
	end := start.Add(duration)
	if _, err := opened.Exec(`INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", topic, start, end, int(duration.Seconds()), start, start); err != nil {
		t.Fatalf("insert session failed: %v", err)
	}
}

func floatEquals(left, right, epsilon float64) bool {
	return math.Abs(left-right) <= epsilon
}
