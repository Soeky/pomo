package delete

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Soeky/pomo/internal/db"
)

func TestFormatDuration(t *testing.T) {
	if got := formatDuration(0); got != "00:00" {
		t.Fatalf("unexpected duration format: %s", got)
	}
	if got := formatDuration(int((65 * time.Minute).Seconds())); got != "01:05" {
		t.Fatalf("unexpected duration format: %s", got)
	}
}

func TestGetRecentSessionsAndDeleteSessionByID(t *testing.T) {
	opened := openDeleteTestDB(t)
	defer opened.Close()

	start := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	id, err := db.InsertSessionAt("break", "", start, 25*time.Minute)
	if err != nil {
		t.Fatalf("insert tracked break event failed: %v", err)
	}

	items, err := getRecentSessions(10)
	if err != nil {
		t.Fatalf("getRecentSessions failed: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("expected at least one recent item")
	}
	if items[0].Topic != "break" {
		t.Fatalf("expected break topic fallback, got %q", items[0].Topic)
	}

	if err := deleteSessionByID(int(id)); err != nil {
		t.Fatalf("deleteSessionByID failed: %v", err)
	}
}

func TestModelQuitAndCountInput(t *testing.T) {
	opened := openDeleteTestDB(t)
	defer opened.Close()
	now := time.Now().UTC()
	if _, err := db.InsertSessionAt("focus", "A", now.Add(-time.Hour), 30*time.Minute); err != nil {
		t.Fatalf("seed tracked focus event failed: %v", err)
	}

	m := NewModel()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m2 := updated.(model)
	if m2.countInput != "1" {
		t.Fatalf("expected count input to be updated, got %q", m2.countInput)
	}
	if cmd != nil {
		t.Fatalf("did not expect command for numeric input")
	}

	_, quitCmd := m2.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if quitCmd == nil {
		t.Fatalf("expected quit command on ctrl+c")
	}

	// Count selection + enter should transition to item selection step.
	m3, _ := NewModel().Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m4, _ := m3.(model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m4.(model).step != stepSelectItems {
		t.Fatalf("expected stepSelectItems, got %d", m4.(model).step)
	}

	// Select first item, confirm, then go back with 'n'.
	m5, _ := m4.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m6, _ := m5.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if m6.(model).step != stepConfirm {
		t.Fatalf("expected stepConfirm, got %d", m6.(model).step)
	}
	m7, _ := m6.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m7.(model).step != stepSelectItems {
		t.Fatalf("expected stepSelectItems after decline, got %d", m7.(model).step)
	}

	// Confirm deletion branch
	m8, _ := m6.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if m8.(model).step != stepConfirm {
		t.Fatalf("model state should remain stepConfirm until quit, got %d", m8.(model).step)
	}

	// View coverage for all steps
	mv := NewModel()
	if v := mv.View(); v == "" {
		t.Fatalf("expected non-empty view for count step")
	}
	mv.step = stepSelectItems
	mv.items = []item{{ID: 1, Topic: "A", StartTime: now.Add(-time.Hour), EndTime: now, Duration: 1200}}
	if v := mv.View(); v == "" {
		t.Fatalf("expected non-empty view for select step")
	}
	mv.step = stepConfirm
	mv.selected[0] = struct{}{}
	if v := mv.View(); v == "" {
		t.Fatalf("expected non-empty view for confirm step")
	}
}

func openDeleteTestDB(t *testing.T) *sql.DB {
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
