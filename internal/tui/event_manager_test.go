package tui

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/events"
	tea "github.com/charmbracelet/bubbletea"
)

type fakeEventManagerService struct {
	rows  []events.Event
	rules []events.RecurrenceRule
	deps  []events.Dependency

	createdEvents []events.Event
	updatedEvents map[int64]events.Event
	deletedEvents []int64
}

func (f *fakeEventManagerService) ListCanonicalInRange(ctx context.Context, from, to time.Time) ([]events.Event, error) {
	return append([]events.Event(nil), f.rows...), nil
}

func (f *fakeEventManagerService) CreateEvent(ctx context.Context, event events.Event) (int64, error) {
	f.createdEvents = append(f.createdEvents, event)
	return 99, nil
}

func (f *fakeEventManagerService) GetEventByID(ctx context.Context, id int64) (events.Event, error) {
	for _, row := range f.rows {
		if row.ID == id {
			return row, nil
		}
	}
	return events.Event{}, fmt.Errorf("not found")
}

func (f *fakeEventManagerService) UpdateEvent(ctx context.Context, id int64, event events.Event) error {
	if f.updatedEvents == nil {
		f.updatedEvents = make(map[int64]events.Event)
	}
	f.updatedEvents[id] = event
	return nil
}

func (f *fakeEventManagerService) DeleteEvent(ctx context.Context, id int64) error {
	f.deletedEvents = append(f.deletedEvents, id)
	return nil
}

func (f *fakeEventManagerService) ListRecurrenceRules(ctx context.Context, activeOnly bool) ([]events.RecurrenceRule, error) {
	return append([]events.RecurrenceRule(nil), f.rules...), nil
}

func (f *fakeEventManagerService) CreateRecurrenceRule(ctx context.Context, rule events.RecurrenceRule) (int64, error) {
	return 55, nil
}

func (f *fakeEventManagerService) GetRecurrenceRuleByID(ctx context.Context, id int64) (events.RecurrenceRule, error) {
	for _, row := range f.rules {
		if row.ID == id {
			return row, nil
		}
	}
	return events.RecurrenceRule{}, fmt.Errorf("not found")
}

func (f *fakeEventManagerService) UpdateRecurrenceRule(ctx context.Context, id int64, rule events.RecurrenceRule) error {
	return nil
}

func (f *fakeEventManagerService) DeleteRecurrenceRule(ctx context.Context, id int64) error {
	return nil
}

func (f *fakeEventManagerService) AddDependency(ctx context.Context, eventID, dependsOnEventID int64, required bool) error {
	return nil
}

func (f *fakeEventManagerService) DeleteDependency(ctx context.Context, eventID, dependsOnEventID int64) error {
	return nil
}

func (f *fakeEventManagerService) SetDependencyOverride(ctx context.Context, eventID int64, enabled, admin bool, reason, origin string) error {
	return nil
}

func (f *fakeEventManagerService) ListDependencies(ctx context.Context, eventID int64) ([]events.Dependency, error) {
	return append([]events.Dependency(nil), f.deps...), nil
}

func TestEventManagerModelStateTransitionsAddSingle(t *testing.T) {
	now := time.Date(2026, 2, 28, 9, 0, 0, 0, time.UTC)
	service := &fakeEventManagerService{
		rows: []events.Event{
			{ID: 1, Title: "A", Domain: "Math", Subtopic: "General", StartTime: now, Status: "planned", Source: "manual"},
		},
		rules: []events.RecurrenceRule{
			{ID: 7, Title: "Weekly", Domain: "Math", Subtopic: "General", RRule: "FREQ=WEEKLY;INTERVAL=1"},
		},
	}

	m := newEventManagerModel(service, func() time.Time { return now })

	initMsg := m.Init()().(eventSnapshotMsg)
	updated, _ := m.Update(initMsg)
	state := updated.(eventManagerModel)
	if len(state.rows) != 1 || len(state.rule) != 1 {
		t.Fatalf("expected loaded rows and rules")
	}

	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state = updated.(eventManagerModel)
	if state.form == nil || state.formKind != eventFormAddSingle {
		t.Fatalf("expected add single form to open")
	}

	state.form.Fields[0].Value = "Deep Work"
	state.form.Fields[1].Value = "2026-03-01T10:00"
	state.form.Fields[2].Value = "2026-03-01T11:00"
	state.form.Fields[3].Value = "Study"
	state.form.Fields[4].Value = "Algorithms"
	state.form.Fields[5].Value = "task"
	state.form.Cursor = len(state.form.Fields) - 1

	updated, submitCmd := state.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if submitCmd == nil {
		t.Fatalf("expected submit command")
	}
	actionMsg := submitCmd().(eventActionMsg)
	if actionMsg.Err != nil {
		t.Fatalf("unexpected action error: %v", actionMsg.Err)
	}
	state = updated.(eventManagerModel)
	updated, refreshCmd := state.Update(actionMsg)
	state = updated.(eventManagerModel)
	if len(service.createdEvents) != 1 {
		t.Fatalf("expected one created event, got %d", len(service.createdEvents))
	}
	if refreshCmd == nil {
		t.Fatalf("expected refresh command after successful submit")
	}
	refreshMsg := refreshCmd().(eventSnapshotMsg)
	updated, _ = state.Update(refreshMsg)
	state = updated.(eventManagerModel)
	if state.err != "" {
		t.Fatalf("did not expect model error, got %s", state.err)
	}
}

func TestEventManagerKeyboardNavigationAndCancel(t *testing.T) {
	service := &fakeEventManagerService{}
	m := newEventManagerModel(service, time.Now)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	state := updated.(eventManagerModel)
	if state.menu.Cursor != 1 {
		t.Fatalf("expected cursor 1 after down, got %d", state.menu.Cursor)
	}

	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	state = updated.(eventManagerModel)
	if state.menu.Cursor != 2 {
		t.Fatalf("expected cursor 2 after j, got %d", state.menu.Cursor)
	}

	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyUp})
	state = updated.(eventManagerModel)
	if state.menu.Cursor != 1 {
		t.Fatalf("expected cursor 1 after up, got %d", state.menu.Cursor)
	}

	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state = updated.(eventManagerModel)
	if state.form == nil {
		t.Fatalf("expected form open for selected action")
	}

	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyEsc})
	state = updated.(eventManagerModel)
	if state.form != nil {
		t.Fatalf("expected form closed on esc")
	}
}

func TestEventManagerDependencyEditorLoadsDependencies(t *testing.T) {
	service := &fakeEventManagerService{
		deps: []events.Dependency{
			{EventID: 10, DependsOnEventID: 9, Required: true, DependsOnStatus: "done", DependsOnTitle: "Lecture"},
		},
	}
	m := newEventManagerModel(service, time.Now)
	m.menu.Cursor = 9 // list dependencies action

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(eventManagerModel)
	if state.form == nil || state.formKind != eventFormListDependencies {
		t.Fatalf("expected list dependency form")
	}
	state.form.Fields[0].Value = "10"
	state.form.Cursor = 0

	updated, submitCmd := state.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if submitCmd == nil {
		t.Fatalf("expected submit command")
	}
	actionMsg := submitCmd().(eventActionMsg)
	if actionMsg.Err != nil {
		t.Fatalf("unexpected list dependency error: %v", actionMsg.Err)
	}

	state = updated.(eventManagerModel)
	updated, _ = state.Update(actionMsg)
	state = updated.(eventManagerModel)
	if len(state.deps) != 1 {
		t.Fatalf("expected one dependency, got %d", len(state.deps))
	}
}
