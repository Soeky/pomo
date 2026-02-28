package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/events"
	"github.com/Soeky/pomo/internal/topics"
	tea "github.com/charmbracelet/bubbletea"
)

type eventManagerService interface {
	ListCanonicalInRange(ctx context.Context, from, to time.Time) ([]events.Event, error)
	CreateEvent(ctx context.Context, event events.Event) (int64, error)
	GetEventByID(ctx context.Context, id int64) (events.Event, error)
	UpdateEvent(ctx context.Context, id int64, event events.Event) error
	DeleteEvent(ctx context.Context, id int64) error
	ListRecurrenceRules(ctx context.Context, activeOnly bool) ([]events.RecurrenceRule, error)
	CreateRecurrenceRule(ctx context.Context, rule events.RecurrenceRule) (int64, error)
	GetRecurrenceRuleByID(ctx context.Context, id int64) (events.RecurrenceRule, error)
	UpdateRecurrenceRule(ctx context.Context, id int64, rule events.RecurrenceRule) error
	DeleteRecurrenceRule(ctx context.Context, id int64) error
	AddDependency(ctx context.Context, eventID, dependsOnEventID int64, required bool) error
	DeleteDependency(ctx context.Context, eventID, dependsOnEventID int64) error
	SetDependencyOverride(ctx context.Context, eventID int64, enabled, admin bool, reason, origin string) error
	ListDependencies(ctx context.Context, eventID int64) ([]events.Dependency, error)
}

type defaultEventManagerService struct{}

func (defaultEventManagerService) ListCanonicalInRange(ctx context.Context, from, to time.Time) ([]events.Event, error) {
	return events.ListCanonicalInRange(ctx, from, to)
}

func (defaultEventManagerService) CreateEvent(ctx context.Context, event events.Event) (int64, error) {
	return events.Create(ctx, event)
}

func (defaultEventManagerService) GetEventByID(ctx context.Context, id int64) (events.Event, error) {
	return events.GetByID(ctx, id)
}

func (defaultEventManagerService) UpdateEvent(ctx context.Context, id int64, event events.Event) error {
	return events.Update(ctx, id, event)
}

func (defaultEventManagerService) DeleteEvent(ctx context.Context, id int64) error {
	return events.Delete(ctx, id)
}

func (defaultEventManagerService) ListRecurrenceRules(ctx context.Context, activeOnly bool) ([]events.RecurrenceRule, error) {
	return events.ListRecurrenceRules(ctx, activeOnly)
}

func (defaultEventManagerService) CreateRecurrenceRule(ctx context.Context, rule events.RecurrenceRule) (int64, error) {
	return events.CreateRecurrenceRule(ctx, rule)
}

func (defaultEventManagerService) GetRecurrenceRuleByID(ctx context.Context, id int64) (events.RecurrenceRule, error) {
	return events.GetRecurrenceRuleByID(ctx, id)
}

func (defaultEventManagerService) UpdateRecurrenceRule(ctx context.Context, id int64, rule events.RecurrenceRule) error {
	return events.UpdateRecurrenceRule(ctx, id, rule)
}

func (defaultEventManagerService) DeleteRecurrenceRule(ctx context.Context, id int64) error {
	return events.DeleteRecurrenceRule(ctx, id)
}

func (defaultEventManagerService) AddDependency(ctx context.Context, eventID, dependsOnEventID int64, required bool) error {
	return events.AddDependency(ctx, eventID, dependsOnEventID, required)
}

func (defaultEventManagerService) DeleteDependency(ctx context.Context, eventID, dependsOnEventID int64) error {
	return events.DeleteDependency(ctx, eventID, dependsOnEventID)
}

func (defaultEventManagerService) SetDependencyOverride(ctx context.Context, eventID int64, enabled, admin bool, reason, origin string) error {
	return events.SetDependencyOverride(ctx, eventID, enabled, admin, reason, origin)
}

func (defaultEventManagerService) ListDependencies(ctx context.Context, eventID int64) ([]events.Dependency, error) {
	return events.ListDependencies(ctx, eventID)
}

type eventManagerModel struct {
	menu     menuModel
	form     *formModel
	formKind eventFormKind

	status string
	err    string

	rows []events.Event
	rule []events.RecurrenceRule
	deps []events.Dependency

	service eventManagerService
	nowFn   func() time.Time
}

type eventFormKind int

const (
	eventFormNone eventFormKind = iota
	eventFormAddSingle
	eventFormEditSingle
	eventFormDeleteSingle
	eventFormAddRecurring
	eventFormEditRecurring
	eventFormDeleteRecurring
	eventFormAddDependency
	eventFormDeleteDependency
	eventFormToggleOverride
	eventFormListDependencies
)

type eventSnapshotMsg struct {
	Rows  []events.Event
	Rules []events.RecurrenceRule
	Err   error
}

type eventActionMsg struct {
	Status  string
	Err     error
	Deps    []events.Dependency
	Refresh bool
}

func NewEventManagerModel() tea.Model {
	return newEventManagerModel(defaultEventManagerService{}, time.Now)
}

func newEventManagerModel(service eventManagerService, nowFn func() time.Time) eventManagerModel {
	return eventManagerModel{
		menu: menuModel{
			Title: "Event manager actions",
			Items: []menuItem{
				{Label: "Add single event", Description: "create one manual event"},
				{Label: "Edit single event", Description: "patch an existing event by ID"},
				{Label: "Delete single event", Description: "delete an event by ID"},
				{Label: "Add recurring rule", Description: "create recurring schedule rule"},
				{Label: "Edit recurring rule", Description: "patch a rule by ID"},
				{Label: "Delete recurring rule", Description: "delete a rule by ID"},
				{Label: "Add dependency", Description: "event A depends on event B"},
				{Label: "Delete dependency", Description: "remove dependency edge"},
				{Label: "Toggle dependency override", Description: "admin override blocked state"},
				{Label: "List dependencies", Description: "show dependencies for one event"},
				{Label: "Refresh snapshot", Description: "reload events and rules list"},
			},
		},
		service: service,
		nowFn:   nowFn,
	}
}

func RunEventManager(opts RunOptions) error {
	return runProgram(NewEventManagerModel(), opts)
}

func (m eventManagerModel) Init() tea.Cmd {
	return m.loadSnapshotCmd()
}

func (m eventManagerModel) loadSnapshotCmd() tea.Cmd {
	service := m.service
	now := m.nowFn()
	from := now.Add(-24 * time.Hour)
	to := now.AddDate(0, 0, 14)
	return func() tea.Msg {
		rows, err := service.ListCanonicalInRange(context.Background(), from, to)
		if err != nil {
			return eventSnapshotMsg{Err: err}
		}
		rules, err := service.ListRecurrenceRules(context.Background(), false)
		if err != nil {
			return eventSnapshotMsg{Err: err}
		}
		return eventSnapshotMsg{Rows: rows, Rules: rules}
	}
}

func (m eventManagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case eventSnapshotMsg:
		if typed.Err != nil {
			m.err = typed.Err.Error()
			return m, nil
		}
		m.err = ""
		m.status = fmt.Sprintf("Loaded %d events and %d recurring rules", len(typed.Rows), len(typed.Rules))
		m.rows = typed.Rows
		m.rule = typed.Rules
		return m, nil
	case eventActionMsg:
		if typed.Err != nil {
			m.err = typed.Err.Error()
			return m, nil
		}
		m.err = ""
		m.status = typed.Status
		if typed.Deps != nil {
			m.deps = typed.Deps
		}
		if typed.Refresh {
			return m, m.loadSnapshotCmd()
		}
		return m, nil
	case tea.KeyMsg:
		if isQuitKey(typed) {
			return m, tea.Quit
		}
		if m.form != nil {
			submitted, canceled := m.form.Update(typed)
			if canceled {
				m.form = nil
				m.formKind = eventFormNone
				m.status = "Canceled action"
				return m, nil
			}
			if submitted {
				values := m.form.values()
				formKind := m.formKind
				m.form = nil
				m.formKind = eventFormNone
				return m, m.submitFormCmd(formKind, values)
			}
			return m, nil
		}

		switch typed.String() {
		case "up", "k":
			m.menu.move(-1)
			return m, nil
		case "down", "j", "tab":
			m.menu.move(1)
			return m, nil
		case "enter":
			m.openFormForSelectedAction()
			return m, nil
		case "r":
			return m, m.loadSnapshotCmd()
		}
	}
	return m, nil
}

func (m *eventManagerModel) openFormForSelectedAction() {
	switch m.menu.selectedIndex() {
	case 0:
		m.formKind = eventFormAddSingle
		m.form = &formModel{
			Title: "Add single event",
			Fields: []formField{
				{Key: "title", Label: "Title", Placeholder: "Math study block"},
				{Key: "start", Label: "Start", Placeholder: time.Now().Add(time.Hour).Format("2006-01-02T15:04")},
				{Key: "end", Label: "End", Placeholder: time.Now().Add(2 * time.Hour).Format("2006-01-02T15:04")},
				{Key: "domain", Label: "Domain", Value: topics.DefaultDomain},
				{Key: "subtopic", Label: "Subtopic", Value: topics.DefaultSubtopic},
				{Key: "kind", Label: "Kind", Value: "task"},
			},
			SubmitLabel: "Enter on last field to create",
		}
	case 1:
		m.formKind = eventFormEditSingle
		m.form = &formModel{
			Title: "Edit single event",
			Fields: []formField{
				{Key: "id", Label: "Event ID", Placeholder: "required"},
				{Key: "title", Label: "Title", Placeholder: "leave blank to keep"},
				{Key: "start", Label: "Start", Placeholder: "leave blank to keep"},
				{Key: "end", Label: "End", Placeholder: "leave blank to keep"},
				{Key: "domain", Label: "Domain", Placeholder: "leave blank to keep"},
				{Key: "subtopic", Label: "Subtopic", Placeholder: "leave blank to keep"},
				{Key: "kind", Label: "Kind", Placeholder: "leave blank to keep"},
				{Key: "layer", Label: "Layer", Placeholder: "leave blank to keep"},
				{Key: "status", Label: "Status", Placeholder: "leave blank to keep"},
				{Key: "source", Label: "Source", Placeholder: "leave blank to keep"},
				{Key: "description", Label: "Description", Placeholder: "leave blank to keep"},
			},
			SubmitLabel: "Enter on last field to update",
		}
	case 2:
		m.formKind = eventFormDeleteSingle
		m.form = &formModel{
			Title: "Delete single event",
			Fields: []formField{
				{Key: "id", Label: "Event ID", Placeholder: "required"},
				{Key: "confirm", Label: "Confirm", Placeholder: "type yes"},
			},
			SubmitLabel: "Enter on last field to delete",
		}
	case 3:
		m.formKind = eventFormAddRecurring
		m.form = &formModel{
			Title: "Add recurring rule",
			Fields: []formField{
				{Key: "title", Label: "Title", Placeholder: "Weekly review"},
				{Key: "start", Label: "Start", Placeholder: time.Now().Add(time.Hour).Format("2006-01-02T15:04")},
				{Key: "duration", Label: "Duration", Placeholder: "1h"},
				{Key: "domain", Label: "Domain", Value: topics.DefaultDomain},
				{Key: "subtopic", Label: "Subtopic", Value: topics.DefaultSubtopic},
				{Key: "kind", Label: "Kind", Value: "task"},
				{Key: "freq", Label: "Freq", Value: "weekly"},
				{Key: "interval", Label: "Interval", Value: "1"},
				{Key: "byday", Label: "ByDay", Placeholder: "MO,WE"},
				{Key: "bymonthday", Label: "ByMonthDay", Placeholder: "1-31 optional"},
				{Key: "until", Label: "Until", Placeholder: "optional"},
				{Key: "timezone", Label: "Timezone", Value: "Local"},
				{Key: "active", Label: "Active", Value: "true"},
			},
			SubmitLabel: "Enter on last field to create",
		}
	case 4:
		m.formKind = eventFormEditRecurring
		m.form = &formModel{
			Title: "Edit recurring rule",
			Fields: []formField{
				{Key: "id", Label: "Rule ID", Placeholder: "required"},
				{Key: "title", Label: "Title", Placeholder: "leave blank to keep"},
				{Key: "start", Label: "Start", Placeholder: "leave blank to keep"},
				{Key: "duration", Label: "Duration", Placeholder: "leave blank to keep"},
				{Key: "domain", Label: "Domain", Placeholder: "leave blank to keep"},
				{Key: "subtopic", Label: "Subtopic", Placeholder: "leave blank to keep"},
				{Key: "kind", Label: "Kind", Placeholder: "leave blank to keep"},
				{Key: "freq", Label: "Freq", Placeholder: "leave blank to keep"},
				{Key: "interval", Label: "Interval", Placeholder: "leave blank to keep"},
				{Key: "byday", Label: "ByDay", Placeholder: "leave blank to keep"},
				{Key: "bymonthday", Label: "ByMonthDay", Placeholder: "leave blank to keep"},
				{Key: "until", Label: "Until", Placeholder: "blank keep, '-' clears"},
				{Key: "timezone", Label: "Timezone", Placeholder: "leave blank to keep"},
				{Key: "active", Label: "Active", Placeholder: "leave blank to keep"},
			},
			SubmitLabel: "Enter on last field to update",
		}
	case 5:
		m.formKind = eventFormDeleteRecurring
		m.form = &formModel{
			Title: "Delete recurring rule",
			Fields: []formField{
				{Key: "id", Label: "Rule ID", Placeholder: "required"},
				{Key: "confirm", Label: "Confirm", Placeholder: "type yes"},
			},
			SubmitLabel: "Enter on last field to delete",
		}
	case 6:
		m.formKind = eventFormAddDependency
		m.form = &formModel{
			Title: "Add dependency",
			Fields: []formField{
				{Key: "event_id", Label: "Event ID", Placeholder: "required"},
				{Key: "depends_on_id", Label: "Depends On ID", Placeholder: "required"},
				{Key: "required", Label: "Required", Value: "true"},
			},
			SubmitLabel: "Enter on last field to add",
		}
	case 7:
		m.formKind = eventFormDeleteDependency
		m.form = &formModel{
			Title: "Delete dependency",
			Fields: []formField{
				{Key: "event_id", Label: "Event ID", Placeholder: "required"},
				{Key: "depends_on_id", Label: "Depends On ID", Placeholder: "required"},
			},
			SubmitLabel: "Enter on last field to delete",
		}
	case 8:
		m.formKind = eventFormToggleOverride
		m.form = &formModel{
			Title: "Toggle dependency override",
			Fields: []formField{
				{Key: "event_id", Label: "Event ID", Placeholder: "required"},
				{Key: "enabled", Label: "Enabled", Value: "true"},
				{Key: "reason", Label: "Reason", Placeholder: "optional"},
			},
			SubmitLabel: "Enter on last field to apply",
		}
	case 9:
		m.formKind = eventFormListDependencies
		m.form = &formModel{
			Title: "List dependencies",
			Fields: []formField{
				{Key: "event_id", Label: "Event ID", Placeholder: "required"},
			},
			SubmitLabel: "Enter to list dependencies",
		}
	default:
		m.formKind = eventFormNone
		m.form = nil
	}
}

func (m eventManagerModel) submitFormCmd(formKind eventFormKind, values map[string]string) tea.Cmd {
	service := m.service
	return func() tea.Msg {
		switch formKind {
		case eventFormAddSingle:
			msg, err := submitAddSingle(service, values)
			return eventActionMsg{Status: msg, Err: err, Refresh: err == nil}
		case eventFormEditSingle:
			msg, err := submitEditSingle(service, values)
			return eventActionMsg{Status: msg, Err: err, Refresh: err == nil}
		case eventFormDeleteSingle:
			msg, err := submitDeleteSingle(service, values)
			return eventActionMsg{Status: msg, Err: err, Refresh: err == nil}
		case eventFormAddRecurring:
			msg, err := submitAddRecurring(service, values)
			return eventActionMsg{Status: msg, Err: err, Refresh: err == nil}
		case eventFormEditRecurring:
			msg, err := submitEditRecurring(service, values)
			return eventActionMsg{Status: msg, Err: err, Refresh: err == nil}
		case eventFormDeleteRecurring:
			msg, err := submitDeleteRecurring(service, values)
			return eventActionMsg{Status: msg, Err: err, Refresh: err == nil}
		case eventFormAddDependency:
			msg, err := submitAddDependency(service, values)
			return eventActionMsg{Status: msg, Err: err, Refresh: err == nil}
		case eventFormDeleteDependency:
			msg, err := submitDeleteDependency(service, values)
			return eventActionMsg{Status: msg, Err: err, Refresh: err == nil}
		case eventFormToggleOverride:
			msg, deps, err := submitToggleOverride(service, values)
			return eventActionMsg{Status: msg, Deps: deps, Err: err}
		case eventFormListDependencies:
			msg, deps, err := submitListDependencies(service, values)
			return eventActionMsg{Status: msg, Deps: deps, Err: err}
		default:
			return eventActionMsg{Status: "No action selected"}
		}
	}
}

func (m eventManagerModel) View() string {
	var b strings.Builder
	b.WriteString("Event Manager (Bubble Tea)\n")
	b.WriteString("Arrows/J/K navigate | Enter select | Q quit | R refresh snapshot\n")
	if strings.TrimSpace(m.err) != "" {
		b.WriteString("Error: ")
		b.WriteString(m.err)
		b.WriteString("\n")
	}
	if strings.TrimSpace(m.status) != "" {
		b.WriteString("Status: ")
		b.WriteString(m.status)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if m.form != nil {
		b.WriteString(m.form.View())
		b.WriteString("\n\n")
	} else {
		b.WriteString(m.menu.View())
		b.WriteString("\n\n")
	}

	b.WriteString("Upcoming canonical events (first 5)\n")
	if len(m.rows) == 0 {
		b.WriteString("- none\n")
	} else {
		limit := len(m.rows)
		if limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			row := m.rows[i]
			b.WriteString(fmt.Sprintf("- #%d %s [%s::%s] %s (%s/%s)\n",
				row.ID,
				row.Title,
				row.Domain,
				row.Subtopic,
				row.StartTime.Format("2006-01-02 15:04"),
				row.Status,
				row.Source))
		}
	}

	b.WriteString("\nRecurring rules (first 5)\n")
	if len(m.rule) == 0 {
		b.WriteString("- none\n")
	} else {
		limit := len(m.rule)
		if limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			r := m.rule[i]
			b.WriteString(fmt.Sprintf("- #%d %s [%s::%s] %s\n", r.ID, r.Title, r.Domain, r.Subtopic, r.RRule))
		}
	}

	if len(m.deps) > 0 {
		b.WriteString("\nDependencies\n")
		for _, dep := range m.deps {
			mode := "required"
			if !dep.Required {
				mode = "optional"
			}
			b.WriteString(fmt.Sprintf("- %d <- %d (%s) status=%s title=%s\n",
				dep.EventID,
				dep.DependsOnEventID,
				mode,
				dep.DependsOnStatus,
				dep.DependsOnTitle))
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

func submitAddSingle(service eventManagerService, values map[string]string) (string, error) {
	title := strings.TrimSpace(values["title"])
	if title == "" {
		return "", fmt.Errorf("title is required")
	}
	start, err := parseDateTime(values["start"])
	if err != nil {
		return "", err
	}
	end, err := parseDateTime(values["end"])
	if err != nil {
		return "", err
	}
	path, err := topics.ParseParts(defaultIfEmpty(values["domain"], topics.DefaultDomain), defaultIfEmpty(values["subtopic"], topics.DefaultSubtopic))
	if err != nil {
		return "", err
	}
	kind := defaultIfEmpty(values["kind"], "task")
	id, err := service.CreateEvent(context.Background(), events.Event{
		Title:     title,
		Kind:      kind,
		Domain:    path.Domain,
		Subtopic:  path.Subtopic,
		StartTime: start,
		EndTime:   end,
		Layer:     "planned",
		Status:    "planned",
		Source:    "manual",
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Created event #%d", id), nil
}

func submitEditSingle(service eventManagerService, values map[string]string) (string, error) {
	eventID, err := parseRequiredInt64(values["id"], "event id")
	if err != nil {
		return "", err
	}
	row, err := service.GetEventByID(context.Background(), eventID)
	if err != nil {
		return "", err
	}

	if v := strings.TrimSpace(values["title"]); v != "" {
		row.Title = v
	}
	if v := strings.TrimSpace(values["start"]); v != "" {
		parsed, err := parseDateTime(v)
		if err != nil {
			return "", err
		}
		row.StartTime = parsed
	}
	if v := strings.TrimSpace(values["end"]); v != "" {
		parsed, err := parseDateTime(v)
		if err != nil {
			return "", err
		}
		row.EndTime = parsed
	}
	if strings.TrimSpace(values["domain"]) != "" || strings.TrimSpace(values["subtopic"]) != "" {
		path, err := topics.ParseParts(defaultIfEmpty(values["domain"], row.Domain), defaultIfEmpty(values["subtopic"], row.Subtopic))
		if err != nil {
			return "", err
		}
		row.Domain = path.Domain
		row.Subtopic = path.Subtopic
	}
	if v := strings.TrimSpace(values["kind"]); v != "" {
		row.Kind = v
	}
	if v := strings.TrimSpace(values["layer"]); v != "" {
		row.Layer = v
	}
	if v := strings.TrimSpace(values["status"]); v != "" {
		row.Status = v
	}
	if v := strings.TrimSpace(values["source"]); v != "" {
		row.Source = v
	}
	if v := strings.TrimSpace(values["description"]); v != "" {
		row.Description = v
	}

	if err := service.UpdateEvent(context.Background(), eventID, row); err != nil {
		return "", err
	}
	return fmt.Sprintf("Updated event #%d", eventID), nil
}

func submitDeleteSingle(service eventManagerService, values map[string]string) (string, error) {
	eventID, err := parseRequiredInt64(values["id"], "event id")
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(strings.TrimSpace(values["confirm"]), "yes") {
		return "", fmt.Errorf("confirmation must be 'yes'")
	}
	if err := service.DeleteEvent(context.Background(), eventID); err != nil {
		return "", err
	}
	return fmt.Sprintf("Deleted event #%d", eventID), nil
}

func submitAddRecurring(service eventManagerService, values map[string]string) (string, error) {
	title := strings.TrimSpace(values["title"])
	if title == "" {
		return "", fmt.Errorf("title is required")
	}
	start, err := parseDateTime(values["start"])
	if err != nil {
		return "", err
	}
	durationSec, err := parseDurationSeconds(values["duration"])
	if err != nil {
		return "", err
	}
	path, err := topics.ParseParts(defaultIfEmpty(values["domain"], topics.DefaultDomain), defaultIfEmpty(values["subtopic"], topics.DefaultSubtopic))
	if err != nil {
		return "", err
	}
	interval, err := parseOptionalInt(values["interval"], 1)
	if err != nil {
		return "", err
	}
	byDays, err := parseRecurrenceWeekdays(values["byday"])
	if err != nil {
		return "", err
	}
	byMonthDay, err := parseOptionalInt(values["bymonthday"], 0)
	if err != nil {
		return "", err
	}
	rrule, err := events.BuildRRule(events.RecurrenceSpec{
		Freq:       defaultIfEmpty(values["freq"], "weekly"),
		Interval:   interval,
		ByDays:     byDays,
		ByMonthDay: byMonthDay,
	})
	if err != nil {
		return "", err
	}
	until, err := parseOptionalDateTime(values["until"])
	if err != nil {
		return "", err
	}
	active, err := parseBoolDefault(values["active"], true)
	if err != nil {
		return "", err
	}
	id, err := service.CreateRecurrenceRule(context.Background(), events.RecurrenceRule{
		Title:       title,
		Domain:      path.Domain,
		Subtopic:    path.Subtopic,
		Kind:        defaultIfEmpty(values["kind"], "task"),
		DurationSec: durationSec,
		RRule:       rrule,
		Timezone:    defaultIfEmpty(values["timezone"], "Local"),
		StartDate:   start,
		EndDate:     until,
		Active:      active,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Created recurring rule #%d", id), nil
}

func submitEditRecurring(service eventManagerService, values map[string]string) (string, error) {
	ruleID, err := parseRequiredInt64(values["id"], "rule id")
	if err != nil {
		return "", err
	}
	rule, err := service.GetRecurrenceRuleByID(context.Background(), ruleID)
	if err != nil {
		return "", err
	}
	if v := strings.TrimSpace(values["title"]); v != "" {
		rule.Title = v
	}
	if v := strings.TrimSpace(values["start"]); v != "" {
		parsed, err := parseDateTime(v)
		if err != nil {
			return "", err
		}
		rule.StartDate = parsed
	}
	if v := strings.TrimSpace(values["duration"]); v != "" {
		durationSec, err := parseDurationSeconds(v)
		if err != nil {
			return "", err
		}
		rule.DurationSec = durationSec
	}
	if strings.TrimSpace(values["domain"]) != "" || strings.TrimSpace(values["subtopic"]) != "" {
		path, err := topics.ParseParts(defaultIfEmpty(values["domain"], rule.Domain), defaultIfEmpty(values["subtopic"], rule.Subtopic))
		if err != nil {
			return "", err
		}
		rule.Domain = path.Domain
		rule.Subtopic = path.Subtopic
	}
	if v := strings.TrimSpace(values["kind"]); v != "" {
		rule.Kind = v
	}
	if v := strings.TrimSpace(values["timezone"]); v != "" {
		rule.Timezone = v
	}
	if v := strings.TrimSpace(values["active"]); v != "" {
		active, err := parseBoolDefault(v, rule.Active)
		if err != nil {
			return "", err
		}
		rule.Active = active
	}
	if v := strings.TrimSpace(values["until"]); v != "" {
		if v == "-" {
			rule.EndDate = nil
		} else {
			until, err := parseDateTime(v)
			if err != nil {
				return "", err
			}
			rule.EndDate = &until
		}
	}

	spec, err := events.ParseRRule(rule.RRule)
	if err != nil {
		return "", err
	}
	changedSpec := false
	if v := strings.TrimSpace(values["freq"]); v != "" {
		spec.Freq = v
		changedSpec = true
	}
	if v := strings.TrimSpace(values["interval"]); v != "" {
		interval, err := strconv.Atoi(v)
		if err != nil {
			return "", err
		}
		spec.Interval = interval
		changedSpec = true
	}
	if v := strings.TrimSpace(values["byday"]); v != "" {
		byDays, err := parseRecurrenceWeekdays(v)
		if err != nil {
			return "", err
		}
		spec.ByDays = byDays
		changedSpec = true
	}
	if v := strings.TrimSpace(values["bymonthday"]); v != "" {
		day, err := strconv.Atoi(v)
		if err != nil {
			return "", err
		}
		spec.ByMonthDay = day
		changedSpec = true
	}
	if changedSpec {
		rule.RRule, err = events.BuildRRule(spec)
		if err != nil {
			return "", err
		}
	}

	if err := service.UpdateRecurrenceRule(context.Background(), ruleID, rule); err != nil {
		return "", err
	}
	return fmt.Sprintf("Updated recurring rule #%d", ruleID), nil
}

func submitDeleteRecurring(service eventManagerService, values map[string]string) (string, error) {
	ruleID, err := parseRequiredInt64(values["id"], "rule id")
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(strings.TrimSpace(values["confirm"]), "yes") {
		return "", fmt.Errorf("confirmation must be 'yes'")
	}
	if err := service.DeleteRecurrenceRule(context.Background(), ruleID); err != nil {
		return "", err
	}
	return fmt.Sprintf("Deleted recurring rule #%d", ruleID), nil
}

func submitAddDependency(service eventManagerService, values map[string]string) (string, error) {
	eventID, err := parseRequiredInt64(values["event_id"], "event id")
	if err != nil {
		return "", err
	}
	dependsOnID, err := parseRequiredInt64(values["depends_on_id"], "depends-on id")
	if err != nil {
		return "", err
	}
	required, err := parseBoolDefault(values["required"], true)
	if err != nil {
		return "", err
	}
	if err := service.AddDependency(context.Background(), eventID, dependsOnID, required); err != nil {
		return "", err
	}
	return fmt.Sprintf("Added dependency %d -> %d", eventID, dependsOnID), nil
}

func submitDeleteDependency(service eventManagerService, values map[string]string) (string, error) {
	eventID, err := parseRequiredInt64(values["event_id"], "event id")
	if err != nil {
		return "", err
	}
	dependsOnID, err := parseRequiredInt64(values["depends_on_id"], "depends-on id")
	if err != nil {
		return "", err
	}
	if err := service.DeleteDependency(context.Background(), eventID, dependsOnID); err != nil {
		return "", err
	}
	return fmt.Sprintf("Deleted dependency %d -/-> %d", eventID, dependsOnID), nil
}

func submitToggleOverride(service eventManagerService, values map[string]string) (string, []events.Dependency, error) {
	eventID, err := parseRequiredInt64(values["event_id"], "event id")
	if err != nil {
		return "", nil, err
	}
	enabled, err := parseBoolDefault(values["enabled"], true)
	if err != nil {
		return "", nil, err
	}
	reason := strings.TrimSpace(values["reason"])
	if err := service.SetDependencyOverride(context.Background(), eventID, enabled, true, reason, "tui"); err != nil {
		return "", nil, err
	}
	deps, err := service.ListDependencies(context.Background(), eventID)
	if err != nil {
		return "", nil, err
	}
	state := "disabled"
	if enabled {
		state = "enabled"
	}
	return fmt.Sprintf("Dependency override %s for event #%d", state, eventID), deps, nil
}

func submitListDependencies(service eventManagerService, values map[string]string) (string, []events.Dependency, error) {
	eventID, err := parseRequiredInt64(values["event_id"], "event id")
	if err != nil {
		return "", nil, err
	}
	deps, err := service.ListDependencies(context.Background(), eventID)
	if err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("Loaded %d dependencies for event #%d", len(deps), eventID), deps, nil
}

func defaultIfEmpty(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}
