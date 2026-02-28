package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/scheduler"
	tea "github.com/charmbracelet/bubbletea"
)

type schedulerReviewService interface {
	GenerateFromDB(ctx context.Context, in scheduler.DBInput) (scheduler.Result, error)
}

type defaultSchedulerReviewService struct{}

func (defaultSchedulerReviewService) GenerateFromDB(ctx context.Context, in scheduler.DBInput) (scheduler.Result, error) {
	return scheduler.GenerateFromDB(ctx, in)
}

type schedulerReviewModel struct {
	form    formModel
	status  string
	err     string
	lastRun *schedulerRunMsg

	service schedulerReviewService
	nowFn   func() time.Time
}

type schedulerRunMsg struct {
	Mode   string
	Result scheduler.Result
	Err    error
}

func NewSchedulerReviewModel() tea.Model {
	return newSchedulerReviewModel(defaultSchedulerReviewService{}, time.Now)
}

func newSchedulerReviewModel(service schedulerReviewService, nowFn func() time.Time) schedulerReviewModel {
	now := nowFn()
	return schedulerReviewModel{
		form: formModel{
			Title: "Scheduler review/apply",
			Fields: []formField{
				{Key: "from", Label: "From", Value: now.Format("2006-01-02T15:04")},
				{Key: "to", Label: "To", Value: now.AddDate(0, 0, 7).Format("2006-01-02T15:04")},
				{Key: "replace", Label: "Replace", Value: "true"},
			},
			SubmitLabel: "Press R to review (dry-run), A to apply, Enter on last field to review",
		},
		service: service,
		nowFn:   nowFn,
	}
}

func RunSchedulerReview(opts RunOptions) error {
	return runProgram(NewSchedulerReviewModel(), opts)
}

func (m schedulerReviewModel) Init() tea.Cmd {
	return nil
}

func (m schedulerReviewModel) runSchedulerCmd(apply bool) tea.Cmd {
	service := m.service
	values := m.form.values()
	return func() tea.Msg {
		from, err := parseDateTime(values["from"])
		if err != nil {
			return schedulerRunMsg{Err: err}
		}
		to, err := parseDateTime(values["to"])
		if err != nil {
			return schedulerRunMsg{Err: err}
		}
		replace, err := parseBoolDefault(values["replace"], true)
		if err != nil {
			return schedulerRunMsg{Err: err}
		}
		result, err := service.GenerateFromDB(context.Background(), scheduler.DBInput{
			From:                     from,
			To:                       to,
			Persist:                  apply,
			ReplaceExistingScheduler: replace,
		})
		mode := "review"
		if apply {
			mode = "apply"
		}
		return schedulerRunMsg{Mode: mode, Result: result, Err: err}
	}
}

func (m schedulerReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case schedulerRunMsg:
		if typed.Err != nil {
			m.err = typed.Err.Error()
			return m, nil
		}
		m.err = ""
		m.lastRun = &typed
		m.status = fmt.Sprintf("Scheduler %s complete: generated=%d diagnostics=%d", typed.Mode, len(typed.Result.Generated), len(typed.Result.Diagnostics))
		return m, nil
	case tea.KeyMsg:
		if isQuitKey(typed) {
			return m, tea.Quit
		}
		switch typed.String() {
		case "r":
			return m, m.runSchedulerCmd(false)
		case "a":
			return m, m.runSchedulerCmd(true)
		}
		submitted, canceled := m.form.Update(typed)
		if canceled {
			return m, nil
		}
		if submitted {
			return m, m.runSchedulerCmd(false)
		}
		return m, nil
	}
	return m, nil
}

func (m schedulerReviewModel) View() string {
	var b strings.Builder
	b.WriteString("Scheduler Review (Bubble Tea)\n")
	b.WriteString("Arrows/J/K/Tab navigate | R review dry-run | A apply | Q quit\n")
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
	b.WriteString(m.form.View())
	b.WriteString("\n\n")
	if m.lastRun != nil {
		b.WriteString(fmt.Sprintf("Last run mode: %s\n", m.lastRun.Mode))
		if m.lastRun.Result.RunID > 0 {
			b.WriteString(fmt.Sprintf("Run ID: %d\n", m.lastRun.Result.RunID))
		}
		for _, summary := range m.lastRun.Result.Summaries {
			b.WriteString(fmt.Sprintf("- target %d %s::%s required=%ds generated=%ds remaining=%ds\n",
				summary.TargetID,
				summary.Domain,
				summary.Subtopic,
				summary.RequiredSeconds,
				summary.GeneratedSeconds,
				summary.RemainingSeconds))
		}
		for _, diag := range m.lastRun.Result.Diagnostics {
			b.WriteString(fmt.Sprintf("! [%s] target=%d missing=%ds %s\n",
				strings.ToUpper(diag.Severity),
				diag.TargetID,
				diag.MissingSeconds,
				diag.Message))
		}
		if len(m.lastRun.Result.Summaries) == 0 && len(m.lastRun.Result.Diagnostics) == 0 {
			b.WriteString("No summaries or diagnostics.\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
