package scheduler

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/events"
)

func TestGenerateDeterministicSnapshot(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC) // Monday
	to := time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)   // Saturday

	in := Input{
		From: from,
		To:   to,
		Constraints: ConstraintConfig{
			ActiveWeekdays:        []string{"mon", "wed", "fri"},
			DayStart:              "09:00",
			DayEnd:                "13:00",
			LunchStart:            "23:00",
			LunchDurationMinutes:  60,
			DinnerStart:           "23:30",
			DinnerDurationMinutes: 60,
			MaxHoursPerDay:        4,
			Timezone:              "UTC",
		},
		Targets: []WorkloadTarget{
			{
				ID:            1,
				Title:         "Math Work",
				Domain:        "Math",
				Subtopic:      "Algebra",
				Cadence:       "weekly",
				TargetSeconds: int((6 * time.Hour).Seconds()),
				Active:        true,
			},
		},
	}

	one, err := Generate(context.Background(), in)
	if err != nil {
		t.Fatalf("Generate first run failed: %v", err)
	}
	two, err := Generate(context.Background(), in)
	if err != nil {
		t.Fatalf("Generate second run failed: %v", err)
	}

	gotOne := snapshotEvents(one.Generated)
	gotTwo := snapshotEvents(two.Generated)
	if gotOne != gotTwo {
		t.Fatalf("expected deterministic output\nfirst:\n%s\nsecond:\n%s", gotOne, gotTwo)
	}

	want := "" +
		"2026-03-02T09:00:00Z -> 2026-03-02T10:00:00Z (Math::Algebra)\n" +
		"2026-03-02T10:00:00Z -> 2026-03-02T11:00:00Z (Math::Algebra)\n" +
		"2026-03-04T09:00:00Z -> 2026-03-04T10:00:00Z (Math::Algebra)\n" +
		"2026-03-04T10:00:00Z -> 2026-03-04T11:00:00Z (Math::Algebra)\n" +
		"2026-03-06T09:00:00Z -> 2026-03-06T10:00:00Z (Math::Algebra)\n" +
		"2026-03-06T10:00:00Z -> 2026-03-06T11:00:00Z (Math::Algebra)\n"
	if gotOne != want {
		t.Fatalf("unexpected scheduler snapshot\nwant:\n%s\ngot:\n%s", want, gotOne)
	}
}

func TestGenerateBalanceAcrossSelectedDays(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)
	result, err := Generate(context.Background(), Input{
		From: from,
		To:   to,
		Constraints: ConstraintConfig{
			ActiveWeekdays:        []string{"mon", "tue", "wed", "thu"},
			DayStart:              "09:00",
			DayEnd:                "17:00",
			LunchStart:            "23:00",
			LunchDurationMinutes:  60,
			DinnerStart:           "23:30",
			DinnerDurationMinutes: 60,
			MaxHoursPerDay:        8,
			Timezone:              "UTC",
		},
		Targets: []WorkloadTarget{
			{
				ID:            2,
				Title:         "Balance Target",
				Domain:        "Physics",
				Subtopic:      "Mechanics",
				Cadence:       "weekly",
				TargetSeconds: int((12 * time.Hour).Seconds()),
				Active:        true,
			},
		},
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	dayTotals := map[string]int{}
	for _, event := range result.Generated {
		day := event.StartTime.UTC().Format("2006-01-02")
		dayTotals[day] += int(event.EndTime.Sub(event.StartTime).Seconds())
	}
	if len(dayTotals) != 4 {
		t.Fatalf("expected distribution across 4 active days, got %d (%v)", len(dayTotals), dayTotals)
	}
	var mins, maxs int
	first := true
	for _, sec := range dayTotals {
		if first {
			mins = sec
			maxs = sec
			first = false
			continue
		}
		if sec < mins {
			mins = sec
		}
		if sec > maxs {
			maxs = sec
		}
	}
	if maxs-mins > int(time.Hour.Seconds()) {
		t.Fatalf("expected balanced distribution within 1h spread, got min=%d max=%d", mins, maxs)
	}
}

func TestGenerateTargetSatisfactionWithFixedEventDeductions(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
	result, err := Generate(context.Background(), Input{
		From: from,
		To:   to,
		Constraints: ConstraintConfig{
			ActiveWeekdays:        []string{"mon", "tue", "wed", "thu", "fri"},
			DayStart:              "09:00",
			DayEnd:                "18:00",
			LunchStart:            "23:00",
			LunchDurationMinutes:  60,
			DinnerStart:           "23:30",
			DinnerDurationMinutes: 60,
			MaxHoursPerDay:        8,
			Timezone:              "UTC",
		},
		Targets: []WorkloadTarget{
			{
				ID:            3,
				Title:         "Math Weekly",
				Domain:        "Math",
				Subtopic:      "Discrete",
				Cadence:       "weekly",
				TargetSeconds: int((8 * time.Hour).Seconds()),
				Active:        true,
			},
		},
		Existing: []ExistingEvent{
			{
				ID:        100,
				Title:     "Lecture",
				Kind:      "class",
				Domain:    "Math",
				Subtopic:  "Discrete",
				StartTime: time.Date(2026, 3, 3, 10, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC),
				Source:    "manual",
				Status:    "planned",
			},
		},
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if len(result.Summaries) != 1 {
		t.Fatalf("expected one summary, got %d", len(result.Summaries))
	}
	summary := result.Summaries[0]
	if summary.RequiredSeconds != int((8 * time.Hour).Seconds()) {
		t.Fatalf("unexpected required seconds: %d", summary.RequiredSeconds)
	}
	if summary.FixedDeductionSeconds != int((2 * time.Hour).Seconds()) {
		t.Fatalf("unexpected fixed deduction seconds: %d", summary.FixedDeductionSeconds)
	}
	if summary.GeneratedSeconds != int((6 * time.Hour).Seconds()) {
		t.Fatalf("unexpected generated seconds: %d", summary.GeneratedSeconds)
	}
	if summary.RemainingSeconds != 0 {
		t.Fatalf("expected remaining=0, got %d", summary.RemainingSeconds)
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %+v", result.Diagnostics)
	}
}

func TestGenerateImpossiblePlanDiagnostics(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	result, err := Generate(context.Background(), Input{
		From: from,
		To:   to,
		Constraints: ConstraintConfig{
			ActiveWeekdays:        []string{"mon"},
			DayStart:              "09:00",
			DayEnd:                "10:00",
			LunchStart:            "23:00",
			LunchDurationMinutes:  60,
			DinnerStart:           "23:30",
			DinnerDurationMinutes: 60,
			MaxHoursPerDay:        1,
			Timezone:              "UTC",
		},
		Targets: []WorkloadTarget{
			{
				ID:            4,
				Title:         "Impossible",
				Domain:        "Chemistry",
				Subtopic:      "Organic",
				Cadence:       "daily",
				TargetSeconds: int((4 * time.Hour).Seconds()),
				Active:        true,
			},
		},
		Existing: []ExistingEvent{
			{
				ID:        300,
				Title:     "Locked Meeting",
				Kind:      "task",
				Domain:    "Admin",
				Subtopic:  "General",
				StartTime: time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2026, 3, 2, 9, 30, 0, 0, time.UTC),
				Source:    "manual",
				Status:    "planned",
			},
		},
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if len(result.Diagnostics) == 0 {
		t.Fatalf("expected diagnostics for impossible plan")
	}
	if result.Diagnostics[0].Code != "insufficient_capacity" {
		t.Fatalf("unexpected diagnostic code: %s", result.Diagnostics[0].Code)
	}
	if result.Diagnostics[0].MissingSeconds <= 0 {
		t.Fatalf("expected positive missing seconds, got %d", result.Diagnostics[0].MissingSeconds)
	}
}

func TestGenerateFromDBPersistsScheduleRunAndEvents(t *testing.T) {
	opened, err := db.Open(filepath.Join(t.TempDir(), "scheduler.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer opened.Close()

	prev := db.DB
	db.DB = opened
	t.Cleanup(func() { db.DB = prev })

	if err := SaveConstraintConfig(context.Background(), ConstraintConfig{
		ActiveWeekdays:        []string{"mon", "tue", "wed", "thu", "fri"},
		DayStart:              "09:00",
		DayEnd:                "17:00",
		LunchStart:            "23:00",
		LunchDurationMinutes:  60,
		DinnerStart:           "23:30",
		DinnerDurationMinutes: 60,
		MaxHoursPerDay:        8,
		Timezone:              "UTC",
	}); err != nil {
		t.Fatalf("SaveConstraintConfig failed: %v", err)
	}

	targetID, err := CreateWorkloadTarget(context.Background(), WorkloadTarget{
		Title:         "Math Target",
		Domain:        "Math",
		Subtopic:      "Analysis",
		Cadence:       "weekly",
		TargetSeconds: int((2 * time.Hour).Seconds()),
		Active:        true,
	})
	if err != nil {
		t.Fatalf("CreateWorkloadTarget failed: %v", err)
	}

	if _, err := opened.Exec(`
		INSERT INTO events(kind, title, domain, subtopic, description, start_time, end_time, duration, timezone, layer, status, source, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"class",
		"Lecture",
		"Math",
		"Analysis",
		"",
		"2026-03-03 10:00:00",
		"2026-03-03 11:00:00",
		3600,
		"UTC",
		"planned",
		"planned",
		"manual",
	); err != nil {
		t.Fatalf("insert fixed lecture failed: %v", err)
	}

	result, err := GenerateFromDB(context.Background(), DBInput{
		From:                     time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
		To:                       time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC),
		Persist:                  true,
		ReplaceExistingScheduler: true,
	})
	if err != nil {
		t.Fatalf("GenerateFromDB failed: %v", err)
	}
	if result.RunID == 0 {
		t.Fatalf("expected non-zero schedule run id")
	}

	var status string
	if err := opened.QueryRow(`SELECT status FROM schedule_runs WHERE id = ?`, result.RunID).Scan(&status); err != nil {
		t.Fatalf("query schedule_runs status failed: %v", err)
	}
	if status != "success" {
		t.Fatalf("expected successful run status, got %s", status)
	}

	var generatedCount int
	if err := opened.QueryRow(`
		SELECT COUNT(1)
		FROM events
		WHERE source = 'scheduler'
		  AND workload_target_id = ?`, targetID).Scan(&generatedCount); err != nil {
		t.Fatalf("count generated scheduler events failed: %v", err)
	}
	if generatedCount == 0 {
		t.Fatalf("expected generated scheduler events")
	}

	var generatedSeconds int
	if err := opened.QueryRow(`
		SELECT COALESCE(SUM(duration), 0)
		FROM events
		WHERE source = 'scheduler'
		  AND workload_target_id = ?`, targetID).Scan(&generatedSeconds); err != nil {
		t.Fatalf("sum generated scheduler duration failed: %v", err)
	}
	if generatedSeconds != int((1 * time.Hour).Seconds()) {
		t.Fatalf("expected 1h generated after 1h fixed deduction, got %d seconds", generatedSeconds)
	}
}

func TestGenerateFromDBEnforcesDependencyConstraints(t *testing.T) {
	opened, err := db.Open(filepath.Join(t.TempDir(), "scheduler-deps.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer opened.Close()

	prev := db.DB
	db.DB = opened
	t.Cleanup(func() { db.DB = prev })

	prereqStart := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	prereqID, err := events.Create(context.Background(), events.Event{
		Kind:      "class",
		Title:     "Lecture",
		Domain:    "Math",
		Subtopic:  "Algebra",
		StartTime: prereqStart,
		EndTime:   prereqStart.Add(time.Hour),
		Layer:     "planned",
		Status:    "planned",
		Source:    "manual",
	})
	if err != nil {
		t.Fatalf("create prerequisite event failed: %v", err)
	}

	dependentStart := time.Date(2026, 3, 10, 11, 0, 0, 0, time.UTC)
	dependentID, err := events.Create(context.Background(), events.Event{
		Kind:      "task",
		Title:     "Tutorial",
		Domain:    "Math",
		Subtopic:  "Algebra",
		StartTime: dependentStart,
		EndTime:   dependentStart.Add(time.Hour),
		Layer:     "planned",
		Status:    "planned",
		Source:    "scheduler",
	})
	if err != nil {
		t.Fatalf("create dependent event failed: %v", err)
	}

	if err := events.AddDependency(context.Background(), dependentID, prereqID, true); err != nil {
		t.Fatalf("add dependency failed: %v", err)
	}

	// Force a stale state so scheduler reconciliation must restore blocking.
	if _, err := opened.Exec(`
		UPDATE events
		SET status = 'planned',
		    blocked_reason = NULL,
		    blocked_override = 0
		WHERE id = ?`, dependentID); err != nil {
		t.Fatalf("set stale dependent state failed: %v", err)
	}

	firstRun, err := GenerateFromDB(context.Background(), DBInput{
		From:                     time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
		To:                       time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC),
		Persist:                  true,
		ReplaceExistingScheduler: false,
	})
	if err != nil {
		t.Fatalf("GenerateFromDB first run failed: %v", err)
	}
	if firstRun.RunID == 0 {
		t.Fatalf("expected schedule run id for first run")
	}

	var blockedStatus string
	var blockedReason string
	if err := opened.QueryRow(`SELECT status, COALESCE(blocked_reason, '') FROM events WHERE id = ?`, dependentID).Scan(&blockedStatus, &blockedReason); err != nil {
		t.Fatalf("query blocked dependent failed: %v", err)
	}
	if blockedStatus != "blocked" {
		t.Fatalf("expected dependent status=blocked after scheduler reconciliation, got %s", blockedStatus)
	}
	if strings.TrimSpace(blockedReason) == "" {
		t.Fatalf("expected blocked reason after scheduler reconciliation")
	}

	var blockActionCount int
	if err := opened.QueryRow(`
		SELECT COUNT(1)
		FROM schedule_run_events
		WHERE run_id = ?
		  AND event_id = ?
		  AND action = 'block'`, firstRun.RunID, dependentID).Scan(&blockActionCount); err != nil {
		t.Fatalf("count block schedule_run_events failed: %v", err)
	}
	if blockActionCount == 0 {
		t.Fatalf("expected block action in schedule_run_events for first run")
	}

	if _, err := opened.Exec(`UPDATE events SET status = 'done', blocked_reason = NULL WHERE id = ?`, prereqID); err != nil {
		t.Fatalf("mark prerequisite done failed: %v", err)
	}
	if _, err := opened.Exec(`UPDATE events SET status = 'blocked' WHERE id = ?`, dependentID); err != nil {
		t.Fatalf("set dependent blocked before second run failed: %v", err)
	}

	secondRun, err := GenerateFromDB(context.Background(), DBInput{
		From:                     time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
		To:                       time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC),
		Persist:                  true,
		ReplaceExistingScheduler: false,
	})
	if err != nil {
		t.Fatalf("GenerateFromDB second run failed: %v", err)
	}
	if secondRun.RunID == 0 {
		t.Fatalf("expected schedule run id for second run")
	}

	var unblockedStatus string
	var unblockedReason string
	if err := opened.QueryRow(`SELECT status, COALESCE(blocked_reason, '') FROM events WHERE id = ?`, dependentID).Scan(&unblockedStatus, &unblockedReason); err != nil {
		t.Fatalf("query unblocked dependent failed: %v", err)
	}
	if unblockedStatus != "planned" {
		t.Fatalf("expected dependent status=planned after prerequisite completion, got %s", unblockedStatus)
	}
	if strings.TrimSpace(unblockedReason) != "" {
		t.Fatalf("expected blocked reason to clear after unblocking, got %q", unblockedReason)
	}
}

func snapshotEvents(events []PlannedEvent) string {
	ordered := append([]PlannedEvent(nil), events...)
	sort.Slice(ordered, func(i, j int) bool {
		if !ordered[i].StartTime.Equal(ordered[j].StartTime) {
			return ordered[i].StartTime.Before(ordered[j].StartTime)
		}
		if !ordered[i].EndTime.Equal(ordered[j].EndTime) {
			return ordered[i].EndTime.Before(ordered[j].EndTime)
		}
		return ordered[i].Title < ordered[j].Title
	})
	var b strings.Builder
	for _, event := range ordered {
		b.WriteString(event.StartTime.UTC().Format(time.RFC3339))
		b.WriteString(" -> ")
		b.WriteString(event.EndTime.UTC().Format(time.RFC3339))
		b.WriteString(" (")
		b.WriteString(event.Domain)
		b.WriteString("::")
		b.WriteString(event.Subtopic)
		b.WriteString(")\n")
	}
	return b.String()
}
