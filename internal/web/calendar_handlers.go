package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/events"
	"github.com/Soeky/pomo/internal/store"
)

type calendarEvent struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Start          string `json:"start"`
	End            string `json:"end,omitempty"`
	Color          string `json:"color,omitempty"`
	Status         string `json:"status,omitempty"`
	BlockingReason string `json:"blocking_reason,omitempty"`
	Editable       bool   `json:"editable"`
}

func (s *Server) calendarPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.tmpl.ExecuteTemplate(w, "calendar.html", map[string]any{
		"ActivePage":            "calendar",
		"PageTitle":             "Calendar",
		"IncludeCalendarAssets": true,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) calendarEvents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		from := parseTimeOrDefault(r.URL.Query().Get("start"), time.Now().AddDate(0, -1, 0))
		to := parseTimeOrDefault(r.URL.Query().Get("end"), time.Now().AddDate(0, 1, 0))
		if s.sqlDB == nil {
			http.Error(w, "database is not initialized", http.StatusInternalServerError)
			return
		}
		if _, err := events.GenerateRecurringEventsInWindow(r.Context(), from, to, 0); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		canonicalEvents, err := events.ListInRange(r.Context(), from, to)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		eventsOut := make([]calendarEvent, 0, len(canonicalEvents))
		for _, e := range canonicalEvents {
			color := "#1a365d"
			switch e.Kind {
			case "focus":
				color = "#2f855a"
			case "break":
				color = "#718096"
			}
			switch e.Source {
			case "recurring":
				color = "#975a16"
			case "scheduler":
				color = "#276749"
			case "manual":
				color = "#2c5282"
			}
			title := e.Title
			if strings.EqualFold(e.Status, "blocked") && strings.TrimSpace(e.BlockedReason) != "" {
				title = fmt.Sprintf("%s (blocked: %s)", e.Title, e.BlockedReason)
			}
			eventsOut = append(eventsOut, calendarEvent{
				ID:             fmt.Sprintf("e-%d", e.ID),
				Title:          title,
				Start:          e.StartTime.Format(time.RFC3339),
				End:            e.EndTime.Format(time.RFC3339),
				Color:          color,
				Status:         e.Status,
				BlockingReason: e.BlockedReason,
				Editable:       true,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(eventsOut)
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		start, err := parseAnyTime(r.FormValue("start_time"))
		if err != nil {
			http.Error(w, "invalid start_time", http.StatusBadRequest)
			return
		}
		end, err := parseAnyTime(r.FormValue("end_time"))
		if err != nil {
			http.Error(w, "invalid end_time", http.StatusBadRequest)
			return
		}
		parsedTopic, err := parseTopicForm(r, "topic", "title")
		if err != nil {
			http.Error(w, "invalid topic format", http.StatusBadRequest)
			return
		}
		title := strings.TrimSpace(r.FormValue("title"))
		if !parsedTopic.Provided {
			http.Error(w, "title, topic, or domain is required", http.StatusBadRequest)
			return
		}
		title = normalizePlannedTitle(title, parsedTopic)
		_, err = s.store.CreatePlannedEvent(r.Context(), store.PlannedEvent{
			Title:       title,
			Description: r.FormValue("description"),
			StartTime:   start,
			EndTime:     end,
			Status:      "planned",
			Source:      "manual",
		}, "web")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) calendarEventByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/calendar/events/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	kind, numericID, err := parsePrefixedID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	eventID, err := s.resolveCalendarEventID(r.Context(), kind, numericID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "event not found (legacy s-/p- ids are deprecated; use e-<id>)", http.StatusGone)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if kind != "e" {
		w.Header().Set("Warning", `299 - "legacy calendar IDs (s-/p-) are deprecated; use e-<id>"`)
	}
	if s.sqlDB == nil {
		http.Error(w, "database is not initialized", http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodPatch:
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		start, err := parseAnyTime(r.FormValue("start_time"))
		if err != nil {
			http.Error(w, "invalid start_time", http.StatusBadRequest)
			return
		}
		end, err := parseAnyTime(r.FormValue("end_time"))
		if err != nil {
			http.Error(w, "invalid end_time", http.StatusBadRequest)
			return
		}
		existing, err := events.GetByID(r.Context(), eventID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		existing.StartTime = start
		existing.EndTime = end
		title := strings.TrimSpace(r.FormValue("title"))
		if title != "" {
			existing.Title = title
		}
		if parsedTopic, err := parseTopicForm(r, "topic", "domain"); err != nil {
			http.Error(w, "invalid topic format", http.StatusBadRequest)
			return
		} else if parsedTopic.Provided && !strings.EqualFold(existing.Kind, "break") {
			existing.Domain = parsedTopic.Path.Domain
			existing.Subtopic = parsedTopic.Path.Subtopic
			if strings.TrimSpace(title) == "" {
				existing.Title = parsedTopic.Path.Canonical()
			}
		}
		if err := events.Update(r.Context(), eventID, existing); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if err := events.Delete(r.Context(), eventID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) resolveCalendarEventID(ctx context.Context, kind string, numericID int) (int64, error) {
	if kind == "e" {
		return int64(numericID), nil
	}
	if s.sqlDB == nil {
		return 0, fmt.Errorf("database is not initialized")
	}
	var legacySource string
	switch kind {
	case "s":
		legacySource = "sessions"
	case "p":
		legacySource = "planned_events"
	default:
		return 0, fmt.Errorf("unsupported id prefix")
	}
	var eventID int64
	err := s.sqlDB.QueryRowContext(ctx, `
		SELECT id
		FROM events
		WHERE legacy_source = ?
		  AND legacy_id = ?
		ORDER BY id ASC
		LIMIT 1`, legacySource, numericID).Scan(&eventID)
	if err != nil {
		return 0, err
	}
	return eventID, nil
}

func (s *Server) recurrenceRules(w http.ResponseWriter, r *http.Request) {
	if s.sqlDB == nil {
		http.Error(w, "database is not initialized", http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		rules, err := events.ListRecurrenceRules(r.Context(), false)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rules)
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		title := strings.TrimSpace(r.FormValue("title"))
		if title == "" {
			http.Error(w, "title required", http.StatusBadRequest)
			return
		}
		start, err := parseAnyTime(r.FormValue("start_time"))
		if err != nil {
			http.Error(w, "invalid start_time", http.StatusBadRequest)
			return
		}
		durationMin, err := strconv.Atoi(strings.TrimSpace(r.FormValue("duration_minutes")))
		if err != nil || durationMin <= 0 {
			http.Error(w, "invalid duration_minutes", http.StatusBadRequest)
			return
		}
		interval := 1
		if raw := strings.TrimSpace(r.FormValue("interval")); raw != "" {
			interval, err = strconv.Atoi(raw)
			if err != nil || interval <= 0 {
				http.Error(w, "invalid interval", http.StatusBadRequest)
				return
			}
		}
		byMonthDay := 0
		if raw := strings.TrimSpace(r.FormValue("bymonthday")); raw != "" {
			byMonthDay, err = strconv.Atoi(raw)
			if err != nil || byMonthDay <= 0 {
				http.Error(w, "invalid bymonthday", http.StatusBadRequest)
				return
			}
		}
		byDays, err := parseRecurrenceWeekdays(r.FormValue("byday"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		rrule, err := events.BuildRRule(events.RecurrenceSpec{
			Freq:       strings.TrimSpace(r.FormValue("freq")),
			Interval:   interval,
			ByDays:     byDays,
			ByMonthDay: byMonthDay,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var untilPtr *time.Time
		if raw := strings.TrimSpace(r.FormValue("until")); raw != "" {
			until, err := parseAnyTime(raw)
			if err != nil {
				http.Error(w, "invalid until", http.StatusBadRequest)
				return
			}
			untilPtr = &until
		}
		parsedTopic, err := parseTopicForm(r, "topic")
		if err != nil {
			http.Error(w, "invalid topic format", http.StatusBadRequest)
			return
		}
		if !parsedTopic.Provided {
			http.Error(w, "domain or topic is required", http.StatusBadRequest)
			return
		}
		active := true
		if strings.TrimSpace(r.FormValue("active")) != "" {
			active = parseBoolish(r.FormValue("active"))
		}
		id, err := events.CreateRecurrenceRule(r.Context(), events.RecurrenceRule{
			Title:       title,
			Domain:      parsedTopic.Path.Domain,
			Subtopic:    parsedTopic.Path.Subtopic,
			Kind:        strings.TrimSpace(r.FormValue("kind")),
			DurationSec: durationMin * 60,
			RRule:       rrule,
			Timezone:    strings.TrimSpace(defaultIfEmptyString(r.FormValue("timezone"), "Local")),
			StartDate:   start,
			EndDate:     untilPtr,
			Active:      active,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": id})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) recurrenceRuleByID(w http.ResponseWriter, r *http.Request) {
	if s.sqlDB == nil {
		http.Error(w, "database is not initialized", http.StatusInternalServerError)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/calendar/recurrence/")
	id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPatch:
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		rule, err := events.GetRecurrenceRuleByID(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if raw := strings.TrimSpace(r.FormValue("title")); raw != "" {
			rule.Title = raw
		}
		if raw := strings.TrimSpace(r.FormValue("start_time")); raw != "" {
			start, err := parseAnyTime(raw)
			if err != nil {
				http.Error(w, "invalid start_time", http.StatusBadRequest)
				return
			}
			rule.StartDate = start
		}
		if raw := strings.TrimSpace(r.FormValue("duration_minutes")); raw != "" {
			durMin, err := strconv.Atoi(raw)
			if err != nil || durMin <= 0 {
				http.Error(w, "invalid duration_minutes", http.StatusBadRequest)
				return
			}
			rule.DurationSec = durMin * 60
		}
		if raw := strings.TrimSpace(r.FormValue("kind")); raw != "" {
			rule.Kind = raw
		}
		parsedTopic, err := parseTopicForm(r, "topic")
		if err != nil {
			http.Error(w, "invalid topic format", http.StatusBadRequest)
			return
		}
		if parsedTopic.Provided {
			rule.Domain = parsedTopic.Path.Domain
			rule.Subtopic = parsedTopic.Path.Subtopic
		}
		if raw := strings.TrimSpace(r.FormValue("timezone")); raw != "" {
			rule.Timezone = raw
		}
		if raw := strings.TrimSpace(r.FormValue("active")); raw != "" {
			rule.Active = parseBoolish(raw)
		}
		if raw := strings.TrimSpace(r.FormValue("until")); raw != "" {
			until, err := parseAnyTime(raw)
			if err != nil {
				http.Error(w, "invalid until", http.StatusBadRequest)
				return
			}
			rule.EndDate = &until
		}
		if raw := strings.TrimSpace(r.FormValue("clear_until")); parseBoolish(raw) {
			rule.EndDate = nil
		}
		interval := 0
		if raw := strings.TrimSpace(r.FormValue("interval")); raw != "" {
			interval, err = strconv.Atoi(raw)
			if err != nil || interval <= 0 {
				http.Error(w, "invalid interval", http.StatusBadRequest)
				return
			}
		}
		byMonthDay := 0
		if raw := strings.TrimSpace(r.FormValue("bymonthday")); raw != "" {
			byMonthDay, err = strconv.Atoi(raw)
			if err != nil || byMonthDay <= 0 {
				http.Error(w, "invalid bymonthday", http.StatusBadRequest)
				return
			}
		}
		if strings.TrimSpace(r.FormValue("freq")) != "" || interval > 0 || strings.TrimSpace(r.FormValue("byday")) != "" || byMonthDay > 0 {
			spec, err := events.ParseRRule(rule.RRule)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if raw := strings.TrimSpace(r.FormValue("freq")); raw != "" {
				spec.Freq = raw
			}
			if interval > 0 {
				spec.Interval = interval
			}
			if raw := strings.TrimSpace(r.FormValue("byday")); raw != "" {
				days, err := parseRecurrenceWeekdays(raw)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				spec.ByDays = days
			}
			if byMonthDay > 0 {
				spec.ByMonthDay = byMonthDay
			}
			rrule, err := events.BuildRRule(spec)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			rule.RRule = rrule
		}
		if err := events.UpdateRecurrenceRule(r.Context(), id, rule); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if err := events.DeleteRecurrenceRule(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func parseRecurrenceWeekdays(raw string) ([]time.Weekday, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]time.Weekday, 0, len(parts))
	seen := map[time.Weekday]struct{}{}
	for _, part := range parts {
		token := strings.ToUpper(strings.TrimSpace(part))
		var day time.Weekday
		switch token {
		case "MO", "MON", "MONDAY":
			day = time.Monday
		case "TU", "TUE", "TUESDAY":
			day = time.Tuesday
		case "WE", "WED", "WEDNESDAY":
			day = time.Wednesday
		case "TH", "THU", "THURSDAY":
			day = time.Thursday
		case "FR", "FRI", "FRIDAY":
			day = time.Friday
		case "SA", "SAT", "SATURDAY":
			day = time.Saturday
		case "SU", "SUN", "SUNDAY":
			day = time.Sunday
		default:
			return nil, fmt.Errorf("invalid weekday token: %s", part)
		}
		if _, ok := seen[day]; !ok {
			out = append(out, day)
			seen[day] = struct{}{}
		}
	}
	return out, nil
}

func parseBoolish(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func defaultIfEmptyString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}
