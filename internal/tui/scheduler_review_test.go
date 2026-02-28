package tui

import (
	"context"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/scheduler"
	tea "github.com/charmbracelet/bubbletea"
)

type fakeSchedulerReviewService struct {
	lastInput scheduler.DBInput
	result    scheduler.Result
}

func (f *fakeSchedulerReviewService) GenerateFromDB(ctx context.Context, in scheduler.DBInput) (scheduler.Result, error) {
	f.lastInput = in
	return f.result, nil
}

func TestSchedulerReviewModelStateTransitions(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	service := &fakeSchedulerReviewService{
		result: scheduler.Result{
			Generated: []scheduler.PlannedEvent{{Title: "A"}},
		},
	}
	m := newSchedulerReviewModel(service, func() time.Time { return now })

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	state := updated.(schedulerReviewModel)
	if state.form.Cursor != 1 {
		t.Fatalf("expected cursor=1 after down, got %d", state.form.Cursor)
	}

	updated, cmd := state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatalf("expected command on review key")
	}
	msg := cmd().(schedulerRunMsg)
	if msg.Err != nil {
		t.Fatalf("unexpected scheduler run error: %v", msg.Err)
	}
	state = updated.(schedulerReviewModel)
	updated, _ = state.Update(msg)
	state = updated.(schedulerReviewModel)
	if state.lastRun == nil || state.lastRun.Mode != "review" {
		t.Fatalf("expected last run mode review")
	}
	if service.lastInput.Persist {
		t.Fatalf("review mode should not persist")
	}

	updated, cmd = state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatalf("expected command on apply key")
	}
	msg = cmd().(schedulerRunMsg)
	if msg.Err != nil {
		t.Fatalf("unexpected scheduler run error: %v", msg.Err)
	}
	state = updated.(schedulerReviewModel)
	updated, _ = state.Update(msg)
	state = updated.(schedulerReviewModel)
	if state.lastRun == nil || state.lastRun.Mode != "apply" {
		t.Fatalf("expected last run mode apply")
	}
	if !service.lastInput.Persist {
		t.Fatalf("apply mode should persist")
	}
}

func TestSchedulerReviewKeyboardAccessibility(t *testing.T) {
	service := &fakeSchedulerReviewService{}
	m := newSchedulerReviewModel(service, time.Now)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	state := updated.(schedulerReviewModel)
	if state.form.Cursor != 1 {
		t.Fatalf("expected down to move cursor to 1, got %d", state.form.Cursor)
	}

	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyUp})
	state = updated.(schedulerReviewModel)
	if state.form.Cursor != 0 {
		t.Fatalf("expected up to move cursor to 0, got %d", state.form.Cursor)
	}

	updated, quitCmd := state.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if quitCmd == nil {
		t.Fatalf("expected quit command on ctrl+c")
	}
	_ = updated
}
