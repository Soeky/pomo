package tui

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/events"
	"github.com/Soeky/pomo/internal/scheduler"
	tea "github.com/charmbracelet/bubbletea"
)

type quitModel struct{}

func (quitModel) Init() tea.Cmd                             { return tea.Quit }
func (m quitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (quitModel) View() string                              { return "" }

func TestNoRendererFromEnvAndRunProgram(t *testing.T) {
	t.Setenv("POMO_TUI_NO_RENDER", "1")
	if !NoRendererFromEnv() {
		t.Fatalf("expected NoRendererFromEnv true when env=1")
	}
	t.Setenv("POMO_TUI_NO_RENDER", "0")
	if NoRendererFromEnv() {
		t.Fatalf("expected NoRendererFromEnv false when env=0")
	}

	if err := runProgram(quitModel{}, RunOptions{Input: bytes.NewBuffer(nil), Output: io.Discard, NoRenderer: true}); err != nil {
		t.Fatalf("runProgram failed: %v", err)
	}
}

func TestDefaultServicesAndRunWrappers(t *testing.T) {
	opened := openTUITestDB(t)
	defer opened.Close()

	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)
	config.LoadConfig()

	cfgService := defaultConfigWizardService{}
	if err := scheduler.SaveConstraintConfig(context.Background(), scheduler.DefaultConstraintConfig()); err != nil {
		t.Fatalf("seed scheduler config failed: %v", err)
	}
	loaded, err := cfgService.Load(context.Background())
	if err != nil {
		t.Fatalf("config wizard load failed: %v", err)
	}
	if len(loaded.ActiveWeekdays) == 0 {
		t.Fatalf("expected non-empty active weekdays from load")
	}
	if err := cfgService.Save(context.Background(), configWizardValues{
		ActiveWeekdays:        []string{"mon", "tue", "wed"},
		DayStart:              "08:00",
		DayEnd:                "22:00",
		LunchStart:            "12:30",
		LunchDurationMinutes:  60,
		DinnerStart:           "19:00",
		DinnerDurationMinutes: 60,
		MaxHoursPerDay:        8,
		Timezone:              "Local",
		BreakThresholdMinutes: 10,
	}); err != nil {
		t.Fatalf("config wizard save failed: %v", err)
	}

	schedulerService := defaultSchedulerReviewService{}
	if _, err := schedulerService.GenerateFromDB(context.Background(), scheduler.DBInput{
		From:    time.Now().Add(-time.Hour),
		To:      time.Now().Add(time.Hour),
		Persist: false,
	}); err != nil {
		t.Fatalf("scheduler review default service failed: %v", err)
	}

	eventService := defaultEventManagerService{}
	start := time.Now().Add(30 * time.Minute).UTC().Truncate(time.Second)
	end := start.Add(45 * time.Minute)
	firstID, err := eventService.CreateEvent(context.Background(), events.Event{
		Kind:      "task",
		Title:     "Coverage::First",
		Domain:    "Coverage",
		Subtopic:  "First",
		StartTime: start,
		EndTime:   end,
		Layer:     "planned",
		Status:    "planned",
		Source:    "manual",
	})
	if err != nil {
		t.Fatalf("create canonical event failed: %v", err)
	}
	secondID, err := eventService.CreateEvent(context.Background(), events.Event{
		Kind:      "task",
		Title:     "Coverage::Second",
		Domain:    "Coverage",
		Subtopic:  "Second",
		StartTime: end.Add(time.Minute),
		EndTime:   end.Add(31 * time.Minute),
		Layer:     "planned",
		Status:    "planned",
		Source:    "manual",
	})
	if err != nil {
		t.Fatalf("create second canonical event failed: %v", err)
	}

	if _, err := eventService.ListCanonicalInRange(context.Background(), start.Add(-time.Hour), end.Add(2*time.Hour)); err != nil {
		t.Fatalf("list canonical events failed: %v", err)
	}
	row, err := eventService.GetEventByID(context.Background(), firstID)
	if err != nil {
		t.Fatalf("get event by id failed: %v", err)
	}
	row.Title = "Coverage::FirstUpdated"
	if err := eventService.UpdateEvent(context.Background(), firstID, row); err != nil {
		t.Fatalf("update event failed: %v", err)
	}

	rr, err := events.BuildRRule(events.RecurrenceSpec{Freq: "weekly", Interval: 1})
	if err != nil {
		t.Fatalf("build rrule failed: %v", err)
	}
	ruleID, err := eventService.CreateRecurrenceRule(context.Background(), events.RecurrenceRule{
		Title:       "Coverage Rule",
		Domain:      "Coverage",
		Subtopic:    "General",
		Kind:        "task",
		DurationSec: 1800,
		RRule:       rr,
		Timezone:    "UTC",
		StartDate:   start,
		Active:      true,
	})
	if err != nil {
		t.Fatalf("create recurrence rule failed: %v", err)
	}
	if _, err := eventService.ListRecurrenceRules(context.Background(), false); err != nil {
		t.Fatalf("list recurrence rules failed: %v", err)
	}
	rule, err := eventService.GetRecurrenceRuleByID(context.Background(), ruleID)
	if err != nil {
		t.Fatalf("get recurrence rule failed: %v", err)
	}
	rule.Title = "Coverage Rule Updated"
	if err := eventService.UpdateRecurrenceRule(context.Background(), ruleID, rule); err != nil {
		t.Fatalf("update recurrence rule failed: %v", err)
	}

	if err := eventService.AddDependency(context.Background(), secondID, firstID, true); err != nil {
		t.Fatalf("add dependency failed: %v", err)
	}
	if _, err := eventService.ListDependencies(context.Background(), secondID); err != nil {
		t.Fatalf("list dependencies failed: %v", err)
	}
	if err := eventService.SetDependencyOverride(context.Background(), secondID, true, true, "coverage", "test"); err != nil {
		t.Fatalf("set dependency override failed: %v", err)
	}
	if err := eventService.DeleteDependency(context.Background(), secondID, firstID); err != nil {
		t.Fatalf("delete dependency failed: %v", err)
	}

	if err := eventService.DeleteRecurrenceRule(context.Background(), ruleID); err != nil {
		t.Fatalf("delete recurrence rule failed: %v", err)
	}
	if err := eventService.DeleteEvent(context.Background(), secondID); err != nil {
		t.Fatalf("delete second event failed: %v", err)
	}
	if err := eventService.DeleteEvent(context.Background(), firstID); err != nil {
		t.Fatalf("delete first event failed: %v", err)
	}

	if err := RunConfigWizard(RunOptions{Input: bytes.NewBufferString("q"), Output: io.Discard, NoRenderer: true}); err != nil {
		t.Fatalf("RunConfigWizard failed: %v", err)
	}
	if err := RunSchedulerReview(RunOptions{Input: bytes.NewBufferString("q"), Output: io.Discard, NoRenderer: true}); err != nil {
		t.Fatalf("RunSchedulerReview failed: %v", err)
	}
	if err := RunEventManager(RunOptions{Input: bytes.NewBufferString("q"), Output: io.Discard, NoRenderer: true}); err != nil {
		t.Fatalf("RunEventManager failed: %v", err)
	}

	if _, ok := NewConfigWizardModel().(tea.Model); !ok {
		t.Fatalf("expected NewConfigWizardModel to return tea.Model")
	}
	if _, ok := NewSchedulerReviewModel().(tea.Model); !ok {
		t.Fatalf("expected NewSchedulerReviewModel to return tea.Model")
	}
	if _, ok := NewEventManagerModel().(tea.Model); !ok {
		t.Fatalf("expected NewEventManagerModel to return tea.Model")
	}
}

func openTUITestDB(t *testing.T) *sql.DB {
	t.Helper()
	opened, err := db.Open(filepath.Join(t.TempDir(), "pomo.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	db.DB = opened
	return opened
}
