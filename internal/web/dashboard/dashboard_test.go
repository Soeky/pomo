package dashboard

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
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
	if len(defs) != 3 {
		t.Fatalf("expected 3 dashboard modules, got %d", len(defs))
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

func seedDashboardData(opened *sql.DB, start, end time.Time) error {
	if _, err := opened.Exec(`INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "A", start, end, int((25 * time.Minute).Seconds()), start, start); err != nil {
		return err
	}
	if _, err := opened.Exec(`INSERT INTO planned_events(title, description, start_time, end_time, status, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"P1", "d", start, end, "done", "manual", start, start); err != nil {
		return err
	}
	return nil
}
