package events

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/topics"
)

type RecurrenceRule struct {
	ID          int64
	Title       string
	Domain      string
	Subtopic    string
	Kind        string
	DurationSec int
	RRule       string
	Timezone    string
	StartDate   time.Time
	EndDate     *time.Time
	Active      bool
}

type RecurrenceSpec struct {
	Freq       string
	Interval   int
	ByDays     []time.Weekday
	ByMonthDay int
}

type RecurrenceGenerationResult struct {
	RulesProcessed int
	Generated      int
	Skipped        int
}

func CreateRecurrenceRule(ctx context.Context, rule RecurrenceRule) (int64, error) {
	if err := normalizeAndValidateRecurrenceRule(&rule); err != nil {
		return 0, err
	}
	now := time.Now()
	active := 0
	if rule.Active {
		active = 1
	}
	res, err := db.DB.ExecContext(ctx, `
		INSERT INTO recurrence_rules(title, domain, subtopic, kind, duration, rrule, timezone, start_date, end_date, active, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.Title, rule.Domain, rule.Subtopic, rule.Kind, rule.DurationSec, rule.RRule, rule.Timezone, rule.StartDate, rule.EndDate, active, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func ListRecurrenceRules(ctx context.Context, activeOnly bool) ([]RecurrenceRule, error) {
	query := `
		SELECT id, title, domain, subtopic, kind, duration, rrule, timezone, start_date, end_date, active
		FROM recurrence_rules`
	args := []any{}
	if activeOnly {
		query += ` WHERE active = 1`
	}
	query += ` ORDER BY id ASC`
	rows, err := db.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RecurrenceRule
	for rows.Next() {
		rule, err := scanRecurrenceRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func GetRecurrenceRuleByID(ctx context.Context, id int64) (RecurrenceRule, error) {
	row := db.DB.QueryRowContext(ctx, `
		SELECT id, title, domain, subtopic, kind, duration, rrule, timezone, start_date, end_date, active
		FROM recurrence_rules
		WHERE id = ?`, id)
	return scanRecurrenceRule(row)
}

func UpdateRecurrenceRule(ctx context.Context, id int64, rule RecurrenceRule) error {
	if err := normalizeAndValidateRecurrenceRule(&rule); err != nil {
		return err
	}
	active := 0
	if rule.Active {
		active = 1
	}
	_, err := db.DB.ExecContext(ctx, `
		UPDATE recurrence_rules
		SET title = ?, domain = ?, subtopic = ?, kind = ?, duration = ?, rrule = ?, timezone = ?, start_date = ?, end_date = ?, active = ?, updated_at = ?
		WHERE id = ?`,
		rule.Title, rule.Domain, rule.Subtopic, rule.Kind, rule.DurationSec, rule.RRule, rule.Timezone, rule.StartDate, rule.EndDate, active, time.Now(), id)
	return err
}

func DeleteRecurrenceRule(ctx context.Context, id int64) error {
	_, err := db.DB.ExecContext(ctx, `DELETE FROM recurrence_rules WHERE id = ?`, id)
	return err
}

func BuildRRule(spec RecurrenceSpec) (string, error) {
	spec.Freq = strings.ToUpper(strings.TrimSpace(spec.Freq))
	if spec.Freq != "DAILY" && spec.Freq != "WEEKLY" && spec.Freq != "MONTHLY" {
		return "", fmt.Errorf("invalid recurrence frequency: %s", spec.Freq)
	}
	if spec.Interval <= 0 {
		spec.Interval = 1
	}

	parts := []string{
		"FREQ=" + spec.Freq,
		"INTERVAL=" + strconv.Itoa(spec.Interval),
	}
	if spec.Freq == "WEEKLY" && len(spec.ByDays) > 0 {
		parts = append(parts, "BYDAY="+formatByDays(spec.ByDays))
	}
	if spec.Freq == "MONTHLY" && spec.ByMonthDay > 0 {
		if spec.ByMonthDay < 1 || spec.ByMonthDay > 31 {
			return "", fmt.Errorf("invalid bymonthday: %d", spec.ByMonthDay)
		}
		parts = append(parts, "BYMONTHDAY="+strconv.Itoa(spec.ByMonthDay))
	}
	return strings.Join(parts, ";"), nil
}

func ParseRRule(raw string) (RecurrenceSpec, error) {
	spec := RecurrenceSpec{Interval: 1}
	pairs := strings.Split(strings.TrimSpace(raw), ";")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return RecurrenceSpec{}, fmt.Errorf("invalid rrule token: %s", pair)
		}
		key := strings.ToUpper(strings.TrimSpace(kv[0]))
		val := strings.ToUpper(strings.TrimSpace(kv[1]))
		switch key {
		case "FREQ":
			spec.Freq = val
		case "INTERVAL":
			n, err := strconv.Atoi(val)
			if err != nil || n <= 0 {
				return RecurrenceSpec{}, fmt.Errorf("invalid interval: %s", val)
			}
			spec.Interval = n
		case "BYDAY":
			days, err := parseByDays(val)
			if err != nil {
				return RecurrenceSpec{}, err
			}
			spec.ByDays = days
		case "BYMONTHDAY":
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 || n > 31 {
				return RecurrenceSpec{}, fmt.Errorf("invalid bymonthday: %s", val)
			}
			spec.ByMonthDay = n
		default:
			return RecurrenceSpec{}, fmt.Errorf("unsupported rrule key: %s", key)
		}
	}
	if spec.Freq != "DAILY" && spec.Freq != "WEEKLY" && spec.Freq != "MONTHLY" {
		return RecurrenceSpec{}, fmt.Errorf("invalid recurrence frequency: %s", spec.Freq)
	}
	return spec, nil
}

func ExpandRecurrenceRuleInWindow(rule RecurrenceRule, from, to time.Time) ([]Event, error) {
	if !rule.Active {
		return nil, nil
	}
	if !to.After(from) {
		return nil, fmt.Errorf("invalid expansion window")
	}
	spec, err := ParseRRule(rule.RRule)
	if err != nil {
		return nil, err
	}
	loc, err := resolveRuleLocation(rule.Timezone)
	if err != nil {
		return nil, err
	}
	base := rule.StartDate.In(loc)
	startLimit := from.In(loc)
	endLimit := to.In(loc)
	var windowEnd *time.Time
	if rule.EndDate != nil {
		v := rule.EndDate.In(loc)
		windowEnd = &v
	}

	var starts []time.Time
	switch spec.Freq {
	case "DAILY":
		starts = expandDaily(base, spec.Interval, startLimit, endLimit, windowEnd)
	case "WEEKLY":
		days := spec.ByDays
		if len(days) == 0 {
			days = []time.Weekday{base.Weekday()}
		}
		starts = expandWeekly(base, spec.Interval, days, startLimit, endLimit, windowEnd)
	case "MONTHLY":
		day := spec.ByMonthDay
		if day == 0 {
			day = base.Day()
		}
		starts = expandMonthly(base, spec.Interval, day, startLimit, endLimit, windowEnd)
	default:
		return nil, fmt.Errorf("unsupported frequency: %s", spec.Freq)
	}

	out := make([]Event, 0, len(starts))
	for _, start := range starts {
		end := start.Add(time.Duration(rule.DurationSec) * time.Second)
		if !end.After(from) || !start.Before(to) {
			continue
		}
		rid := rule.ID
		out = append(out, Event{
			Kind:             rule.Kind,
			Title:            rule.Title,
			Domain:           rule.Domain,
			Subtopic:         rule.Subtopic,
			StartTime:        start,
			EndTime:          end,
			DurationSec:      rule.DurationSec,
			Layer:            "planned",
			Status:           "planned",
			Source:           "recurring",
			RecurrenceRuleID: &rid,
		})
	}
	return out, nil
}

func GenerateRecurringEventsInWindow(ctx context.Context, from, to time.Time, onlyRuleID int64) (RecurrenceGenerationResult, error) {
	if !to.After(from) {
		return RecurrenceGenerationResult{}, fmt.Errorf("invalid generation window")
	}
	query := `
		SELECT id, title, domain, subtopic, kind, duration, rrule, timezone, start_date, end_date, active
		FROM recurrence_rules
		WHERE active = 1`
	args := []any{}
	if onlyRuleID > 0 {
		query += ` AND id = ?`
		args = append(args, onlyRuleID)
	}
	query += ` ORDER BY id ASC`
	rows, err := db.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return RecurrenceGenerationResult{}, err
	}
	var rules []RecurrenceRule
	for rows.Next() {
		rule, err := scanRecurrenceRule(rows)
		if err != nil {
			_ = rows.Close()
			return RecurrenceGenerationResult{}, err
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return RecurrenceGenerationResult{}, err
	}
	_ = rows.Close()

	now := time.Now()
	result := RecurrenceGenerationResult{}
	for _, rule := range rules {
		result.RulesProcessed++
		occurrences, err := ExpandRecurrenceRuleInWindow(rule, from, to)
		if err != nil {
			return RecurrenceGenerationResult{}, err
		}
		for _, occurrence := range occurrences {
			res, err := db.DB.ExecContext(ctx, `
				INSERT OR IGNORE INTO events(
					kind, title, domain, subtopic, description,
					start_time, end_time, duration, timezone,
					layer, status, source, recurrence_rule_id,
					created_at, updated_at
				)
				VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				occurrence.Kind, occurrence.Title, occurrence.Domain, occurrence.Subtopic, occurrence.Description,
				occurrence.StartTime, occurrence.EndTime, occurrence.DurationSec, rule.Timezone,
				occurrence.Layer, occurrence.Status, occurrence.Source, rule.ID,
				now, now,
			)
			if err != nil {
				return RecurrenceGenerationResult{}, err
			}
			affected, err := res.RowsAffected()
			if err != nil {
				return RecurrenceGenerationResult{}, err
			}
			if affected == 1 {
				result.Generated++
			} else {
				result.Skipped++
			}
		}
	}
	return result, nil
}

func normalizeAndValidateRecurrenceRule(rule *RecurrenceRule) error {
	rule.Kind = strings.ToLower(strings.TrimSpace(rule.Kind))
	if rule.Kind == "" {
		rule.Kind = "task"
	}
	if _, ok := allowedKinds[rule.Kind]; !ok {
		return fmt.Errorf("invalid kind: %s", rule.Kind)
	}
	rule.Title = strings.TrimSpace(rule.Title)
	if rule.Title == "" {
		return fmt.Errorf("title is required")
	}
	if rule.DurationSec <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	if rule.StartDate.IsZero() {
		return fmt.Errorf("start date is required")
	}
	if rule.EndDate != nil && !rule.EndDate.After(rule.StartDate) {
		return fmt.Errorf("end date must be after start date")
	}
	path, err := topics.ParseParts(rule.Domain, rule.Subtopic)
	if err != nil {
		return err
	}
	rule.Domain = path.Domain
	rule.Subtopic = path.Subtopic
	if strings.TrimSpace(rule.Timezone) == "" {
		rule.Timezone = "Local"
	}
	loc, err := resolveRuleLocation(rule.Timezone)
	if err != nil {
		return err
	}
	spec, err := ParseRRule(rule.RRule)
	if err != nil {
		return err
	}
	if spec.Freq == "MONTHLY" && spec.ByMonthDay == 0 {
		spec.ByMonthDay = rule.StartDate.In(loc).Day()
	}
	if spec.Freq == "WEEKLY" && len(spec.ByDays) == 0 {
		spec.ByDays = []time.Weekday{rule.StartDate.In(loc).Weekday()}
	}
	normalized, err := BuildRRule(spec)
	if err != nil {
		return err
	}
	rule.RRule = normalized
	return nil
}

func resolveRuleLocation(name string) (*time.Location, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, "Local") {
		return time.Local, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %q: %w", name, err)
	}
	return loc, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRecurrenceRule(s scanner) (RecurrenceRule, error) {
	var rule RecurrenceRule
	var end sql.NullTime
	var active int
	err := s.Scan(&rule.ID, &rule.Title, &rule.Domain, &rule.Subtopic, &rule.Kind, &rule.DurationSec, &rule.RRule, &rule.Timezone, &rule.StartDate, &end, &active)
	if err != nil {
		return RecurrenceRule{}, err
	}
	rule.Active = active == 1
	if end.Valid {
		v := end.Time
		rule.EndDate = &v
	}
	return rule, nil
}

func parseByDays(raw string) ([]time.Weekday, error) {
	parts := strings.Split(raw, ",")
	set := map[time.Weekday]struct{}{}
	for _, part := range parts {
		token := strings.ToUpper(strings.TrimSpace(part))
		if token == "" {
			continue
		}
		day, ok := dayTokenToWeekday(token)
		if !ok {
			return nil, fmt.Errorf("invalid byday token: %s", token)
		}
		set[day] = struct{}{}
	}
	if len(set) == 0 {
		return nil, nil
	}
	out := make([]time.Weekday, 0, len(set))
	for day := range set {
		out = append(out, day)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func formatByDays(days []time.Weekday) string {
	if len(days) == 0 {
		return ""
	}
	uniq := map[time.Weekday]struct{}{}
	for _, day := range days {
		uniq[day] = struct{}{}
	}
	var ordered []time.Weekday
	for day := range uniq {
		ordered = append(ordered, day)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })

	out := make([]string, 0, len(ordered))
	for _, day := range ordered {
		out = append(out, weekdayToToken(day))
	}
	return strings.Join(out, ",")
}

func dayTokenToWeekday(token string) (time.Weekday, bool) {
	switch token {
	case "SU":
		return time.Sunday, true
	case "MO":
		return time.Monday, true
	case "TU":
		return time.Tuesday, true
	case "WE":
		return time.Wednesday, true
	case "TH":
		return time.Thursday, true
	case "FR":
		return time.Friday, true
	case "SA":
		return time.Saturday, true
	default:
		return 0, false
	}
}

func weekdayToToken(day time.Weekday) string {
	switch day {
	case time.Sunday:
		return "SU"
	case time.Monday:
		return "MO"
	case time.Tuesday:
		return "TU"
	case time.Wednesday:
		return "WE"
	case time.Thursday:
		return "TH"
	case time.Friday:
		return "FR"
	case time.Saturday:
		return "SA"
	default:
		return "MO"
	}
}

func expandDaily(base time.Time, interval int, from, to time.Time, until *time.Time) []time.Time {
	step := interval
	if step <= 0 {
		step = 1
	}
	cursor := base
	for cursor.Before(from) {
		cursor = cursor.AddDate(0, 0, step)
	}
	var out []time.Time
	for cursor.Before(to) {
		if until != nil && cursor.After(*until) {
			break
		}
		out = append(out, cursor)
		cursor = cursor.AddDate(0, 0, step)
	}
	return out
}

func expandWeekly(base time.Time, interval int, byDays []time.Weekday, from, to time.Time, until *time.Time) []time.Time {
	step := interval
	if step <= 0 {
		step = 1
	}
	daySet := map[time.Weekday]struct{}{}
	for _, d := range byDays {
		daySet[d] = struct{}{}
	}

	startDay := floorDate(base)
	cursor := floorDate(from)
	if cursor.Before(startDay) {
		cursor = startDay
	}
	var out []time.Time
	for cursor.Before(to) {
		if _, ok := daySet[cursor.Weekday()]; ok {
			weekDelta := int(cursor.Sub(startDay).Hours()) / (24 * 7)
			if weekDelta >= 0 && weekDelta%step == 0 {
				candidate := time.Date(cursor.Year(), cursor.Month(), cursor.Day(), base.Hour(), base.Minute(), base.Second(), base.Nanosecond(), base.Location())
				if !candidate.Before(base) && !candidate.Before(from) {
					if until == nil || !candidate.After(*until) {
						out = append(out, candidate)
					}
				}
			}
		}
		cursor = cursor.AddDate(0, 0, 1)
	}
	return out
}

func expandMonthly(base time.Time, interval int, day int, from, to time.Time, until *time.Time) []time.Time {
	step := interval
	if step <= 0 {
		step = 1
	}
	if day < 1 {
		day = 1
	}
	if day > 31 {
		day = 31
	}
	cursor := time.Date(base.Year(), base.Month(), 1, base.Hour(), base.Minute(), base.Second(), base.Nanosecond(), base.Location())
	for cursor.Before(from) {
		cursor = cursor.AddDate(0, step, 0)
	}
	var out []time.Time
	for cursor.Before(to) {
		last := daysInMonth(cursor.Year(), cursor.Month(), cursor.Location())
		d := day
		if d > last {
			d = last
		}
		candidate := time.Date(cursor.Year(), cursor.Month(), d, base.Hour(), base.Minute(), base.Second(), base.Nanosecond(), base.Location())
		if !candidate.Before(base) && !candidate.Before(from) {
			if until == nil || !candidate.After(*until) {
				out = append(out, candidate)
			}
		}
		cursor = cursor.AddDate(0, step, 0)
	}
	return out
}

func daysInMonth(year int, month time.Month, loc *time.Location) int {
	firstNext := time.Date(year, month+1, 1, 0, 0, 0, 0, loc)
	last := firstNext.AddDate(0, 0, -1)
	return last.Day()
}

func floorDate(v time.Time) time.Time {
	return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, v.Location())
}
