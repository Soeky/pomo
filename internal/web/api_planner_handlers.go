package web

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/scheduler"
)

type plannerGenerateRequest struct {
	From    string `json:"from"`
	To      string `json:"to"`
	DryRun  bool   `json:"dry_run"`
	Replace bool   `json:"replace"`
}

type plannerTargetCreateRequest struct {
	Title       string  `json:"title"`
	Topic       string  `json:"topic"`
	Domain      string  `json:"domain"`
	Subtopic    string  `json:"subtopic"`
	Cadence     string  `json:"cadence"`
	Duration    string  `json:"duration"`
	Hours       float64 `json:"hours"`
	Occurrences int     `json:"occurrences"`
	Active      *bool   `json:"active"`
}

type plannerTargetActiveRequest struct {
	Active bool `json:"active"`
}

type plannerConstraintPatchRequest struct {
	ActiveWeekdays       []string `json:"active_weekdays"`
	DayStart             string   `json:"day_start"`
	DayEnd               string   `json:"day_end"`
	LunchStart           string   `json:"lunch_start"`
	LunchDurationMinutes int      `json:"lunch_duration_minutes"`
	DinnerStart          string   `json:"dinner_start"`
	DinnerDurationMinute int      `json:"dinner_duration_minutes"`
	MaxHoursPerDay       int      `json:"max_hours_per_day"`
	Timezone             string   `json:"timezone"`
}

func (s *Server) apiPlannerStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	from, to, err := plannerRangeFromRequest(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	var planned, done, blocked, canceled int
	if err := db.DB.QueryRow(`
		SELECT
			COALESCE(SUM(CASE WHEN status='planned' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status='done' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status='blocked' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status='canceled' THEN 1 ELSE 0 END), 0)
		FROM events
		WHERE layer='planned' AND start_time >= ? AND start_time < ?`, from, to).Scan(&planned, &done, &blocked, &canceled); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	total := planned + done + blocked + canceled
	completion := 0.0
	if total > 0 {
		completion = float64(done) * 100 / float64(total)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"from":       from.Format(time.RFC3339),
		"to":         to.Format(time.RFC3339),
		"planned":    planned,
		"done":       done,
		"blocked":    blocked,
		"canceled":   canceled,
		"total":      total,
		"completion": completion,
	})
}

func (s *Server) apiPlannerGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req plannerGenerateRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	from, err := parseAnyTime(req.From)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid from")
		return
	}
	to, err := parseAnyTime(req.To)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid to")
		return
	}

	result, err := scheduler.GenerateFromDB(context.Background(), scheduler.DBInput{
		From:                     from,
		To:                       to,
		Persist:                  !req.DryRun,
		ReplaceExistingScheduler: req.Replace,
	})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) apiPlannerTargets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		activeOnly := parseBoolish(r.URL.Query().Get("active_only"))
		rows, err := scheduler.ListWorkloadTargets(r.Context(), activeOnly)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"rows": rows})
	case http.MethodPost:
		var req plannerTargetCreateRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		targetSeconds, err := resolveTargetSecondsWeb(req.Duration, req.Hours)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.Occurrences > 0 && targetSeconds == 0 {
			writeJSONError(w, http.StatusBadRequest, "duration or hours is required when occurrences is set")
			return
		}
		if req.Occurrences == 0 && targetSeconds == 0 {
			writeJSONError(w, http.StatusBadRequest, "target duration is required")
			return
		}
		domain, subtopic := normalizeDomainSubtopic(req.Topic, req.Domain, req.Subtopic)
		active := true
		if req.Active != nil {
			active = *req.Active
		}
		id, err := scheduler.CreateWorkloadTarget(r.Context(), scheduler.WorkloadTarget{
			Title:             strings.TrimSpace(req.Title),
			Domain:            domain,
			Subtopic:          subtopic,
			Cadence:           strings.TrimSpace(defaultIfEmptyString(req.Cadence, "weekly")),
			TargetSeconds:     targetSeconds,
			TargetOccurrences: req.Occurrences,
			Active:            active,
		})
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": id})
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) apiPlannerTargetByID(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/planner/targets/"), "/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(path, "/")
	id, err := parsePositiveInt64(parts[0])
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid target id")
		return
	}

	if len(parts) == 1 && r.Method == http.MethodDelete {
		if err := scheduler.DeleteWorkloadTarget(r.Context(), id); err != nil {
			if err == sql.ErrNoRows {
				writeJSONError(w, http.StatusNotFound, "target not found")
				return
			}
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
		return
	}

	if len(parts) == 2 && parts[1] == "active" && r.Method == http.MethodPatch {
		var req plannerTargetActiveRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := scheduler.SetWorkloadTargetActive(r.Context(), id, req.Active); err != nil {
			if err == sql.ErrNoRows {
				writeJSONError(w, http.StatusNotFound, "target not found")
				return
			}
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"updated": true, "active": req.Active})
		return
	}

	writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (s *Server) apiPlannerConstraints(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := scheduler.LoadConstraintConfig(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, cfg)
	case http.MethodPatch:
		cfg, err := scheduler.LoadConstraintConfig(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		var req plannerConstraintPatchRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if len(req.ActiveWeekdays) > 0 {
			cfg.ActiveWeekdays = normalizeConstraintWeekdaysWeb(req.ActiveWeekdays)
		}
		if strings.TrimSpace(req.DayStart) != "" {
			cfg.DayStart = strings.TrimSpace(req.DayStart)
		}
		if strings.TrimSpace(req.DayEnd) != "" {
			cfg.DayEnd = strings.TrimSpace(req.DayEnd)
		}
		if strings.TrimSpace(req.LunchStart) != "" {
			cfg.LunchStart = strings.TrimSpace(req.LunchStart)
		}
		if req.LunchDurationMinutes > 0 {
			cfg.LunchDurationMinutes = req.LunchDurationMinutes
		}
		if strings.TrimSpace(req.DinnerStart) != "" {
			cfg.DinnerStart = strings.TrimSpace(req.DinnerStart)
		}
		if req.DinnerDurationMinute > 0 {
			cfg.DinnerDurationMinutes = req.DinnerDurationMinute
		}
		if req.MaxHoursPerDay > 0 {
			cfg.MaxHoursPerDay = req.MaxHoursPerDay
		}
		if strings.TrimSpace(req.Timezone) != "" {
			cfg.Timezone = strings.TrimSpace(req.Timezone)
		}
		if err := scheduler.SaveConstraintConfig(r.Context(), cfg); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"updated": true})
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func plannerRangeFromRequest(r *http.Request) (time.Time, time.Time, error) {
	fromRaw := strings.TrimSpace(r.URL.Query().Get("from"))
	toRaw := strings.TrimSpace(r.URL.Query().Get("to"))
	now := time.Now()
	if fromRaw == "" {
		fromRaw = now.AddDate(0, 0, -7).Format("2006-01-02T15:04")
	}
	if toRaw == "" {
		toRaw = now.AddDate(0, 0, 1).Format("2006-01-02T15:04")
	}
	from, err := parseAnyTime(fromRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid from")
	}
	to, err := parseAnyTime(toRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid to")
	}
	if !to.After(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid range")
	}
	return from, to, nil
}

func resolveTargetSecondsWeb(durationRaw string, hours float64) (int, error) {
	durationRaw = strings.TrimSpace(durationRaw)
	if durationRaw != "" {
		duration, err := time.ParseDuration(durationRaw)
		if err != nil || duration <= 0 {
			return 0, fmt.Errorf("invalid duration")
		}
		return int(duration.Seconds()), nil
	}
	if hours > 0 {
		return int((time.Duration(hours * float64(time.Hour))).Seconds()), nil
	}
	return 0, nil
}

func normalizeConstraintWeekdaysWeb(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, token := range in {
		normalized := strings.ToLower(strings.TrimSpace(token))
		switch normalized {
		case "monday":
			normalized = "mon"
		case "tuesday":
			normalized = "tue"
		case "wednesday":
			normalized = "wed"
		case "thursday":
			normalized = "thu"
		case "friday":
			normalized = "fri"
		case "saturday":
			normalized = "sat"
		case "sunday":
			normalized = "sun"
		}
		switch normalized {
		case "mon", "tue", "wed", "thu", "fri", "sat", "sun":
			if _, ok := seen[normalized]; !ok {
				out = append(out, normalized)
				seen[normalized] = struct{}{}
			}
		}
	}
	return out
}
