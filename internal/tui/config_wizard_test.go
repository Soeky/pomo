package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

type fakeConfigWizardService struct {
	loadValues configWizardValues
	saved      configWizardValues
	saveCalled bool
}

func (f *fakeConfigWizardService) Load(ctx context.Context) (configWizardValues, error) {
	return f.loadValues, nil
}

func (f *fakeConfigWizardService) Save(ctx context.Context, values configWizardValues) error {
	f.saveCalled = true
	f.saved = values
	return nil
}

func TestConfigWizardModelLoadAndSave(t *testing.T) {
	service := &fakeConfigWizardService{
		loadValues: configWizardValues{
			ActiveWeekdays:        []string{"mon", "wed", "fri"},
			DayStart:              "08:00",
			DayEnd:                "22:00",
			LunchStart:            "12:30",
			LunchDurationMinutes:  60,
			DinnerStart:           "19:00",
			DinnerDurationMinutes: 60,
			MaxHoursPerDay:        8,
			Timezone:              "Local",
			BreakThresholdMinutes: 10,
		},
	}
	m := newConfigWizardModel(service)

	loadedMsg := m.Init()().(configWizardLoadedMsg)
	updated, _ := m.Update(loadedMsg)
	state := updated.(configWizardModel)
	if !state.loaded {
		t.Fatalf("expected wizard to be loaded")
	}
	if got := state.form.Fields[0].Value; got != "mon,wed,fri" {
		t.Fatalf("unexpected loaded weekday value: %s", got)
	}

	state.form.Fields[0].Value = "mon,tue,thu"
	state.form.Fields[9].Value = "15"

	updated, cmd := state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatalf("expected save command")
	}
	savedMsg := cmd().(configWizardSavedMsg)
	if savedMsg.Err != nil {
		t.Fatalf("unexpected save error: %v", savedMsg.Err)
	}
	state = updated.(configWizardModel)
	updated, _ = state.Update(savedMsg)
	state = updated.(configWizardModel)
	if !service.saveCalled {
		t.Fatalf("expected save to be called")
	}
	if service.saved.BreakThresholdMinutes != 15 {
		t.Fatalf("expected break threshold 15, got %d", service.saved.BreakThresholdMinutes)
	}
	if len(service.saved.ActiveWeekdays) != 3 || service.saved.ActiveWeekdays[1] != "tue" {
		t.Fatalf("unexpected saved weekdays: %#v", service.saved.ActiveWeekdays)
	}
}

func TestConfigWizardKeyboardAccessibility(t *testing.T) {
	service := &fakeConfigWizardService{
		loadValues: configWizardValues{
			ActiveWeekdays:        []string{"mon", "tue"},
			DayStart:              "08:00",
			DayEnd:                "22:00",
			LunchStart:            "12:30",
			LunchDurationMinutes:  60,
			DinnerStart:           "19:00",
			DinnerDurationMinutes: 60,
			MaxHoursPerDay:        8,
			Timezone:              "Local",
			BreakThresholdMinutes: 10,
		},
	}
	m := newConfigWizardModel(service)
	loadedMsg := m.Init()().(configWizardLoadedMsg)
	updated, _ := m.Update(loadedMsg)
	state := updated.(configWizardModel)

	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyDown})
	state = updated.(configWizardModel)
	if state.form.Cursor != 1 {
		t.Fatalf("expected down to move cursor to 1, got %d", state.form.Cursor)
	}

	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyUp})
	state = updated.(configWizardModel)
	if state.form.Cursor != 0 {
		t.Fatalf("expected up to move cursor to 0, got %d", state.form.Cursor)
	}

	_, quitCmd := state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if quitCmd == nil {
		t.Fatalf("expected quit command on q")
	}
}
