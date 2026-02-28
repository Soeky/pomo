package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/topics"
)

func TestParityPagesGet(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	paths := []string{
		"/events",
		"/dependencies",
		"/planner",
		"/reports",
		"/config",
		"/delete",
		"/workflow",
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status for %s: got=%d want=%d", path, rec.Code, http.StatusOK)
		}
	}
}

func TestParityPagesMethodNotAllowed(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	paths := []string{
		"/events",
		"/dependencies",
		"/planner",
		"/reports",
		"/config",
		"/delete",
		"/workflow",
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("unexpected status for %s: got=%d want=%d", path, rec.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestSessionRuntimeAPIParity(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	startRes := doJSONRequest(t, mux, http.MethodPost, "/api/sessions/start", map[string]any{
		"duration": "25m",
		"domain":   "Runtime",
		"subtopic": "Focus",
	}, http.StatusCreated)
	if startRes["ID"] == nil {
		t.Fatalf("expected start response to include ID: %+v", startRes)
	}

	statusRes := doJSONRequest(t, mux, http.MethodGet, "/api/sessions/status", nil, http.StatusOK)
	if active, _ := statusRes["Active"].(bool); !active {
		t.Fatalf("expected active session status, got: %+v", statusRes)
	}

	stopRes := doJSONRequest(t, mux, http.MethodPost, "/api/sessions/stop", map[string]any{}, http.StatusOK)
	if stopped, _ := stopRes["Stopped"].(bool); !stopped {
		t.Fatalf("expected stopped=true, got: %+v", stopRes)
	}

	breakRes := doJSONRequest(t, mux, http.MethodPost, "/api/sessions/break", map[string]any{
		"duration": "5m",
	}, http.StatusCreated)
	if breakRes["ID"] == nil {
		t.Fatalf("expected break response to include ID: %+v", breakRes)
	}

	correctRes := doJSONRequest(t, mux, http.MethodPost, "/api/sessions/correct", map[string]any{
		"session_type":  "start",
		"back_duration": "10m",
		"domain":        "Runtime",
		"subtopic":      "Corrected",
	}, http.StatusOK)
	if correctRes["SessionType"] == nil {
		t.Fatalf("expected corrected session payload, got: %+v", correctRes)
	}
}

func TestSessionRuntimeAPIValidationBranches(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	assertErrorContains(t, doJSONRequest(t, mux, http.MethodGet, "/api/sessions/start", nil, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/sessions/start", map[string]any{"duration": "invalid"}, http.StatusBadRequest), "invalid duration")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/sessions/break", map[string]any{"duration": "invalid"}, http.StatusBadRequest), "invalid duration")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodGet, "/api/sessions/stop", nil, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/sessions/status", nil, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/sessions/correct", map[string]any{
		"session_type":  "start",
		"back_duration": "invalid",
	}, http.StatusBadRequest), "invalid back_duration")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/sessions/correct", map[string]any{
		"session_type":  "invalid",
		"back_duration": "10m",
	}, http.StatusBadRequest), "invalid session type")
}

func TestEventDependencyRecurrenceAPIParity(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	eventARes := doJSONRequest(t, mux, http.MethodPost, "/api/events", map[string]any{
		"kind":       "task",
		"title":      "Parity Event A",
		"domain":     "Parity",
		"subtopic":   "A",
		"start_time": "2026-03-01T10:00",
		"end_time":   "2026-03-01T11:00",
		"layer":      "planned",
		"status":     "planned",
		"source":     "manual",
	}, http.StatusCreated)
	eventAID := int64FromAny(t, eventARes["id"])

	eventBRes := doJSONRequest(t, mux, http.MethodPost, "/api/events", map[string]any{
		"kind":       "task",
		"title":      "Parity Event B",
		"domain":     "Parity",
		"subtopic":   "B",
		"start_time": "2026-03-01T11:30",
		"end_time":   "2026-03-01T12:30",
		"layer":      "planned",
		"status":     "planned",
		"source":     "manual",
	}, http.StatusCreated)
	eventBID := int64FromAny(t, eventBRes["id"])

	listRes := doJSONRequest(t, mux, http.MethodGet, "/api/events?from=2026-03-01T00:00&to=2026-03-02T00:00", nil, http.StatusOK)
	if rows, ok := listRes["rows"].([]any); !ok || len(rows) < 2 {
		t.Fatalf("expected event list to include created rows, got: %+v", listRes)
	}

	doJSONRequest(t, mux, http.MethodPatch, "/api/events/"+strconv.FormatInt(eventAID, 10), map[string]any{
		"title": "Parity Event A Updated",
	}, http.StatusOK)

	doJSONRequest(t, mux, http.MethodPost, "/api/events/"+strconv.FormatInt(eventBID, 10)+"/dependencies", map[string]any{
		"depends_on_event_id": eventAID,
		"required":            true,
	}, http.StatusCreated)

	depsRes := doJSONRequest(t, mux, http.MethodGet, "/api/events/"+strconv.FormatInt(eventBID, 10)+"/dependencies", nil, http.StatusOK)
	if rows, ok := depsRes["rows"].([]any); !ok || len(rows) == 0 {
		t.Fatalf("expected dependencies list, got: %+v", depsRes)
	}

	doJSONRequest(t, mux, http.MethodPost, "/api/events/"+strconv.FormatInt(eventBID, 10)+"/override", map[string]any{
		"admin":  true,
		"reason": "test override",
	}, http.StatusOK)

	doJSONRequest(t, mux, http.MethodDelete, "/api/events/"+strconv.FormatInt(eventBID, 10)+"/dependencies/"+strconv.FormatInt(eventAID, 10), nil, http.StatusOK)

	ruleRes := doJSONRequest(t, mux, http.MethodPost, "/api/events/recurrence", map[string]any{
		"title":      "Parity Recurrence",
		"domain":     "Parity",
		"subtopic":   "Recurring",
		"kind":       "task",
		"start_time": "2026-03-02T09:00",
		"duration":   "1h",
		"freq":       "weekly",
		"interval":   1,
		"byday":      "MO,WE",
		"timezone":   "Local",
		"active":     true,
	}, http.StatusCreated)
	ruleID := int64FromAny(t, ruleRes["id"])

	ruleListRes := doJSONRequest(t, mux, http.MethodGet, "/api/events/recurrence", nil, http.StatusOK)
	if rows, ok := ruleListRes["rows"].([]any); !ok || len(rows) == 0 {
		t.Fatalf("expected recurrence rows, got: %+v", ruleListRes)
	}

	doJSONRequest(t, mux, http.MethodPatch, "/api/events/recurrence/"+strconv.FormatInt(ruleID, 10), map[string]any{
		"title":    "Parity Recurrence Updated",
		"interval": 2,
	}, http.StatusOK)

	doJSONRequest(t, mux, http.MethodPost, "/api/events/recurrence/expand", map[string]any{
		"from":    "2026-03-01T00:00",
		"to":      "2026-03-31T23:59",
		"rule_id": ruleID,
	}, http.StatusOK)

	doJSONRequest(t, mux, http.MethodDelete, "/api/events/recurrence/"+strconv.FormatInt(ruleID, 10), nil, http.StatusOK)
	doJSONRequest(t, mux, http.MethodDelete, "/api/events/"+strconv.FormatInt(eventAID, 10), nil, http.StatusOK)
	doJSONRequest(t, mux, http.MethodDelete, "/api/events/"+strconv.FormatInt(eventBID, 10), nil, http.StatusOK)
}

func TestEventAPIValidationBranches(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/events", map[string]any{
		"title":      "Missing Time",
		"domain":     "Parity",
		"start_time": "bad",
		"end_time":   "2026-03-01T11:00",
	}, http.StatusBadRequest), "invalid start_time")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/events", map[string]any{
		"title":      "Missing Time",
		"domain":     "Parity",
		"start_time": "2026-03-01T10:00",
		"end_time":   "bad",
	}, http.StatusBadRequest), "invalid end_time")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/events", map[string]any{
		"title":      "Unknown Field",
		"domain":     "Parity",
		"start_time": "2026-03-01T10:00",
		"end_time":   "2026-03-01T11:00",
		"unknown":    "x",
	}, http.StatusBadRequest), "unknown field")

	eventOne := doJSONRequest(t, mux, http.MethodPost, "/api/events", map[string]any{
		"kind":       "task",
		"title":      "Branch Event One",
		"domain":     "Parity",
		"subtopic":   "One",
		"start_time": "2026-03-01T10:00",
		"end_time":   "2026-03-01T11:00",
		"layer":      "planned",
		"status":     "planned",
		"source":     "manual",
	}, http.StatusCreated)
	eventOneID := int64FromAny(t, eventOne["id"])

	eventTwo := doJSONRequest(t, mux, http.MethodPost, "/api/events", map[string]any{
		"kind":       "task",
		"title":      "Branch Event Two",
		"domain":     "Parity",
		"subtopic":   "Two",
		"start_time": "2026-03-01T12:00",
		"end_time":   "2026-03-01T13:00",
		"layer":      "planned",
		"status":     "planned",
		"source":     "manual",
	}, http.StatusCreated)
	eventTwoID := int64FromAny(t, eventTwo["id"])

	assertErrorContains(t, doJSONRequest(t, mux, http.MethodGet, "/api/events/nope", nil, http.StatusBadRequest), "invalid event id")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPut, "/api/events/"+strconv.FormatInt(eventOneID, 10), map[string]any{}, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPatch, "/api/events/"+strconv.FormatInt(eventOneID, 10), map[string]any{"start_time": "bad"}, http.StatusBadRequest), "invalid start_time")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPatch, "/api/events/"+strconv.FormatInt(eventOneID, 10), map[string]any{"end_time": "bad"}, http.StatusBadRequest), "invalid end_time")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPatch, "/api/events/9999999", map[string]any{"title": "x"}, http.StatusNotFound), "no rows")

	doJSONRequest(t, mux, http.MethodPatch, "/api/events/"+strconv.FormatInt(eventOneID, 10), map[string]any{"topic": "Work::Deep"}, http.StatusOK)

	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/events/"+strconv.FormatInt(eventOneID, 10)+"/dependencies", map[string]any{
		"required": true,
	}, http.StatusBadRequest), "event")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPatch, "/api/events/"+strconv.FormatInt(eventOneID, 10)+"/dependencies", map[string]any{}, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodGet, "/api/events/"+strconv.FormatInt(eventOneID, 10)+"/dependencies/nope", nil, http.StatusBadRequest), "invalid depends_on_event_id")

	doJSONRequest(t, mux, http.MethodPost, "/api/events/"+strconv.FormatInt(eventOneID, 10)+"/dependencies", map[string]any{
		"depends_on_event_id": eventTwoID,
		"required":            true,
	}, http.StatusCreated)

	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/events/"+strconv.FormatInt(eventOneID, 10)+"/dependencies/"+strconv.FormatInt(eventTwoID, 10), nil, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodGet, "/api/events/"+strconv.FormatInt(eventOneID, 10)+"/override", nil, http.StatusMethodNotAllowed), "method not allowed")

	req := httptest.NewRequest(http.MethodGet, "/api/events/"+strconv.FormatInt(eventOneID, 10)+"/unknown/path", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status for unknown event path: got=%d want=%d", rec.Code, http.StatusNotFound)
	}
}

func TestRecurrenceAPIValidationBranches(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/events/recurrence", map[string]any{
		"title":      "Invalid Recurrence",
		"domain":     "Parity",
		"subtopic":   "Rec",
		"kind":       "task",
		"start_time": "bad",
		"duration":   "1h",
		"freq":       "weekly",
	}, http.StatusBadRequest), "invalid start_time")

	rule := doJSONRequest(t, mux, http.MethodPost, "/api/events/recurrence", map[string]any{
		"title":      "Validation Recurrence",
		"domain":     "Parity",
		"subtopic":   "Rec",
		"kind":       "task",
		"start_time": "2026-03-03T09:00",
		"duration":   "1h",
		"freq":       "weekly",
		"interval":   1,
		"byday":      "MO,WE",
		"timezone":   "Local",
		"active":     true,
	}, http.StatusCreated)
	ruleID := int64FromAny(t, rule["id"])

	assertErrorContains(t, doJSONRequest(t, mux, http.MethodGet, "/api/events/recurrence/expand", nil, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/events/recurrence/expand", map[string]any{
		"from": "bad",
		"to":   "2026-03-31T23:59",
	}, http.StatusBadRequest), "invalid from")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/events/recurrence/expand", map[string]any{
		"from": "2026-03-01T00:00",
		"to":   "bad",
	}, http.StatusBadRequest), "invalid to")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPatch, "/api/events/recurrence/nope", map[string]any{}, http.StatusBadRequest), "invalid recurrence rule id")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPatch, "/api/events/recurrence/"+strconv.FormatInt(ruleID, 10), map[string]any{
		"start_time": "bad",
	}, http.StatusBadRequest), "invalid start_time")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPatch, "/api/events/recurrence/"+strconv.FormatInt(ruleID, 10), map[string]any{
		"duration": "invalid",
	}, http.StatusBadRequest), "invalid duration")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPatch, "/api/events/recurrence/"+strconv.FormatInt(ruleID, 10), map[string]any{
		"until": "bad",
	}, http.StatusBadRequest), "invalid until")

	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPatch, "/api/events/recurrence/"+strconv.FormatInt(ruleID, 10), map[string]any{
		"clear_legacy": false,
	}, http.StatusBadRequest), "unknown field")

	doJSONRequest(t, mux, http.MethodPatch, "/api/events/recurrence/"+strconv.FormatInt(ruleID, 10), map[string]any{
		"topic":       "Parity::Updated",
		"freq":        "monthly",
		"interval":    2,
		"bymonthday":  15,
		"clear_until": true,
		"active":      false,
		"duration":    "90m",
		"timezone":    "Local",
		"start_time":  "2026-03-03T09:00",
		"title":       "Validation Recurrence Updated",
	}, http.StatusOK)
	doJSONRequest(t, mux, http.MethodPatch, "/api/events/recurrence/"+strconv.FormatInt(ruleID, 10), map[string]any{
		"rrule": "FREQ=DAILY;INTERVAL=1",
		"until": "",
	}, http.StatusOK)

	assertErrorContains(t, doJSONRequest(t, mux, http.MethodGet, "/api/events/recurrence/"+strconv.FormatInt(ruleID, 10), nil, http.StatusMethodNotAllowed), "method not allowed")
	doJSONRequest(t, mux, http.MethodDelete, "/api/events/recurrence/"+strconv.FormatInt(ruleID, 10), nil, http.StatusOK)
}

func TestPlannerAPIValidationBranches(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/planner/status", nil, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodGet, "/api/planner/status?from=2026-03-07T00:00&to=2026-03-06T00:00", nil, http.StatusBadRequest), "invalid range")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodGet, "/api/planner/generate", nil, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/planner/generate", map[string]any{
		"from": "bad",
		"to":   "2026-03-08T00:00",
	}, http.StatusBadRequest), "invalid from")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPatch, "/api/planner/targets", map[string]any{}, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/planner/targets", map[string]any{
		"domain":      "Plan",
		"subtopic":    "General",
		"cadence":     "weekly",
		"occurrences": 2,
	}, http.StatusBadRequest), "duration or hours is required")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/planner/targets", map[string]any{
		"domain":   "Plan",
		"subtopic": "General",
		"cadence":  "weekly",
	}, http.StatusBadRequest), "target duration is required")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/planner/targets", map[string]any{
		"domain":   "Plan",
		"subtopic": "General",
		"cadence":  "weekly",
		"duration": "nope",
	}, http.StatusBadRequest), "invalid duration")

	target := doJSONRequest(t, mux, http.MethodPost, "/api/planner/targets", map[string]any{
		"title":    "Validation Target",
		"domain":   "Plan",
		"subtopic": "General",
		"cadence":  "weekly",
		"duration": "1h",
	}, http.StatusCreated)
	targetID := int64FromAny(t, target["id"])

	assertErrorContains(t, doJSONRequest(t, mux, http.MethodGet, "/api/planner/targets/abc", nil, http.StatusBadRequest), "invalid target id")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodDelete, "/api/planner/targets/999999", nil, http.StatusNotFound), "target not found")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPatch, "/api/planner/targets/999999/active", map[string]any{"active": true}, http.StatusNotFound), "target not found")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodGet, "/api/planner/targets/"+strconv.FormatInt(targetID, 10)+"/unknown", nil, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodDelete, "/api/planner/constraints", nil, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPatch, "/api/planner/constraints", map[string]any{"unknown": true}, http.StatusBadRequest), "unknown field")

	doJSONRequest(t, mux, http.MethodPatch, "/api/planner/constraints", map[string]any{
		"active_weekdays": []string{"monday", "mon", "invalid", "tuesday"},
	}, http.StatusOK)
}

func TestPlannerReportsConfigDeleteWorkflowAPIParity(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)
	config.LoadConfig()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	constraints := doJSONRequest(t, mux, http.MethodGet, "/api/planner/constraints", nil, http.StatusOK)
	if constraints["day_start"] == nil {
		t.Fatalf("expected constraints payload, got: %+v", constraints)
	}

	doJSONRequest(t, mux, http.MethodPatch, "/api/planner/constraints", map[string]any{
		"active_weekdays":         []string{"mon", "tue", "wed"},
		"day_start":               "08:00",
		"day_end":                 "22:00",
		"lunch_start":             "12:30",
		"lunch_duration_minutes":  60,
		"dinner_start":            "19:00",
		"dinner_duration_minutes": 60,
		"max_hours_per_day":       8,
		"timezone":                "Local",
	}, http.StatusOK)

	targetRes := doJSONRequest(t, mux, http.MethodPost, "/api/planner/targets", map[string]any{
		"title":    "Parity Target",
		"domain":   "Plan",
		"subtopic": "General",
		"cadence":  "weekly",
		"duration": "8h",
		"active":   true,
	}, http.StatusCreated)
	targetID := int64FromAny(t, targetRes["id"])

	targetListRes := doJSONRequest(t, mux, http.MethodGet, "/api/planner/targets", nil, http.StatusOK)
	if rows, ok := targetListRes["rows"].([]any); !ok || len(rows) == 0 {
		t.Fatalf("expected targets list, got: %+v", targetListRes)
	}

	doJSONRequest(t, mux, http.MethodPatch, "/api/planner/targets/"+strconv.FormatInt(targetID, 10)+"/active", map[string]any{
		"active": false,
	}, http.StatusOK)

	from := time.Now().AddDate(0, 0, -7).Format("2006-01-02T15:04")
	to := time.Now().AddDate(0, 0, 7).Format("2006-01-02T15:04")

	statusRes := doJSONRequest(t, mux, http.MethodGet, "/api/planner/status?from="+from+"&to="+to, nil, http.StatusOK)
	if statusRes["planned"] == nil {
		t.Fatalf("expected planner status payload, got: %+v", statusRes)
	}

	generateRes := doJSONRequest(t, mux, http.MethodPost, "/api/planner/generate", map[string]any{
		"from":    from,
		"to":      to,
		"dry_run": true,
		"replace": true,
	}, http.StatusOK)
	if generateRes["Diagnostics"] == nil {
		t.Fatalf("expected generation payload, got: %+v", generateRes)
	}

	doJSONRequest(t, mux, http.MethodDelete, "/api/planner/targets/"+strconv.FormatInt(targetID, 10), nil, http.StatusOK)

	statRes := doJSONRequest(t, mux, http.MethodGet, "/api/reports/stat?arg1=day", nil, http.StatusOK)
	if rendered, _ := statRes["rendered"].(string); !strings.Contains(rendered, "Work") {
		t.Fatalf("expected rendered stat report, got: %+v", statRes)
	}
	doJSONRequest(t, mux, http.MethodGet, "/api/reports/adherence?arg1=week", nil, http.StatusOK)
	doJSONRequest(t, mux, http.MethodGet, "/api/reports/plan-vs-actual?arg1=week", nil, http.StatusOK)

	configListRes := doJSONRequest(t, mux, http.MethodGet, "/api/config", nil, http.StatusOK)
	if configListRes["values"] == nil {
		t.Fatalf("expected config values payload, got: %+v", configListRes)
	}
	doJSONRequest(t, mux, http.MethodGet, "/api/config/default_focus", nil, http.StatusOK)
	doJSONRequest(t, mux, http.MethodPatch, "/api/config/default_focus", map[string]any{"value": "30"}, http.StatusOK)
	doJSONRequest(t, mux, http.MethodGet, "/api/config/describe", nil, http.StatusOK)
	doJSONRequest(t, mux, http.MethodGet, "/api/config/describe/default_focus", nil, http.StatusOK)

	startRes := doJSONRequest(t, mux, http.MethodPost, "/api/sessions/start", map[string]any{
		"duration": "25m",
		"domain":   "Delete",
		"subtopic": "One",
	}, http.StatusCreated)
	sessionID := intFromAny(t, startRes["ID"])
	doJSONRequest(t, mux, http.MethodPost, "/api/sessions/stop", map[string]any{}, http.StatusOK)

	recentRes := doJSONRequest(t, mux, http.MethodGet, "/api/delete/sessions/recent?limit=10", nil, http.StatusOK)
	if recentRes["Rows"] == nil {
		t.Fatalf("expected recent sessions payload, got: %+v", recentRes)
	}
	bulkRes := doJSONRequest(t, mux, http.MethodPost, "/api/delete/sessions/bulk", map[string]any{
		"ids": []int{sessionID},
	}, http.StatusOK)
	if deleted := intFromAny(t, bulkRes["deleted"]); deleted < 1 {
		t.Fatalf("expected at least one deleted session, got: %+v", bulkRes)
	}

	workflowRes := doJSONRequest(t, mux, http.MethodGet, "/api/workflow", nil, http.StatusOK)
	if workflowRes["steps"] == nil {
		t.Fatalf("expected workflow payload, got: %+v", workflowRes)
	}
}

func TestReportConfigDeleteWorkflowValidationBranches(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)
	config.LoadConfig()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/reports/stat", nil, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/reports/adherence", nil, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/reports/plan-vs-actual", nil, http.StatusMethodNotAllowed), "method not allowed")
	doJSONRequest(t, mux, http.MethodGet, "/api/reports/stat?arg1=day", nil, http.StatusOK)

	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/config", nil, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodGet, "/api/config/not_a_key", nil, http.StatusBadRequest), "unknown config key")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPut, "/api/config/default_focus", map[string]any{}, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPatch, "/api/config/default_focus", map[string]any{"unknown": "field"}, http.StatusBadRequest), "unknown field")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/config/describe", nil, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodGet, "/api/config/describe/not_a_key", nil, http.StatusBadRequest), "unknown config key")

	req := httptest.NewRequest(http.MethodGet, "/api/config/describe/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status for empty describe key: got=%d want=%d", rec.Code, http.StatusNotFound)
	}

	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/delete/sessions/recent", nil, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodGet, "/api/delete/sessions/recent?limit=-1", nil, http.StatusBadRequest), "invalid limit")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodGet, "/api/delete/sessions/recent?limit=abc", nil, http.StatusBadRequest), "invalid limit")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodGet, "/api/delete/sessions/bulk", nil, http.StatusMethodNotAllowed), "method not allowed")
	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/delete/sessions/bulk", map[string]any{"ids": []int{}}, http.StatusBadRequest), "ids are required")

	noopDelete := doJSONRequest(t, mux, http.MethodPost, "/api/delete/sessions/bulk", map[string]any{"ids": []int{-1, 0}}, http.StatusOK)
	if deleted := intFromAny(t, noopDelete["deleted"]); deleted != 0 {
		t.Fatalf("unexpected delete count for invalid ids: %d", deleted)
	}

	assertErrorContains(t, doJSONRequest(t, mux, http.MethodPost, "/api/workflow", nil, http.StatusMethodNotAllowed), "method not allowed")
}

func TestWebHelperUtilities(t *testing.T) {
	if _, err := parsePositiveInt64("0"); err == nil {
		t.Fatalf("expected parsePositiveInt64 to reject zero")
	}
	if _, err := parsePositiveInt64("-3"); err == nil {
		t.Fatalf("expected parsePositiveInt64 to reject negative values")
	}
	if _, err := parseDurationSeconds(""); err == nil {
		t.Fatalf("expected parseDurationSeconds empty error")
	}
	if _, err := parseDurationSeconds("bad"); err == nil {
		t.Fatalf("expected parseDurationSeconds invalid error")
	}
	if seconds, err := parseDurationSeconds("90m"); err != nil || seconds != 5400 {
		t.Fatalf("unexpected parseDurationSeconds output: seconds=%d err=%v", seconds, err)
	}

	domain, subtopic := normalizeDomainSubtopic("", "", "")
	if domain != topics.DefaultDomain || subtopic != topics.DefaultSubtopic {
		t.Fatalf("unexpected default normalized topic: %s::%s", domain, subtopic)
	}
	if topic := normalizeTopicInput("", "", ""); topic != "" {
		t.Fatalf("unexpected empty normalized topic result: %s", topic)
	}
	if topic := normalizeTopicInput("", "Math", "General"); topic != "Math::General" {
		t.Fatalf("unexpected normalized split topic: %s", topic)
	}
	if topic := normalizeTopicInput("Math::Probability", "", ""); topic != "Math::Probability" {
		t.Fatalf("unexpected normalized combined topic: %s", topic)
	}

	if seconds, err := resolveTargetSecondsWeb("1h", 0); err != nil || seconds != 3600 {
		t.Fatalf("unexpected resolveTargetSecondsWeb duration result: seconds=%d err=%v", seconds, err)
	}
	if seconds, err := resolveTargetSecondsWeb("", 1.5); err != nil || seconds != 5400 {
		t.Fatalf("unexpected resolveTargetSecondsWeb hours result: seconds=%d err=%v", seconds, err)
	}
	if _, err := resolveTargetSecondsWeb("bad", 0); err == nil {
		t.Fatalf("expected resolveTargetSecondsWeb invalid duration error")
	}
	if seconds, err := resolveTargetSecondsWeb("", 0); err != nil || seconds != 0 {
		t.Fatalf("unexpected resolveTargetSecondsWeb zero result: seconds=%d err=%v", seconds, err)
	}

	weekdays := normalizeConstraintWeekdaysWeb([]string{"monday", "mon", "tuesday", "invalid", "sun"})
	if strings.Join(weekdays, ",") != "mon,tue,sun" {
		t.Fatalf("unexpected normalized weekdays: %v", weekdays)
	}

	rangeReq := httptest.NewRequest(http.MethodGet, "/api/planner/status?from=2026-03-01T00:00&to=2026-03-01T01:00", nil)
	from, to, err := plannerRangeFromRequest(rangeReq)
	if err != nil {
		t.Fatalf("plannerRangeFromRequest returned error for valid range: %v", err)
	}
	if !to.After(from) {
		t.Fatalf("expected valid planner range to be increasing")
	}
	if _, _, err := plannerRangeFromRequest(httptest.NewRequest(http.MethodGet, "/api/planner/status?from=bad&to=2026-03-01T01:00", nil)); err == nil {
		t.Fatalf("expected invalid from range error")
	}
	if _, _, err := plannerRangeFromRequest(httptest.NewRequest(http.MethodGet, "/api/planner/status?from=2026-03-01T01:00&to=bad", nil)); err == nil {
		t.Fatalf("expected invalid to range error")
	}

	argCases := []struct {
		rawURL string
		want   []string
	}{
		{rawURL: "/api/reports/stat?arg1=week&arg2=2026-03-01", want: []string{"week", "2026-03-01"}},
		{rawURL: "/api/reports/stat?arg1=month", want: []string{"month"}},
		{rawURL: "/api/reports/stat?timeframe=day", want: []string{"day"}},
		{rawURL: "/api/reports/stat?date=2026-03-01", want: []string{"2026-03-01"}},
		{rawURL: "/api/reports/stat?from=2026-03-01&to=2026-03-08", want: []string{"2026-03-01", "2026-03-08"}},
		{rawURL: "/api/reports/stat", want: nil},
	}
	for _, tc := range argCases {
		got := reportArgsFromQuery(httptest.NewRequest(http.MethodGet, tc.rawURL, nil))
		if len(got) != len(tc.want) {
			t.Fatalf("unexpected report arg length for %s: got=%v want=%v", tc.rawURL, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("unexpected report arg value for %s at %d: got=%v want=%v", tc.rawURL, i, got, tc.want)
			}
		}
	}
}

func doJSONRequest(t *testing.T, mux *http.ServeMux, method, path string, payload any, expectedStatus int) map[string]any {
	t.Helper()

	var body *bytes.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload failed: %v", err)
		}
		body = bytes.NewReader(raw)
	} else {
		body = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != expectedStatus {
		t.Fatalf("unexpected status for %s %s: got=%d want=%d body=%s", method, path, rec.Code, expectedStatus, rec.Body.String())
	}

	out := map[string]any{}
	if strings.TrimSpace(rec.Body.String()) == "" {
		return out
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response failed (%s %s): %v body=%s", method, path, err, rec.Body.String())
	}
	return out
}

func assertErrorContains(t *testing.T, payload map[string]any, want string) {
	t.Helper()

	raw, _ := payload["error"].(string)
	if !strings.Contains(strings.ToLower(raw), strings.ToLower(want)) {
		t.Fatalf("expected error containing %q, got payload=%+v", want, payload)
	}
}

func int64FromAny(t *testing.T, value any) int64 {
	t.Helper()
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	default:
		t.Fatalf("unexpected numeric type: %T (%v)", value, value)
		return 0
	}
}

func intFromAny(t *testing.T, value any) int {
	t.Helper()
	switch v := value.(type) {
	case float64:
		return int(v)
	case int64:
		return int(v)
	case int:
		return v
	default:
		t.Fatalf("unexpected numeric type: %T (%v)", value, value)
		return 0
	}
}
