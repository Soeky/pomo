package web

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/events"
	"github.com/Soeky/pomo/internal/topics"
)

type eventCreateRequest struct {
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Topic       string `json:"topic"`
	Domain      string `json:"domain"`
	Subtopic    string `json:"subtopic"`
	Description string `json:"description"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	Layer       string `json:"layer"`
	Status      string `json:"status"`
	Source      string `json:"source"`
}

type eventPatchRequest struct {
	Kind        *string `json:"kind"`
	Title       *string `json:"title"`
	Topic       *string `json:"topic"`
	Domain      *string `json:"domain"`
	Subtopic    *string `json:"subtopic"`
	Description *string `json:"description"`
	StartTime   *string `json:"start_time"`
	EndTime     *string `json:"end_time"`
	Layer       *string `json:"layer"`
	Status      *string `json:"status"`
	Source      *string `json:"source"`
}

type recurrenceCreateRequest struct {
	Title       string `json:"title"`
	Topic       string `json:"topic"`
	Domain      string `json:"domain"`
	Subtopic    string `json:"subtopic"`
	Kind        string `json:"kind"`
	Duration    string `json:"duration"`
	RRule       string `json:"rrule"`
	Freq        string `json:"freq"`
	Interval    int    `json:"interval"`
	ByDay       string `json:"byday"`
	ByMonthDay  int    `json:"bymonthday"`
	Timezone    string `json:"timezone"`
	StartTime   string `json:"start_time"`
	Until       string `json:"until"`
	Active      *bool  `json:"active"`
	Description string `json:"description"`
}

type recurrencePatchRequest struct {
	Title      *string `json:"title"`
	Topic      *string `json:"topic"`
	Domain     *string `json:"domain"`
	Subtopic   *string `json:"subtopic"`
	Kind       *string `json:"kind"`
	Duration   *string `json:"duration"`
	RRule      *string `json:"rrule"`
	Freq       *string `json:"freq"`
	Interval   *int    `json:"interval"`
	ByDay      *string `json:"byday"`
	ByMonthDay *int    `json:"bymonthday"`
	Timezone   *string `json:"timezone"`
	StartTime  *string `json:"start_time"`
	Until      *string `json:"until"`
	ClearUntil *bool   `json:"clear_until"`
	Active     *bool   `json:"active"`
}

type recurrenceExpandRequest struct {
	From   string `json:"from"`
	To     string `json:"to"`
	RuleID int64  `json:"rule_id"`
}

type dependencyAddRequest struct {
	DependsOnEventID int64 `json:"depends_on_event_id"`
	Required         *bool `json:"required"`
}

type dependencyOverrideRequest struct {
	Admin  bool   `json:"admin"`
	Clear  bool   `json:"clear"`
	Reason string `json:"reason"`
}

func (s *Server) apiEvents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		from := parseTimeOrDefault(r.URL.Query().Get("from"), time.Now().AddDate(0, 0, -1))
		to := parseTimeOrDefault(r.URL.Query().Get("to"), time.Now().AddDate(0, 0, 7))
		rows, err := events.ListInRange(r.Context(), from, to)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		layer := strings.TrimSpace(r.URL.Query().Get("layer"))
		status := strings.TrimSpace(r.URL.Query().Get("status"))
		source := strings.TrimSpace(r.URL.Query().Get("source"))
		kind := strings.TrimSpace(r.URL.Query().Get("kind"))
		filtered := make([]events.Event, 0, len(rows))
		for _, row := range rows {
			if layer != "" && !strings.EqualFold(row.Layer, layer) {
				continue
			}
			if status != "" && !strings.EqualFold(row.Status, status) {
				continue
			}
			if source != "" && !strings.EqualFold(row.Source, source) {
				continue
			}
			if kind != "" && !strings.EqualFold(row.Kind, kind) {
				continue
			}
			filtered = append(filtered, row)
		}
		writeJSON(w, http.StatusOK, map[string]any{"rows": filtered})
	case http.MethodPost:
		var req eventCreateRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		start, err := parseAnyTime(req.StartTime)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid start_time")
			return
		}
		end, err := parseAnyTime(req.EndTime)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid end_time")
			return
		}
		domain, subtopic := normalizeDomainSubtopic(req.Topic, req.Domain, req.Subtopic)
		id, err := events.Create(r.Context(), events.Event{
			Kind:        strings.TrimSpace(req.Kind),
			Title:       strings.TrimSpace(req.Title),
			Domain:      domain,
			Subtopic:    subtopic,
			Description: strings.TrimSpace(req.Description),
			StartTime:   start,
			EndTime:     end,
			Layer:       strings.TrimSpace(req.Layer),
			Status:      strings.TrimSpace(req.Status),
			Source:      strings.TrimSpace(req.Source),
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

func (s *Server) apiEventByPath(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/events/"), "/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(path, "/")

	if parts[0] == "recurrence" {
		s.apiRecurrenceByPath(w, r, parts[1:])
		return
	}

	eventID, err := parsePositiveInt64(parts[0])
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid event id")
		return
	}

	if len(parts) == 1 {
		s.apiEventByID(w, r, eventID)
		return
	}

	switch parts[1] {
	case "dependencies":
		if len(parts) == 2 {
			s.apiEventDependencies(w, r, eventID)
			return
		}
		if len(parts) == 3 {
			dependsOnID, err := parsePositiveInt64(parts[2])
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "invalid depends_on_event_id")
				return
			}
			s.apiEventDependencyDelete(w, r, eventID, dependsOnID)
			return
		}
	case "override":
		if len(parts) == 2 {
			s.apiEventOverride(w, r, eventID)
			return
		}
	}

	http.NotFound(w, r)
}

func (s *Server) apiEventByID(w http.ResponseWriter, r *http.Request, eventID int64) {
	switch r.Method {
	case http.MethodPatch:
		var req eventPatchRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		current, err := events.GetByID(r.Context(), eventID)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, err.Error())
			return
		}
		if req.Kind != nil {
			current.Kind = strings.TrimSpace(*req.Kind)
		}
		if req.Title != nil {
			current.Title = strings.TrimSpace(*req.Title)
		}
		if req.Description != nil {
			current.Description = strings.TrimSpace(*req.Description)
		}
		if req.Layer != nil {
			current.Layer = strings.TrimSpace(*req.Layer)
		}
		if req.Status != nil {
			current.Status = strings.TrimSpace(*req.Status)
		}
		if req.Source != nil {
			current.Source = strings.TrimSpace(*req.Source)
		}
		if req.StartTime != nil {
			start, err := parseAnyTime(*req.StartTime)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "invalid start_time")
				return
			}
			current.StartTime = start
		}
		if req.EndTime != nil {
			end, err := parseAnyTime(*req.EndTime)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "invalid end_time")
				return
			}
			current.EndTime = end
		}
		if req.Topic != nil || req.Domain != nil || req.Subtopic != nil {
			topicVal := ""
			if req.Topic != nil {
				topicVal = *req.Topic
			}
			domainVal := current.Domain
			if req.Domain != nil {
				domainVal = *req.Domain
			}
			subtopicVal := current.Subtopic
			if req.Subtopic != nil {
				subtopicVal = *req.Subtopic
			}
			domain, subtopic := normalizeDomainSubtopic(topicVal, domainVal, subtopicVal)
			current.Domain = domain
			current.Subtopic = subtopic
		}
		if err := events.Update(r.Context(), eventID, current); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"updated": true})
	case http.MethodDelete:
		if err := events.Delete(r.Context(), eventID); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) apiEventDependencies(w http.ResponseWriter, r *http.Request, eventID int64) {
	switch r.Method {
	case http.MethodGet:
		rows, err := events.ListDependencies(r.Context(), eventID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"rows": rows})
	case http.MethodPost:
		var req dependencyAddRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		required := true
		if req.Required != nil {
			required = *req.Required
		}
		if err := events.AddDependency(r.Context(), eventID, req.DependsOnEventID, required); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"created": true})
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) apiEventDependencyDelete(w http.ResponseWriter, r *http.Request, eventID, dependsOnID int64) {
	if r.Method != http.MethodDelete {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := events.DeleteDependency(r.Context(), eventID, dependsOnID); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (s *Server) apiEventOverride(w http.ResponseWriter, r *http.Request, eventID int64) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req dependencyOverrideRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	enabled := !req.Clear
	if err := events.SetDependencyOverride(r.Context(), eventID, enabled, req.Admin, req.Reason, "web"); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": true, "enabled": enabled})
}

func (s *Server) apiRecurrenceByPath(w http.ResponseWriter, r *http.Request, tail []string) {
	if len(tail) == 0 {
		switch r.Method {
		case http.MethodGet:
			activeOnly := parseBoolish(r.URL.Query().Get("active_only"))
			rows, err := events.ListRecurrenceRules(r.Context(), activeOnly)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"rows": rows})
		case http.MethodPost:
			var req recurrenceCreateRequest
			if err := decodeJSONBody(r, &req); err != nil {
				writeJSONError(w, http.StatusBadRequest, err.Error())
				return
			}
			id, err := s.createRecurrenceRuleFromRequest(r.Context(), req)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"id": id})
		default:
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	if len(tail) == 1 && tail[0] == "expand" {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req recurrenceExpandRequest
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
		result, err := events.GenerateRecurringEventsInWindow(r.Context(), from, to, req.RuleID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}

	ruleID, err := parsePositiveInt64(tail[0])
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid recurrence rule id")
		return
	}

	switch r.Method {
	case http.MethodPatch:
		var req recurrencePatchRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.updateRecurrenceRuleFromRequest(r.Context(), ruleID, req); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"updated": true})
	case http.MethodDelete:
		if err := events.DeleteRecurrenceRule(r.Context(), ruleID); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) createRecurrenceRuleFromRequest(ctx context.Context, req recurrenceCreateRequest) (int64, error) {
	start, err := parseAnyTime(req.StartTime)
	if err != nil {
		return 0, fmt.Errorf("invalid start_time")
	}
	durationSec, err := parseDurationSeconds(req.Duration)
	if err != nil {
		return 0, err
	}
	domain, subtopic := normalizeDomainSubtopic(req.Topic, req.Domain, req.Subtopic)

	rrule := strings.TrimSpace(req.RRule)
	if rrule == "" {
		byDays, err := parseRecurrenceWeekdays(req.ByDay)
		if err != nil {
			return 0, err
		}
		rrule, err = events.BuildRRule(events.RecurrenceSpec{
			Freq:       strings.TrimSpace(req.Freq),
			Interval:   req.Interval,
			ByDays:     byDays,
			ByMonthDay: req.ByMonthDay,
		})
		if err != nil {
			return 0, err
		}
	}

	var untilPtr *time.Time
	if strings.TrimSpace(req.Until) != "" {
		until, err := parseAnyTime(req.Until)
		if err != nil {
			return 0, fmt.Errorf("invalid until")
		}
		untilPtr = &until
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	return events.CreateRecurrenceRule(ctx, events.RecurrenceRule{
		Title:       strings.TrimSpace(req.Title),
		Domain:      domain,
		Subtopic:    subtopic,
		Kind:        strings.TrimSpace(req.Kind),
		DurationSec: durationSec,
		RRule:       rrule,
		Timezone:    strings.TrimSpace(defaultIfEmptyString(req.Timezone, "Local")),
		StartDate:   start,
		EndDate:     untilPtr,
		Active:      active,
	})
}

func (s *Server) updateRecurrenceRuleFromRequest(ctx context.Context, ruleID int64, req recurrencePatchRequest) error {
	rule, err := events.GetRecurrenceRuleByID(ctx, ruleID)
	if err != nil {
		return err
	}
	if req.Title != nil {
		rule.Title = strings.TrimSpace(*req.Title)
	}
	if req.Kind != nil {
		rule.Kind = strings.TrimSpace(*req.Kind)
	}
	if req.StartTime != nil {
		start, err := parseAnyTime(*req.StartTime)
		if err != nil {
			return fmt.Errorf("invalid start_time")
		}
		rule.StartDate = start
	}
	if req.Duration != nil {
		durationSec, err := parseDurationSeconds(*req.Duration)
		if err != nil {
			return err
		}
		rule.DurationSec = durationSec
	}
	if req.Timezone != nil {
		rule.Timezone = strings.TrimSpace(*req.Timezone)
	}
	if req.Active != nil {
		rule.Active = *req.Active
	}
	if req.ClearUntil != nil && *req.ClearUntil {
		rule.EndDate = nil
	}
	if req.Until != nil {
		untilRaw := strings.TrimSpace(*req.Until)
		if untilRaw == "" {
			rule.EndDate = nil
		} else {
			until, err := parseAnyTime(untilRaw)
			if err != nil {
				return fmt.Errorf("invalid until")
			}
			rule.EndDate = &until
		}
	}
	if req.Topic != nil || req.Domain != nil || req.Subtopic != nil {
		topicVal := ""
		if req.Topic != nil {
			topicVal = *req.Topic
		}
		domainVal := rule.Domain
		if req.Domain != nil {
			domainVal = *req.Domain
		}
		subtopicVal := rule.Subtopic
		if req.Subtopic != nil {
			subtopicVal = *req.Subtopic
		}
		rule.Domain, rule.Subtopic = normalizeDomainSubtopic(topicVal, domainVal, subtopicVal)
	}

	if req.RRule != nil {
		rule.RRule = strings.TrimSpace(*req.RRule)
	} else if req.Freq != nil || req.Interval != nil || req.ByDay != nil || req.ByMonthDay != nil {
		spec, err := events.ParseRRule(rule.RRule)
		if err != nil {
			return err
		}
		if req.Freq != nil {
			spec.Freq = strings.TrimSpace(*req.Freq)
		}
		if req.Interval != nil {
			spec.Interval = *req.Interval
		}
		if req.ByDay != nil {
			byDays, err := parseRecurrenceWeekdays(*req.ByDay)
			if err != nil {
				return err
			}
			spec.ByDays = byDays
		}
		if req.ByMonthDay != nil {
			spec.ByMonthDay = *req.ByMonthDay
		}
		rrule, err := events.BuildRRule(spec)
		if err != nil {
			return err
		}
		rule.RRule = rrule
	}

	return events.UpdateRecurrenceRule(ctx, ruleID, rule)
}

func parsePositiveInt64(raw string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return value, nil
}

func normalizeDomainSubtopic(topic, domain, subtopic string) (string, string) {
	topic = strings.TrimSpace(topic)
	if topic != "" {
		if parsed, err := topics.Parse(topic); err == nil {
			return parsed.Domain, parsed.Subtopic
		}
	}
	if parsed, err := topics.ParseParts(strings.TrimSpace(domain), strings.TrimSpace(subtopic)); err == nil {
		return parsed.Domain, parsed.Subtopic
	}
	return topics.DefaultDomain, topics.DefaultSubtopic
}

func parseDurationSeconds(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("duration is required")
	}
	duration, err := time.ParseDuration(raw)
	if err == nil && duration > 0 {
		return int(duration.Seconds()), nil
	}
	if minutes, convErr := strconv.Atoi(raw); convErr == nil && minutes > 0 {
		return minutes * 60, nil
	}
	return 0, fmt.Errorf("invalid duration")
}
