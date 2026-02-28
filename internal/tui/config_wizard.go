package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/scheduler"
	tea "github.com/charmbracelet/bubbletea"
)

type configWizardValues struct {
	ActiveWeekdays        []string
	DayStart              string
	DayEnd                string
	LunchStart            string
	LunchDurationMinutes  int
	DinnerStart           string
	DinnerDurationMinutes int
	MaxHoursPerDay        int
	Timezone              string
	BreakThresholdMinutes int
}

type configWizardService interface {
	Load(ctx context.Context) (configWizardValues, error)
	Save(ctx context.Context, values configWizardValues) error
}

type defaultConfigWizardService struct{}

func (defaultConfigWizardService) Load(ctx context.Context) (configWizardValues, error) {
	cfg, err := scheduler.LoadConstraintConfig(ctx)
	if err != nil {
		return configWizardValues{}, err
	}
	threshold := config.AppConfig.BreakCreditThresholdMinutes
	if threshold <= 0 {
		threshold = 10
	}
	return configWizardValues{
		ActiveWeekdays:        append([]string(nil), cfg.ActiveWeekdays...),
		DayStart:              cfg.DayStart,
		DayEnd:                cfg.DayEnd,
		LunchStart:            cfg.LunchStart,
		LunchDurationMinutes:  cfg.LunchDurationMinutes,
		DinnerStart:           cfg.DinnerStart,
		DinnerDurationMinutes: cfg.DinnerDurationMinutes,
		MaxHoursPerDay:        cfg.MaxHoursPerDay,
		Timezone:              cfg.Timezone,
		BreakThresholdMinutes: threshold,
	}, nil
}

func (defaultConfigWizardService) Save(ctx context.Context, values configWizardValues) error {
	if _, err := time.Parse("15:04", values.DayStart); err != nil {
		return fmt.Errorf("invalid day start")
	}
	if _, err := time.Parse("15:04", values.DayEnd); err != nil {
		return fmt.Errorf("invalid day end")
	}
	if _, err := time.Parse("15:04", values.LunchStart); err != nil {
		return fmt.Errorf("invalid lunch start")
	}
	if _, err := time.Parse("15:04", values.DinnerStart); err != nil {
		return fmt.Errorf("invalid dinner start")
	}
	if len(values.ActiveWeekdays) == 0 {
		return fmt.Errorf("active weekdays cannot be empty")
	}
	if values.LunchDurationMinutes <= 0 || values.DinnerDurationMinutes <= 0 || values.MaxHoursPerDay <= 0 || values.BreakThresholdMinutes <= 0 {
		return fmt.Errorf("duration and threshold values must be positive")
	}
	if strings.TrimSpace(values.Timezone) == "" {
		values.Timezone = "Local"
	}

	if err := scheduler.SaveConstraintConfig(ctx, scheduler.ConstraintConfig{
		ActiveWeekdays:        append([]string(nil), values.ActiveWeekdays...),
		DayStart:              values.DayStart,
		DayEnd:                values.DayEnd,
		LunchStart:            values.LunchStart,
		LunchDurationMinutes:  values.LunchDurationMinutes,
		DinnerStart:           values.DinnerStart,
		DinnerDurationMinutes: values.DinnerDurationMinutes,
		MaxHoursPerDay:        values.MaxHoursPerDay,
		Timezone:              values.Timezone,
	}); err != nil {
		return err
	}

	config.AppConfig.ActiveWeekdays = append([]string(nil), values.ActiveWeekdays...)
	config.AppConfig.DayStart = values.DayStart
	config.AppConfig.DayEnd = values.DayEnd
	config.AppConfig.LunchStart = values.LunchStart
	config.AppConfig.LunchDurationMinutes = values.LunchDurationMinutes
	config.AppConfig.DinnerStart = values.DinnerStart
	config.AppConfig.DinnerDurationMinutes = values.DinnerDurationMinutes
	config.AppConfig.BreakCreditThresholdMinutes = values.BreakThresholdMinutes
	return config.SaveConfig()
}

type configWizardModel struct {
	form    formModel
	loaded  bool
	status  string
	err     string
	service configWizardService
}

type configWizardLoadedMsg struct {
	Values configWizardValues
	Err    error
}

type configWizardSavedMsg struct {
	Status string
	Err    error
}

func NewConfigWizardModel() tea.Model {
	return newConfigWizardModel(defaultConfigWizardService{})
}

func newConfigWizardModel(service configWizardService) configWizardModel {
	return configWizardModel{
		form: formModel{
			Title: "Config wizard (constraints + thresholds)",
			Fields: []formField{
				{Key: "active_weekdays", Label: "Weekdays", Placeholder: "mon,tue,wed,thu,fri"},
				{Key: "day_start", Label: "Day start", Placeholder: "08:00"},
				{Key: "day_end", Label: "Day end", Placeholder: "22:00"},
				{Key: "lunch_start", Label: "Lunch start", Placeholder: "12:30"},
				{Key: "lunch_duration", Label: "Lunch mins", Placeholder: "60"},
				{Key: "dinner_start", Label: "Dinner start", Placeholder: "19:00"},
				{Key: "dinner_duration", Label: "Dinner mins", Placeholder: "60"},
				{Key: "max_hours_day", Label: "Max hours/day", Placeholder: "8"},
				{Key: "timezone", Label: "Timezone", Placeholder: "Local"},
				{Key: "break_threshold", Label: "Break threshold", Placeholder: "10"},
			},
			SubmitLabel: "Press S to save, Enter on last field also saves, Esc cancels edit",
		},
		service: service,
	}
}

func RunConfigWizard(opts RunOptions) error {
	return runProgram(NewConfigWizardModel(), opts)
}

func (m configWizardModel) Init() tea.Cmd {
	service := m.service
	return func() tea.Msg {
		values, err := service.Load(context.Background())
		return configWizardLoadedMsg{Values: values, Err: err}
	}
}

func (m configWizardModel) saveCmd(values map[string]string) tea.Cmd {
	service := m.service
	return func() tea.Msg {
		weekdays, err := normalizeConstraintWeekdays(values["active_weekdays"])
		if err != nil {
			return configWizardSavedMsg{Err: err}
		}
		lunchDuration, err := strconv.Atoi(strings.TrimSpace(values["lunch_duration"]))
		if err != nil {
			return configWizardSavedMsg{Err: fmt.Errorf("invalid lunch duration")}
		}
		dinnerDuration, err := strconv.Atoi(strings.TrimSpace(values["dinner_duration"]))
		if err != nil {
			return configWizardSavedMsg{Err: fmt.Errorf("invalid dinner duration")}
		}
		maxHoursDay, err := strconv.Atoi(strings.TrimSpace(values["max_hours_day"]))
		if err != nil {
			return configWizardSavedMsg{Err: fmt.Errorf("invalid max hours/day")}
		}
		breakThreshold, err := strconv.Atoi(strings.TrimSpace(values["break_threshold"]))
		if err != nil {
			return configWizardSavedMsg{Err: fmt.Errorf("invalid break threshold")}
		}

		saveValues := configWizardValues{
			ActiveWeekdays:        weekdays,
			DayStart:              strings.TrimSpace(values["day_start"]),
			DayEnd:                strings.TrimSpace(values["day_end"]),
			LunchStart:            strings.TrimSpace(values["lunch_start"]),
			LunchDurationMinutes:  lunchDuration,
			DinnerStart:           strings.TrimSpace(values["dinner_start"]),
			DinnerDurationMinutes: dinnerDuration,
			MaxHoursPerDay:        maxHoursDay,
			Timezone:              defaultIfEmpty(values["timezone"], "Local"),
			BreakThresholdMinutes: breakThreshold,
		}
		if err := service.Save(context.Background(), saveValues); err != nil {
			return configWizardSavedMsg{Err: err}
		}
		return configWizardSavedMsg{Status: "Configuration saved"}
	}
}

func (m configWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case configWizardLoadedMsg:
		if typed.Err != nil {
			m.err = typed.Err.Error()
			return m, nil
		}
		m.err = ""
		m.loaded = true
		values := typed.Values
		m.form.Fields[0].Value = strings.Join(values.ActiveWeekdays, ",")
		m.form.Fields[1].Value = values.DayStart
		m.form.Fields[2].Value = values.DayEnd
		m.form.Fields[3].Value = values.LunchStart
		m.form.Fields[4].Value = strconv.Itoa(values.LunchDurationMinutes)
		m.form.Fields[5].Value = values.DinnerStart
		m.form.Fields[6].Value = strconv.Itoa(values.DinnerDurationMinutes)
		m.form.Fields[7].Value = strconv.Itoa(values.MaxHoursPerDay)
		m.form.Fields[8].Value = values.Timezone
		m.form.Fields[9].Value = strconv.Itoa(values.BreakThresholdMinutes)
		m.status = "Loaded current config values"
		return m, nil
	case configWizardSavedMsg:
		if typed.Err != nil {
			m.err = typed.Err.Error()
			return m, nil
		}
		m.err = ""
		m.status = typed.Status
		return m, nil
	case tea.KeyMsg:
		if isQuitKey(typed) {
			return m, tea.Quit
		}
		if !m.loaded {
			return m, nil
		}
		if typed.String() == "s" {
			return m, m.saveCmd(m.form.values())
		}
		submitted, canceled := m.form.Update(typed)
		if canceled {
			m.status = "Canceled field edit"
			return m, nil
		}
		if submitted {
			return m, m.saveCmd(m.form.values())
		}
		return m, nil
	}
	return m, nil
}

func (m configWizardModel) View() string {
	var b strings.Builder
	b.WriteString("Config Wizard (Bubble Tea)\n")
	b.WriteString("Arrows/J/K/Tab navigate | Type to edit | S save | Q quit\n")
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
	if !m.loaded {
		b.WriteString("Loading configuration...\n")
		return strings.TrimRight(b.String(), "\n")
	}
	b.WriteString(m.form.View())
	return strings.TrimRight(b.String(), "\n")
}
