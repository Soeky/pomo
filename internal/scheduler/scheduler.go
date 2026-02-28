package scheduler

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/events"
	"github.com/Soeky/pomo/internal/topics"
)

type Input struct {
	From        time.Time
	To          time.Time
	Constraints ConstraintConfig
	Targets     []WorkloadTarget
	Existing    []ExistingEvent
}

type PlannedEvent struct {
	Title            string
	Kind             string
	Domain           string
	Subtopic         string
	StartTime        time.Time
	EndTime          time.Time
	Source           string
	Status           string
	Dependency       []int64
	WorkloadTargetID int64
}

type Result struct {
	Generated   []PlannedEvent
	Blocked     []PlannedEvent
	Diagnostics []Diagnostic
	Summaries   []TargetSummary
	RunID       int64
}

type Diagnostic struct {
	TargetID       int64
	Severity       string
	Code           string
	Message        string
	MissingSeconds int
}

type WorkloadTarget struct {
	ID                int64
	Title             string
	Domain            string
	Subtopic          string
	Cadence           string
	TargetSeconds     int
	TargetOccurrences int
	Active            bool
	Kind              string
}

type ExistingEvent struct {
	ID               int64
	Title            string
	Kind             string
	Domain           string
	Subtopic         string
	StartTime        time.Time
	EndTime          time.Time
	Source           string
	Status           string
	WorkloadTargetID *int64
}

type TargetSummary struct {
	TargetID                 int64
	Title                    string
	Domain                   string
	Subtopic                 string
	Cadence                  string
	RequiredSeconds          int
	FixedDeductionSeconds    int
	SchedulerExistingSeconds int
	GeneratedSeconds         int
	RemainingSeconds         int
}

type ConstraintConfig struct {
	ActiveWeekdays        []string `json:"active_weekdays"`
	DayStart              string   `json:"day_start"`
	DayEnd                string   `json:"day_end"`
	LunchStart            string   `json:"lunch_start"`
	LunchDurationMinutes  int      `json:"lunch_duration_minutes"`
	DinnerStart           string   `json:"dinner_start"`
	DinnerDurationMinutes int      `json:"dinner_duration_minutes"`
	MaxHoursPerDay        int      `json:"max_hours_per_day"`
	Timezone              string   `json:"timezone"`
}

type DBInput struct {
	From                     time.Time
	To                       time.Time
	Persist                  bool
	ReplaceExistingScheduler bool
}

type Engine interface {
	Generate(ctx context.Context, in Input) (Result, error)
}

type BalancedEngine struct{}

func (BalancedEngine) Generate(ctx context.Context, in Input) (Result, error) {
	return Generate(ctx, in)
}

const (
	constraintKeyBalancedV1 = "balanced_v1"
	defaultMaxHoursPerDay   = 8
	defaultChunkSeconds     = 3600
	minChunkSeconds         = 900
)

type compiledConstraints struct {
	location          *time.Location
	activeWeekdays    []time.Weekday
	activeWeekdaySet  map[time.Weekday]struct{}
	dayStartMinute    int
	dayEndMinute      int
	lunchStartMinute  int
	lunchDurationSec  int
	dinnerStartMinute int
	dinnerDurationSec int
	maxHoursPerDay    int
}

type interval struct {
	start time.Time
	end   time.Time
}

type dayState struct {
	day             time.Time
	freeIntervals   []interval
	remainingCapSec int
}

type targetState struct {
	target    WorkloadTarget
	summary   *TargetSummary
	remaining int
	generated int
	nextDay   int
	chunkSec  int
}

func DefaultConstraintConfig() ConstraintConfig {
	cfg := ConstraintConfig{
		ActiveWeekdays:        append([]string(nil), config.AppConfig.ActiveWeekdays...),
		DayStart:              strings.TrimSpace(config.AppConfig.DayStart),
		DayEnd:                strings.TrimSpace(config.AppConfig.DayEnd),
		LunchStart:            strings.TrimSpace(config.AppConfig.LunchStart),
		LunchDurationMinutes:  config.AppConfig.LunchDurationMinutes,
		DinnerStart:           strings.TrimSpace(config.AppConfig.DinnerStart),
		DinnerDurationMinutes: config.AppConfig.DinnerDurationMinutes,
		MaxHoursPerDay:        defaultMaxHoursPerDay,
		Timezone:              "Local",
	}

	if len(cfg.ActiveWeekdays) == 0 {
		cfg.ActiveWeekdays = []string{"mon", "tue", "wed", "thu", "fri"}
	}
	if cfg.DayStart == "" {
		cfg.DayStart = "08:00"
	}
	if cfg.DayEnd == "" {
		cfg.DayEnd = "22:00"
	}
	if cfg.LunchStart == "" {
		cfg.LunchStart = "12:30"
	}
	if cfg.LunchDurationMinutes <= 0 {
		cfg.LunchDurationMinutes = 60
	}
	if cfg.DinnerStart == "" {
		cfg.DinnerStart = "19:00"
	}
	if cfg.DinnerDurationMinutes <= 0 {
		cfg.DinnerDurationMinutes = 60
	}
	if cfg.MaxHoursPerDay <= 0 {
		cfg.MaxHoursPerDay = defaultMaxHoursPerDay
	}
	return cfg
}

func LoadConstraintConfig(ctx context.Context) (ConstraintConfig, error) {
	if db.DB == nil {
		return ConstraintConfig{}, fmt.Errorf("database is not initialized")
	}

	var raw string
	err := db.DB.QueryRowContext(ctx, `SELECT value_json FROM schedule_constraints WHERE key = ?`, constraintKeyBalancedV1).Scan(&raw)
	if err == sql.ErrNoRows {
		cfg, normalizeErr := normalizeConstraintConfig(DefaultConstraintConfig())
		if normalizeErr != nil {
			return ConstraintConfig{}, normalizeErr
		}
		return cfg, nil
	}
	if err != nil {
		return ConstraintConfig{}, err
	}

	var cfg ConstraintConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return ConstraintConfig{}, err
	}
	return normalizeConstraintConfig(cfg)
}

func SaveConstraintConfig(ctx context.Context, cfg ConstraintConfig) error {
	if db.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	normalized, err := normalizeConstraintConfig(cfg)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	now := time.Now()
	_, err = db.DB.ExecContext(ctx, `
		INSERT INTO schedule_constraints(key, value_json, created_at, updated_at)
		VALUES(?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at`,
		constraintKeyBalancedV1, string(payload), now, now)
	return err
}

func ListWorkloadTargets(ctx context.Context, activeOnly bool) ([]WorkloadTarget, error) {
	if db.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	query := `
		SELECT id, title, domain, subtopic, cadence, target_seconds, target_occurrences, active
		FROM workload_targets`
	if activeOnly {
		query += ` WHERE active = 1`
	}
	query += ` ORDER BY domain ASC, subtopic ASC, cadence ASC, id ASC`

	rows, err := db.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WorkloadTarget
	for rows.Next() {
		var t WorkloadTarget
		var active int
		if err := rows.Scan(&t.ID, &t.Title, &t.Domain, &t.Subtopic, &t.Cadence, &t.TargetSeconds, &t.TargetOccurrences, &active); err != nil {
			return nil, err
		}
		t.Active = active == 1
		t.Kind = "task"
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func CreateWorkloadTarget(ctx context.Context, t WorkloadTarget) (int64, error) {
	if db.DB == nil {
		return 0, fmt.Errorf("database is not initialized")
	}
	normalized, err := normalizeTarget(t)
	if err != nil {
		return 0, err
	}

	active := 0
	if normalized.Active {
		active = 1
	}
	now := time.Now()
	res, err := db.DB.ExecContext(ctx, `
		INSERT INTO workload_targets(title, domain, subtopic, cadence, target_seconds, target_occurrences, active, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		normalized.Title,
		normalized.Domain,
		normalized.Subtopic,
		normalized.Cadence,
		normalized.TargetSeconds,
		normalized.TargetOccurrences,
		active,
		now,
		now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func DeleteWorkloadTarget(ctx context.Context, id int64) error {
	if db.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	res, err := db.DB.ExecContext(ctx, `DELETE FROM workload_targets WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func SetWorkloadTargetActive(ctx context.Context, id int64, active bool) error {
	if db.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	value := 0
	if active {
		value = 1
	}
	res, err := db.DB.ExecContext(ctx, `UPDATE workload_targets SET active = ?, updated_at = ? WHERE id = ?`, value, time.Now(), id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func GenerateFromDB(ctx context.Context, in DBInput) (Result, error) {
	if db.DB == nil {
		return Result{}, fmt.Errorf("database is not initialized")
	}
	if !in.To.After(in.From) {
		return Result{}, fmt.Errorf("invalid scheduling window")
	}

	constraints, err := LoadConstraintConfig(ctx)
	if err != nil {
		return Result{}, err
	}
	targets, err := ListWorkloadTargets(ctx, true)
	if err != nil {
		return Result{}, err
	}

	includeScheduler := !in.ReplaceExistingScheduler
	existing, err := listEventsInRange(ctx, in.From, in.To, includeScheduler)
	if err != nil {
		return Result{}, err
	}

	result, err := Generate(ctx, Input{
		From:        in.From,
		To:          in.To,
		Constraints: constraints,
		Targets:     targets,
		Existing:    existing,
	})
	if err != nil {
		return Result{}, err
	}
	if !in.Persist {
		return result, nil
	}
	return persistResult(ctx, in, constraints, targets, result)
}

func Generate(_ context.Context, in Input) (Result, error) {
	if !in.To.After(in.From) {
		return Result{}, fmt.Errorf("invalid scheduling window")
	}

	normalizedConstraints, err := normalizeConstraintConfig(in.Constraints)
	if err != nil {
		return Result{}, err
	}
	compiled, err := compileConstraints(normalizedConstraints)
	if err != nil {
		return Result{}, err
	}

	targets := make([]WorkloadTarget, 0, len(in.Targets))
	for _, target := range in.Targets {
		if !target.Active {
			continue
		}
		normalized, err := normalizeTarget(target)
		if err != nil {
			return Result{}, err
		}
		targets = append(targets, normalized)
	}
	sortTargets(targets)

	existing := normalizeExistingEvents(in.Existing)
	days := buildDayStates(in.From.In(compiled.location), in.To.In(compiled.location), compiled, existing)
	activeDates := collectActiveDates(in.From.In(compiled.location), in.To.In(compiled.location), compiled)

	result := Result{}
	if len(targets) == 0 {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{
			Severity: "warning",
			Code:     "no_targets",
			Message:  "no active workload targets configured",
		})
		return result, nil
	}

	targetStates := make([]*targetState, 0, len(targets))
	for i := range targets {
		target := targets[i]
		required := requiredSeconds(target, activeDates)
		fixedDeduction := deductionSeconds(existing, in.From, in.To, target, true)
		schedulerExisting := deductionSeconds(existing, in.From, in.To, target, false)
		remaining := required - fixedDeduction - schedulerExisting
		if remaining < 0 {
			remaining = 0
		}
		summary := TargetSummary{
			TargetID:                 target.ID,
			Title:                    target.Title,
			Domain:                   target.Domain,
			Subtopic:                 target.Subtopic,
			Cadence:                  target.Cadence,
			RequiredSeconds:          required,
			FixedDeductionSeconds:    fixedDeduction,
			SchedulerExistingSeconds: schedulerExisting,
			GeneratedSeconds:         0,
			RemainingSeconds:         remaining,
		}
		result.Summaries = append(result.Summaries, summary)

		if remaining == 0 {
			continue
		}
		chunkSec := defaultChunkSeconds
		if target.TargetOccurrences > 0 && target.TargetSeconds > 0 {
			chunkSec = target.TargetSeconds
		}
		targetStates = append(targetStates, &targetState{
			target:    target,
			summary:   &result.Summaries[len(result.Summaries)-1],
			remaining: remaining,
			generated: 0,
			nextDay:   0,
			chunkSec:  chunkSec,
		})
	}

	if len(days) == 0 && len(targetStates) > 0 {
		for _, state := range targetStates {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				TargetID:       state.target.ID,
				Severity:       "error",
				Code:           "no_active_days",
				Message:        "no active days available in scheduling window",
				MissingSeconds: state.remaining,
			})
			state.summary.RemainingSeconds = state.remaining
		}
		return result, nil
	}

	for {
		progress := false
		allDone := true
		for _, state := range targetStates {
			if state.remaining <= 0 {
				continue
			}
			allDone = false
			baseChunk := state.chunkSec
			if baseChunk <= 0 {
				baseChunk = defaultChunkSeconds
			}
			if baseChunk > state.remaining {
				baseChunk = state.remaining
			}

			allocated := false
			tryChunk := baseChunk
			for tryChunk >= minChunkSeconds {
				if tryChunk > state.remaining {
					tryChunk = state.remaining
				}
				if allocateChunk(state, days, tryChunk, &result.Generated) {
					state.remaining -= tryChunk
					state.generated += tryChunk
					state.summary.GeneratedSeconds += tryChunk
					allocated = true
					progress = true
					break
				}
				if tryChunk == minChunkSeconds {
					break
				}
				tryChunk /= 2
				if tryChunk < minChunkSeconds {
					tryChunk = minChunkSeconds
				}
			}
			if !allocated && state.remaining < minChunkSeconds && state.remaining > 0 {
				if allocateChunk(state, days, state.remaining, &result.Generated) {
					state.summary.GeneratedSeconds += state.remaining
					state.generated += state.remaining
					state.remaining = 0
					progress = true
				}
			}
		}
		if allDone || !progress {
			break
		}
	}

	for _, state := range targetStates {
		state.summary.RemainingSeconds = state.remaining
		if state.remaining > 0 {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				TargetID:       state.target.ID,
				Severity:       "error",
				Code:           "insufficient_capacity",
				Message:        fmt.Sprintf("insufficient capacity for target %s::%s (%s)", state.target.Domain, state.target.Subtopic, state.target.Cadence),
				MissingSeconds: state.remaining,
			})
		}
	}

	sort.Slice(result.Generated, func(i, j int) bool {
		if !result.Generated[i].StartTime.Equal(result.Generated[j].StartTime) {
			return result.Generated[i].StartTime.Before(result.Generated[j].StartTime)
		}
		if result.Generated[i].WorkloadTargetID != result.Generated[j].WorkloadTargetID {
			return result.Generated[i].WorkloadTargetID < result.Generated[j].WorkloadTargetID
		}
		return result.Generated[i].Title < result.Generated[j].Title
	})
	return result, nil
}

func persistResult(ctx context.Context, in DBInput, constraints ConstraintConfig, targets []WorkloadTarget, result Result) (Result, error) {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now()
	hash := buildInputHash(in, constraints, targets)
	res, err := tx.ExecContext(ctx, `INSERT INTO schedule_runs(started_at, status, input_hash) VALUES (?, 'running', ?)`, now, hash)
	if err != nil {
		return Result{}, err
	}
	runID, err := res.LastInsertId()
	if err != nil {
		return Result{}, err
	}

	if in.ReplaceExistingScheduler {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM events
			WHERE source = 'scheduler'
			  AND start_time < ?
			  AND end_time > ?`, in.To, in.From); err != nil {
			return Result{}, err
		}
	}

	for _, planned := range result.Generated {
		duration := int(planned.EndTime.Sub(planned.StartTime).Seconds())
		if duration <= 0 {
			continue
		}
		inserted, err := tx.ExecContext(ctx, `
			INSERT INTO events(
				kind, title, domain, subtopic, description,
				start_time, end_time, duration, timezone,
				layer, status, source, workload_target_id,
				created_at, updated_at
			)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			planned.Kind,
			planned.Title,
			planned.Domain,
			planned.Subtopic,
			"",
			planned.StartTime,
			planned.EndTime,
			duration,
			"Local",
			"planned",
			planned.Status,
			planned.Source,
			planned.WorkloadTargetID,
			now,
			now,
		)
		if err != nil {
			return Result{}, err
		}
		eventID, err := inserted.LastInsertId()
		if err != nil {
			return Result{}, err
		}
		detailsJSON, err := json.Marshal(map[string]any{
			"target_id": planned.WorkloadTargetID,
			"title":     planned.Title,
		})
		if err != nil {
			return Result{}, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO schedule_run_events(run_id, event_id, action, details_json, created_at)
			VALUES(?, ?, 'create', ?, ?)`,
			runID, eventID, string(detailsJSON), now); err != nil {
			return Result{}, err
		}
	}

	dependencyTransitions, err := events.ReconcileBlockedStatusesInWindowTx(ctx, tx, in.From, in.To)
	if err != nil {
		return Result{}, err
	}
	for _, transition := range dependencyTransitions {
		detailsJSON, err := json.Marshal(map[string]any{
			"old_status":   transition.OldStatus,
			"new_status":   transition.NewStatus,
			"reason":       transition.Reason,
			"triggered_by": transition.TriggeredBy,
		})
		if err != nil {
			return Result{}, err
		}
		action := "update"
		if transition.NewStatus == "blocked" {
			action = "block"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO schedule_run_events(run_id, event_id, action, details_json, created_at)
			VALUES(?, ?, ?, ?, ?)`,
			runID, transition.EventID, action, string(detailsJSON), now); err != nil {
			return Result{}, err
		}
		if transition.NewStatus == "blocked" {
			blockedEvent, err := loadPlannedEventByIDTx(ctx, tx, transition.EventID)
			if err != nil {
				return Result{}, err
			}
			blockedEvent.Status = "blocked"
			result.Blocked = append(result.Blocked, blockedEvent)
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Severity: "warning",
				Code:     "dependency_blocked",
				Message:  fmt.Sprintf("event %d blocked: %s", transition.EventID, transition.Reason),
			})
		}
	}

	status := "success"
	errorParts := make([]string, 0)
	for _, diag := range result.Diagnostics {
		if strings.EqualFold(diag.Severity, "error") {
			status = "failed"
			errorParts = append(errorParts, diag.Message)
		}
	}
	errText := ""
	if len(errorParts) > 0 {
		errText = strings.Join(errorParts, "; ")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE schedule_runs
		SET finished_at = ?, status = ?, error = ?
		WHERE id = ?`, time.Now(), status, errText, runID); err != nil {
		return Result{}, err
	}

	if err := tx.Commit(); err != nil {
		return Result{}, err
	}
	result.RunID = runID
	return result, nil
}

func loadPlannedEventByIDTx(ctx context.Context, tx *sql.Tx, eventID int64) (PlannedEvent, error) {
	var planned PlannedEvent
	if err := tx.QueryRowContext(ctx, `
		SELECT title, kind, domain, subtopic, start_time, end_time, source, status, COALESCE(workload_target_id, 0)
		FROM events
		WHERE id = ?`, eventID).
		Scan(
			&planned.Title,
			&planned.Kind,
			&planned.Domain,
			&planned.Subtopic,
			&planned.StartTime,
			&planned.EndTime,
			&planned.Source,
			&planned.Status,
			&planned.WorkloadTargetID,
		); err != nil {
		return PlannedEvent{}, err
	}
	return planned, nil
}

func buildInputHash(in DBInput, constraints ConstraintConfig, targets []WorkloadTarget) string {
	payload := struct {
		From                     string           `json:"from"`
		To                       string           `json:"to"`
		Persist                  bool             `json:"persist"`
		ReplaceExistingScheduler bool             `json:"replace_existing_scheduler"`
		Constraints              ConstraintConfig `json:"constraints"`
		Targets                  []WorkloadTarget `json:"targets"`
	}{
		From:                     in.From.Format(time.RFC3339Nano),
		To:                       in.To.Format(time.RFC3339Nano),
		Persist:                  in.Persist,
		ReplaceExistingScheduler: in.ReplaceExistingScheduler,
		Constraints:              constraints,
		Targets:                  targets,
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func listEventsInRange(ctx context.Context, from, to time.Time, includeScheduler bool) ([]ExistingEvent, error) {
	query := `
		SELECT id, title, kind, domain, subtopic, start_time, end_time, source, status, workload_target_id
		FROM events
		WHERE start_time < ?
		  AND end_time > ?
		  AND status != 'canceled'`
	if !includeScheduler {
		query += ` AND source != 'scheduler'`
	}
	query += ` ORDER BY start_time ASC, id ASC`

	rows, err := db.DB.QueryContext(ctx, query, to, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ExistingEvent
	for rows.Next() {
		var event ExistingEvent
		var targetID sql.NullInt64
		if err := rows.Scan(
			&event.ID,
			&event.Title,
			&event.Kind,
			&event.Domain,
			&event.Subtopic,
			&event.StartTime,
			&event.EndTime,
			&event.Source,
			&event.Status,
			&targetID,
		); err != nil {
			return nil, err
		}
		if targetID.Valid {
			v := targetID.Int64
			event.WorkloadTargetID = &v
		}
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func normalizeConstraintConfig(cfg ConstraintConfig) (ConstraintConfig, error) {
	defaults := DefaultConstraintConfig()
	normalized := cfg
	if len(normalized.ActiveWeekdays) == 0 {
		normalized.ActiveWeekdays = defaults.ActiveWeekdays
	}
	if strings.TrimSpace(normalized.DayStart) == "" {
		normalized.DayStart = defaults.DayStart
	}
	if strings.TrimSpace(normalized.DayEnd) == "" {
		normalized.DayEnd = defaults.DayEnd
	}
	if strings.TrimSpace(normalized.LunchStart) == "" {
		normalized.LunchStart = defaults.LunchStart
	}
	if normalized.LunchDurationMinutes <= 0 {
		normalized.LunchDurationMinutes = defaults.LunchDurationMinutes
	}
	if strings.TrimSpace(normalized.DinnerStart) == "" {
		normalized.DinnerStart = defaults.DinnerStart
	}
	if normalized.DinnerDurationMinutes <= 0 {
		normalized.DinnerDurationMinutes = defaults.DinnerDurationMinutes
	}
	if normalized.MaxHoursPerDay <= 0 {
		normalized.MaxHoursPerDay = defaults.MaxHoursPerDay
	}
	if strings.TrimSpace(normalized.Timezone) == "" {
		normalized.Timezone = defaults.Timezone
	}

	weekdayTokens, err := normalizeWeekdayTokens(normalized.ActiveWeekdays)
	if err != nil {
		return ConstraintConfig{}, err
	}
	if len(weekdayTokens) == 0 {
		return ConstraintConfig{}, fmt.Errorf("at least one active weekday is required")
	}
	normalized.ActiveWeekdays = weekdayTokens

	startMinute, err := parseClockMinutes(normalized.DayStart)
	if err != nil {
		return ConstraintConfig{}, fmt.Errorf("invalid day_start: %w", err)
	}
	endMinute, err := parseClockMinutes(normalized.DayEnd)
	if err != nil {
		return ConstraintConfig{}, fmt.Errorf("invalid day_end: %w", err)
	}
	if endMinute <= startMinute {
		return ConstraintConfig{}, fmt.Errorf("day_end must be later than day_start")
	}
	if _, err := parseClockMinutes(normalized.LunchStart); err != nil {
		return ConstraintConfig{}, fmt.Errorf("invalid lunch_start: %w", err)
	}
	if _, err := parseClockMinutes(normalized.DinnerStart); err != nil {
		return ConstraintConfig{}, fmt.Errorf("invalid dinner_start: %w", err)
	}

	if normalized.MaxHoursPerDay <= 0 {
		return ConstraintConfig{}, fmt.Errorf("max_hours_per_day must be positive")
	}
	dayWindowHours := (endMinute - startMinute) / 60
	if normalized.MaxHoursPerDay > dayWindowHours {
		normalized.MaxHoursPerDay = dayWindowHours
	}
	if normalized.MaxHoursPerDay <= 0 {
		normalized.MaxHoursPerDay = 1
	}
	return normalized, nil
}

func compileConstraints(cfg ConstraintConfig) (compiledConstraints, error) {
	location, err := resolveLocation(cfg.Timezone)
	if err != nil {
		return compiledConstraints{}, err
	}
	startMinute, _ := parseClockMinutes(cfg.DayStart)
	endMinute, _ := parseClockMinutes(cfg.DayEnd)
	lunchMinute, _ := parseClockMinutes(cfg.LunchStart)
	dinnerMinute, _ := parseClockMinutes(cfg.DinnerStart)
	weekdays, err := parseWeekdayTokens(cfg.ActiveWeekdays)
	if err != nil {
		return compiledConstraints{}, err
	}
	set := make(map[time.Weekday]struct{}, len(weekdays))
	for _, day := range weekdays {
		set[day] = struct{}{}
	}
	return compiledConstraints{
		location:          location,
		activeWeekdays:    weekdays,
		activeWeekdaySet:  set,
		dayStartMinute:    startMinute,
		dayEndMinute:      endMinute,
		lunchStartMinute:  lunchMinute,
		lunchDurationSec:  cfg.LunchDurationMinutes * 60,
		dinnerStartMinute: dinnerMinute,
		dinnerDurationSec: cfg.DinnerDurationMinutes * 60,
		maxHoursPerDay:    cfg.MaxHoursPerDay,
	}, nil
}

func normalizeTarget(t WorkloadTarget) (WorkloadTarget, error) {
	normalized := t
	path, err := topics.ParseParts(normalized.Domain, normalized.Subtopic)
	if err != nil {
		return WorkloadTarget{}, err
	}
	normalized.Domain = path.Domain
	normalized.Subtopic = path.Subtopic

	normalized.Cadence = strings.ToLower(strings.TrimSpace(normalized.Cadence))
	switch normalized.Cadence {
	case "daily", "weekly", "monthly":
	default:
		return WorkloadTarget{}, fmt.Errorf("invalid cadence: %s", normalized.Cadence)
	}

	if normalized.TargetSeconds < 0 {
		return WorkloadTarget{}, fmt.Errorf("target_seconds must be non-negative")
	}
	if normalized.TargetOccurrences < 0 {
		return WorkloadTarget{}, fmt.Errorf("target_occurrences must be non-negative")
	}
	if normalized.TargetSeconds == 0 && normalized.TargetOccurrences == 0 {
		return WorkloadTarget{}, fmt.Errorf("target must set duration and/or occurrences")
	}
	if normalized.TargetOccurrences > 0 && normalized.TargetSeconds == 0 {
		normalized.TargetSeconds = defaultChunkSeconds
	}
	if strings.TrimSpace(normalized.Title) == "" {
		normalized.Title = fmt.Sprintf("%s::%s", normalized.Domain, normalized.Subtopic)
	}
	if strings.TrimSpace(normalized.Kind) == "" {
		normalized.Kind = "task"
	}
	return normalized, nil
}

func sortTargets(targets []WorkloadTarget) {
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Domain != targets[j].Domain {
			return targets[i].Domain < targets[j].Domain
		}
		if targets[i].Subtopic != targets[j].Subtopic {
			return targets[i].Subtopic < targets[j].Subtopic
		}
		if targets[i].Cadence != targets[j].Cadence {
			return targets[i].Cadence < targets[j].Cadence
		}
		if targets[i].Title != targets[j].Title {
			return targets[i].Title < targets[j].Title
		}
		return targets[i].ID < targets[j].ID
	})
}

func normalizeExistingEvents(events []ExistingEvent) []ExistingEvent {
	out := make([]ExistingEvent, 0, len(events))
	for _, event := range events {
		if !event.EndTime.After(event.StartTime) {
			continue
		}
		event.Source = strings.ToLower(strings.TrimSpace(event.Source))
		event.Status = strings.ToLower(strings.TrimSpace(event.Status))
		if event.Status == "canceled" {
			continue
		}
		if strings.TrimSpace(event.Domain) == "" {
			event.Domain = topics.DefaultDomain
		}
		if strings.TrimSpace(event.Subtopic) == "" {
			event.Subtopic = topics.DefaultSubtopic
		}
		if strings.TrimSpace(event.Title) == "" {
			event.Title = fmt.Sprintf("%s::%s", event.Domain, event.Subtopic)
		}
		if strings.TrimSpace(event.Kind) == "" {
			event.Kind = "task"
		}
		out = append(out, event)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].StartTime.Equal(out[j].StartTime) {
			return out[i].StartTime.Before(out[j].StartTime)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func buildDayStates(from, to time.Time, constraints compiledConstraints, existing []ExistingEvent) []dayState {
	dayCursor := startOfDay(from, constraints.location)
	endDay := startOfDay(to, constraints.location)
	if endDay.Before(to) {
		endDay = endDay.AddDate(0, 0, 1)
	}

	var days []dayState
	for dayCursor.Before(endDay) {
		if _, active := constraints.activeWeekdaySet[dayCursor.Weekday()]; !active {
			dayCursor = dayCursor.AddDate(0, 0, 1)
			continue
		}

		dayWindowStart := dayCursor.Add(time.Duration(constraints.dayStartMinute) * time.Minute)
		dayWindowEnd := dayCursor.Add(time.Duration(constraints.dayEndMinute) * time.Minute)
		if dayWindowEnd.Before(from) || !dayWindowStart.Before(to) {
			dayCursor = dayCursor.AddDate(0, 0, 1)
			continue
		}
		windowStart := maxTime(dayWindowStart, from)
		windowEnd := minTime(dayWindowEnd, to)
		if !windowEnd.After(windowStart) {
			dayCursor = dayCursor.AddDate(0, 0, 1)
			continue
		}

		freeIntervals := []interval{{start: windowStart, end: windowEnd}}
		if constraints.lunchDurationSec > 0 {
			mealStart := dayCursor.Add(time.Duration(constraints.lunchStartMinute) * time.Minute)
			mealEnd := mealStart.Add(time.Duration(constraints.lunchDurationSec) * time.Second)
			freeIntervals = subtractIntervals(freeIntervals, interval{start: mealStart, end: mealEnd})
		}
		if constraints.dinnerDurationSec > 0 {
			mealStart := dayCursor.Add(time.Duration(constraints.dinnerStartMinute) * time.Minute)
			mealEnd := mealStart.Add(time.Duration(constraints.dinnerDurationSec) * time.Second)
			freeIntervals = subtractIntervals(freeIntervals, interval{start: mealStart, end: mealEnd})
		}

		occupiedSec := 0
		for _, commitment := range existing {
			if commitment.EndTime.Before(windowStart) || !commitment.StartTime.Before(windowEnd) {
				continue
			}
			overlap := overlapInterval(
				interval{start: windowStart, end: windowEnd},
				interval{start: commitment.StartTime.In(constraints.location), end: commitment.EndTime.In(constraints.location)},
			)
			if overlap <= 0 {
				continue
			}
			occupiedSec += overlap
			freeIntervals = subtractIntervals(freeIntervals, interval{
				start: commitment.StartTime.In(constraints.location),
				end:   commitment.EndTime.In(constraints.location),
			})
		}

		remainingCap := constraints.maxHoursPerDay*3600 - occupiedSec
		if remainingCap < 0 {
			remainingCap = 0
		}
		freeSec := intervalsSeconds(freeIntervals)
		if remainingCap > freeSec {
			remainingCap = freeSec
		}
		days = append(days, dayState{
			day:             dayCursor,
			freeIntervals:   freeIntervals,
			remainingCapSec: remainingCap,
		})
		dayCursor = dayCursor.AddDate(0, 0, 1)
	}
	return days
}

func collectActiveDates(from, to time.Time, constraints compiledConstraints) []time.Time {
	dayCursor := startOfDay(from, constraints.location)
	endDay := startOfDay(to, constraints.location)
	if endDay.Before(to) {
		endDay = endDay.AddDate(0, 0, 1)
	}
	var dates []time.Time
	for dayCursor.Before(endDay) {
		if _, active := constraints.activeWeekdaySet[dayCursor.Weekday()]; !active {
			dayCursor = dayCursor.AddDate(0, 0, 1)
			continue
		}
		dayStart := dayCursor.Add(time.Duration(constraints.dayStartMinute) * time.Minute)
		dayEnd := dayCursor.Add(time.Duration(constraints.dayEndMinute) * time.Minute)
		if dayEnd.After(from) && dayStart.Before(to) {
			dates = append(dates, dayCursor)
		}
		dayCursor = dayCursor.AddDate(0, 0, 1)
	}
	return dates
}

func requiredSeconds(target WorkloadTarget, activeDates []time.Time) int {
	units := cadenceUnits(target.Cadence, activeDates)
	if units <= 0 {
		return 0
	}
	if target.TargetOccurrences > 0 {
		return target.TargetOccurrences * target.TargetSeconds * units
	}
	return target.TargetSeconds * units
}

func cadenceUnits(cadence string, activeDates []time.Time) int {
	if len(activeDates) == 0 {
		return 0
	}
	switch cadence {
	case "daily":
		return len(activeDates)
	case "weekly":
		weeks := make(map[string]struct{}, len(activeDates))
		for _, date := range activeDates {
			year, week := date.ISOWeek()
			key := strconv.Itoa(year) + "-" + strconv.Itoa(week)
			weeks[key] = struct{}{}
		}
		return len(weeks)
	case "monthly":
		months := make(map[string]struct{}, len(activeDates))
		for _, date := range activeDates {
			key := strconv.Itoa(date.Year()) + "-" + strconv.Itoa(int(date.Month()))
			months[key] = struct{}{}
		}
		return len(months)
	default:
		return 0
	}
}

func deductionSeconds(existing []ExistingEvent, from, to time.Time, target WorkloadTarget, onlyFixed bool) int {
	total := 0
	for _, event := range existing {
		if !event.EndTime.After(from) || !event.StartTime.Before(to) {
			continue
		}
		isScheduler := strings.EqualFold(event.Source, "scheduler")
		if onlyFixed && isScheduler {
			continue
		}
		if !onlyFixed {
			if !isScheduler || event.WorkloadTargetID == nil || *event.WorkloadTargetID != target.ID {
				continue
			}
		} else if !topicsEqual(event.Domain, event.Subtopic, target.Domain, target.Subtopic) {
			continue
		}

		overlap := overlapInterval(
			interval{start: from, end: to},
			interval{start: event.StartTime, end: event.EndTime},
		)
		if overlap > 0 {
			total += overlap
		}
	}
	return total
}

func allocateChunk(state *targetState, days []dayState, chunkSec int, generated *[]PlannedEvent) bool {
	if chunkSec <= 0 || len(days) == 0 {
		return false
	}
	for i := 0; i < len(days); i++ {
		index := (state.nextDay + i) % len(days)
		day := &days[index]
		if day.remainingCapSec < chunkSec {
			continue
		}
		start, end, ok := reserveInterval(day, chunkSec)
		if !ok {
			continue
		}
		day.remainingCapSec -= chunkSec
		state.nextDay = (index + 1) % len(days)
		*generated = append(*generated, PlannedEvent{
			Title:            state.target.Title,
			Kind:             state.target.Kind,
			Domain:           state.target.Domain,
			Subtopic:         state.target.Subtopic,
			StartTime:        start,
			EndTime:          end,
			Source:           "scheduler",
			Status:           "planned",
			WorkloadTargetID: state.target.ID,
		})
		return true
	}
	return false
}

func reserveInterval(day *dayState, seconds int) (time.Time, time.Time, bool) {
	for i := range day.freeIntervals {
		slot := day.freeIntervals[i]
		length := int(slot.end.Sub(slot.start).Seconds())
		if length < seconds {
			continue
		}
		start := slot.start
		end := start.Add(time.Duration(seconds) * time.Second)
		if end.Equal(slot.end) {
			day.freeIntervals = append(day.freeIntervals[:i], day.freeIntervals[i+1:]...)
		} else {
			day.freeIntervals[i].start = end
		}
		return start, end, true
	}
	return time.Time{}, time.Time{}, false
}

func subtractIntervals(base []interval, remove interval) []interval {
	if !remove.end.After(remove.start) {
		return base
	}
	out := make([]interval, 0, len(base))
	for _, item := range base {
		if !item.end.After(remove.start) || !remove.end.After(item.start) {
			out = append(out, item)
			continue
		}

		if remove.start.After(item.start) {
			left := interval{start: item.start, end: minTime(remove.start, item.end)}
			if left.end.After(left.start) {
				out = append(out, left)
			}
		}
		if remove.end.Before(item.end) {
			right := interval{start: maxTime(remove.end, item.start), end: item.end}
			if right.end.After(right.start) {
				out = append(out, right)
			}
		}
	}
	return out
}

func overlapInterval(a, b interval) int {
	start := maxTime(a.start, b.start)
	end := minTime(a.end, b.end)
	if !end.After(start) {
		return 0
	}
	return int(end.Sub(start).Seconds())
}

func intervalsSeconds(items []interval) int {
	total := 0
	for _, item := range items {
		if item.end.After(item.start) {
			total += int(item.end.Sub(item.start).Seconds())
		}
	}
	return total
}

func parseClockMinutes(raw string) (int, error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(raw))
	if err != nil {
		return 0, err
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

func normalizeWeekdayTokens(tokens []string) ([]string, error) {
	weekdays, err := parseWeekdayTokens(tokens)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(weekdays))
	for _, day := range weekdays {
		out = append(out, weekdayToToken(day))
	}
	return out, nil
}

func parseWeekdayTokens(tokens []string) ([]time.Weekday, error) {
	seen := map[time.Weekday]struct{}{}
	out := make([]time.Weekday, 0, len(tokens))
	for _, token := range tokens {
		day, ok := parseWeekdayToken(token)
		if !ok {
			return nil, fmt.Errorf("invalid weekday token: %s", token)
		}
		if _, exists := seen[day]; exists {
			continue
		}
		out = append(out, day)
		seen[day] = struct{}{}
	}
	sort.Slice(out, func(i, j int) bool { return weekdayRank(out[i]) < weekdayRank(out[j]) })
	return out, nil
}

func parseWeekdayToken(token string) (time.Weekday, bool) {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "mon", "monday":
		return time.Monday, true
	case "tue", "tues", "tuesday":
		return time.Tuesday, true
	case "wed", "wednesday":
		return time.Wednesday, true
	case "thu", "thurs", "thursday":
		return time.Thursday, true
	case "fri", "friday":
		return time.Friday, true
	case "sat", "saturday":
		return time.Saturday, true
	case "sun", "sunday":
		return time.Sunday, true
	default:
		return 0, false
	}
}

func weekdayToToken(day time.Weekday) string {
	switch day {
	case time.Monday:
		return "mon"
	case time.Tuesday:
		return "tue"
	case time.Wednesday:
		return "wed"
	case time.Thursday:
		return "thu"
	case time.Friday:
		return "fri"
	case time.Saturday:
		return "sat"
	default:
		return "sun"
	}
}

func weekdayRank(day time.Weekday) int {
	switch day {
	case time.Monday:
		return 1
	case time.Tuesday:
		return 2
	case time.Wednesday:
		return 3
	case time.Thursday:
		return 4
	case time.Friday:
		return 5
	case time.Saturday:
		return 6
	default:
		return 7
	}
}

func resolveLocation(raw string) (*time.Location, error) {
	name := strings.TrimSpace(raw)
	if name == "" || strings.EqualFold(name, "local") {
		return time.Local, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, err
	}
	return loc, nil
}

func topicsEqual(domainA, subtopicA, domainB, subtopicB string) bool {
	return strings.EqualFold(strings.TrimSpace(domainA), strings.TrimSpace(domainB)) &&
		strings.EqualFold(strings.TrimSpace(subtopicA), strings.TrimSpace(subtopicB))
}

func startOfDay(t time.Time, location *time.Location) time.Time {
	tt := t.In(location)
	return time.Date(tt.Year(), tt.Month(), tt.Day(), 0, 0, 0, 0, location)
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
