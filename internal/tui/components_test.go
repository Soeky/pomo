package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMenuMoveWraps(t *testing.T) {
	m := menuModel{
		Items: []menuItem{
			{Label: "one"},
			{Label: "two"},
			{Label: "three"},
		},
	}

	m.move(1)
	if m.Cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", m.Cursor)
	}
	m.move(10)
	if m.Cursor != 0 {
		t.Fatalf("expected wrapped cursor 0, got %d", m.Cursor)
	}
	m.move(-1)
	if m.Cursor != 2 {
		t.Fatalf("expected wrapped cursor 2, got %d", m.Cursor)
	}
}

func TestFormUpdateNavigationAndSubmit(t *testing.T) {
	f := formModel{
		Fields: []formField{
			{Key: "a", Label: "A"},
			{Key: "b", Label: "B"},
		},
	}

	submitted, canceled := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if submitted || canceled {
		t.Fatalf("did not expect submit/cancel on rune input")
	}
	if got := f.Fields[0].Value; got != "x" {
		t.Fatalf("expected first field value x, got %q", got)
	}

	submitted, canceled = f.Update(tea.KeyMsg{Type: tea.KeyTab})
	if submitted || canceled {
		t.Fatalf("did not expect submit/cancel on tab")
	}
	if f.Cursor != 1 {
		t.Fatalf("expected cursor 1 after tab, got %d", f.Cursor)
	}

	submitted, canceled = f.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !submitted || canceled {
		t.Fatalf("expected submit on enter at last field")
	}
}

func TestFormUpdateSupportsKeyboardAccessibilityKeys(t *testing.T) {
	f := formModel{
		Fields: []formField{
			{Key: "a", Label: "A", Value: "ab"},
			{Key: "b", Label: "B"},
			{Key: "c", Label: "C"},
		},
		Cursor: 1,
	}

	_, _ = f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	if f.Fields[1].Value != "z" {
		t.Fatalf("expected typed rune on focused field")
	}

	_, _ = f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if f.Fields[1].Value != "zk" {
		t.Fatalf("expected k rune to append text, got %q", f.Fields[1].Value)
	}

	_, _ = f.Update(tea.KeyMsg{Type: tea.KeyUp})
	if f.Cursor != 0 {
		t.Fatalf("expected up to move cursor to 0, got %d", f.Cursor)
	}

	_, _ = f.Update(tea.KeyMsg{Type: tea.KeyDown})
	if f.Cursor != 1 {
		t.Fatalf("expected down to move cursor to 1, got %d", f.Cursor)
	}

	_, _ = f.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if f.Cursor != 0 {
		t.Fatalf("expected shift+tab to move cursor to 0, got %d", f.Cursor)
	}

	_, _ = f.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if got := f.Fields[0].Value; got != "a" {
		t.Fatalf("expected backspace to trim rune, got %q", got)
	}

	submitted, canceled := f.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if submitted || !canceled {
		t.Fatalf("expected esc to cancel")
	}
}

func TestIsQuitKey(t *testing.T) {
	if !isQuitKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}) {
		t.Fatalf("expected q to quit")
	}
	if !isQuitKey(tea.KeyMsg{Type: tea.KeyCtrlC}) {
		t.Fatalf("expected ctrl+c to quit")
	}
	if isQuitKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}) {
		t.Fatalf("did not expect x to quit")
	}
}
