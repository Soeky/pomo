package stats

import (
	"database/sql"
	"reflect"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/config"
)

func TestComputeEffectiveFocusTotalsThresholdEdges(t *testing.T) {
	t.Parallel()

	sessions := []EffectiveSession{
		{Type: "focus", Topic: "Math::A", DurationSeconds: int((25 * time.Minute).Seconds())},
		{Type: "break", Topic: "", DurationSeconds: int((10 * time.Minute).Seconds())},
		{Type: "focus", Topic: "Math::B", DurationSeconds: int((25 * time.Minute).Seconds())},
		{Type: "break", Topic: "", DurationSeconds: int((10 * time.Minute).Seconds()) + 1},
		{Type: "focus", Topic: "Math::C", DurationSeconds: int((25 * time.Minute).Seconds())},
	}

	got := ComputeEffectiveFocusTotals(sessions, 10)
	if got.RawFocusMinutes != 75 {
		t.Fatalf("unexpected raw focus minutes: %d", got.RawFocusMinutes)
	}
	if got.CreditedBreakMinutes != 10 {
		t.Fatalf("unexpected credited break minutes: %d", got.CreditedBreakMinutes)
	}
	if got.EffectiveFocusMinutes != 85 {
		t.Fatalf("unexpected effective focus minutes: %d", got.EffectiveFocusMinutes)
	}
}

func TestComputeEffectiveFocusTotalsRequiresSameDomainNeighbors(t *testing.T) {
	t.Parallel()

	sessions := []EffectiveSession{
		{Type: "focus", Topic: "Math::A", DurationSeconds: int((25 * time.Minute).Seconds())},
		{Type: "break", Topic: "", DurationSeconds: int((5 * time.Minute).Seconds())},
		{Type: "focus", Topic: "Physics::A", DurationSeconds: int((25 * time.Minute).Seconds())},
	}

	got := ComputeEffectiveFocusTotals(sessions, 10)
	if got.RawFocusMinutes != 50 {
		t.Fatalf("unexpected raw focus minutes: %d", got.RawFocusMinutes)
	}
	if got.CreditedBreakMinutes != 0 {
		t.Fatalf("unexpected credited break minutes: %d", got.CreditedBreakMinutes)
	}
	if got.EffectiveFocusMinutes != 50 {
		t.Fatalf("unexpected effective focus minutes: %d", got.EffectiveFocusMinutes)
	}
}

func TestEffectiveMetricsAreDerivedOnlyAndDoNotMutateRows(t *testing.T) {
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
	mustExecStats(t, opened, `INSERT INTO events(kind, title, domain, subtopic, start_time, end_time, duration, layer, status, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"task", "Manual Event", "Planning", "General", base, base.Add(time.Hour), int(time.Hour.Seconds()), "planned", "planned", "manual", base, base)

	sessionsBefore := snapshotRows(t, opened, `SELECT printf('%d|%s|%s|%d|%s|%s|%s|%s', id, type, COALESCE(topic, ''), COALESCE(duration, 0), start_time, COALESCE(end_time, ''), created_at, updated_at) FROM sessions ORDER BY id`)
	eventsBefore := snapshotRows(t, opened, `SELECT printf('%d|%s|%s|%s|%s|%d|%s|%s|%s|%s|%d|%s', id, kind, title, domain, subtopic, COALESCE(duration, 0), layer, status, source, COALESCE(legacy_source, ''), COALESCE(legacy_id, 0), updated_at) FROM events ORDER BY id`)

	totals, err := QueryEffectiveFocusTotals(base.Add(-time.Minute), base.Add(3*time.Hour), 10)
	if err != nil {
		t.Fatalf("QueryEffectiveFocusTotals failed: %v", err)
	}
	if totals.EffectiveFocusMinutes != 60 {
		t.Fatalf("unexpected effective focus total: %d", totals.EffectiveFocusMinutes)
	}
	if _, err := BuildReport([]string{"2026-02-25"}, base.Add(2*time.Hour)); err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}

	sessionsAfter := snapshotRows(t, opened, `SELECT printf('%d|%s|%s|%d|%s|%s|%s|%s', id, type, COALESCE(topic, ''), COALESCE(duration, 0), start_time, COALESCE(end_time, ''), created_at, updated_at) FROM sessions ORDER BY id`)
	eventsAfter := snapshotRows(t, opened, `SELECT printf('%d|%s|%s|%s|%s|%d|%s|%s|%s|%s|%d|%s', id, kind, title, domain, subtopic, COALESCE(duration, 0), layer, status, source, COALESCE(legacy_source, ''), COALESCE(legacy_id, 0), updated_at) FROM events ORDER BY id`)

	if !reflect.DeepEqual(sessionsBefore, sessionsAfter) {
		t.Fatalf("sessions rows mutated by derived metrics query")
	}
	if !reflect.DeepEqual(eventsBefore, eventsAfter) {
		t.Fatalf("events rows mutated by derived metrics query")
	}
}

func snapshotRows(t *testing.T, opened *sql.DB, query string) []string {
	t.Helper()

	rows, err := opened.Query(query)
	if err != nil {
		t.Fatalf("snapshot query failed: %v", err)
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			t.Fatalf("snapshot scan failed: %v", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("snapshot rows failed: %v", err)
	}
	return out
}
