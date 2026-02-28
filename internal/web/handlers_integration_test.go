package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/events"
)

func TestCalendarEventsGetEmpty(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/calendar/events", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusOK)
	}

	var events []calendarEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no events, got %d", len(events))
	}
}

func TestCalendarPageGet(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/calendar", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected calendar page status: %d", rec.Code)
	}
}

func TestCreateSessionAndListTable(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	form := url.Values{
		"type":       {"focus"},
		"topic":      {"ProjectX"},
		"start_time": {"2026-02-25T10:00"},
		"end_time":   {"2026-02-25T10:25"},
	}
	req := httptest.NewRequest(http.MethodPost, "/sessions/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("unexpected create status: got=%d want=%d", rec.Code, http.StatusCreated)
	}

	reqTable := httptest.NewRequest(http.MethodGet, "/sessions/table", nil)
	recTable := httptest.NewRecorder()
	mux.ServeHTTP(recTable, reqTable)

	if recTable.Code != http.StatusOK {
		t.Fatalf("unexpected table status: got=%d want=%d", recTable.Code, http.StatusOK)
	}
	body := recTable.Body.String()
	if !strings.Contains(body, "ProjectX") {
		t.Fatalf("expected sessions table to contain created topic")
	}
}

func TestCreateSessionWithSplitTopicFields(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	form := url.Values{
		"type":       {"focus"},
		"domain":     {"Math"},
		"subtopic":   {"Discrete Probability"},
		"start_time": {"2026-02-25T10:00"},
		"end_time":   {"2026-02-25T10:25"},
	}
	req := httptest.NewRequest(http.MethodPost, "/sessions/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("unexpected create status: got=%d want=%d", rec.Code, http.StatusCreated)
	}

	var topic string
	if err := opened.QueryRow(`SELECT topic FROM sessions ORDER BY id DESC LIMIT 1`).Scan(&topic); err != nil {
		t.Fatalf("query created split-topic session failed: %v", err)
	}
	if topic != "Math::Discrete Probability" {
		t.Fatalf("unexpected canonical split-topic value: %s", topic)
	}
}

func TestSessionsTableMethodNotAllowed(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/sessions/table", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected sessions table status: %d", rec.Code)
	}
}

func TestSessionPatchAndDelete(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	id := insertSession(t, opened, "focus", "OldTopic", "2026-02-25T10:00:00Z", "2026-02-25T10:25:00Z")
	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	patchForm := url.Values{
		"type":       {"break"},
		"topic":      {"UpdatedTopic"},
		"start_time": {"2026-02-25T10:05"},
		"end_time":   {"2026-02-25T10:15"},
	}
	patchReq := httptest.NewRequest(http.MethodPatch, "/sessions/"+id, strings.NewReader(patchForm.Encode()))
	patchReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	patchRec := httptest.NewRecorder()
	mux.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusNoContent {
		t.Fatalf("unexpected patch status: got=%d want=%d", patchRec.Code, http.StatusNoContent)
	}

	var gotType, gotTopic string
	if err := opened.QueryRow(`SELECT type, topic FROM sessions WHERE id = ?`, id).Scan(&gotType, &gotTopic); err != nil {
		t.Fatalf("query patched session: %v", err)
	}
	if gotType != "break" || gotTopic != "" {
		t.Fatalf("unexpected patched values: type=%s topic=%s", gotType, gotTopic)
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/sessions/"+id, nil)
	delRec := httptest.NewRecorder()
	mux.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("unexpected delete status: got=%d want=%d", delRec.Code, http.StatusNoContent)
	}
}

func TestCreateSessionValidation(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	form := url.Values{
		"type":     {"focus"},
		"topic":    {"NoStart"},
		"end_time": {"2026-02-25T10:25"},
	}
	req := httptest.NewRequest(http.MethodPost, "/sessions/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected create status: got=%d want=%d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "invalid start_time") {
		t.Fatalf("expected invalid start_time error, got: %s", rec.Body.String())
	}
}

func TestSessionByIDValidationBranches(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	id := insertSession(t, opened, "focus", "T", "2026-02-25T10:00:00Z", "2026-02-25T10:25:00Z")
	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	badIDReq := httptest.NewRequest(http.MethodPatch, "/sessions/not-an-int", nil)
	badIDRec := httptest.NewRecorder()
	mux.ServeHTTP(badIDRec, badIDReq)
	if badIDRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected bad id status: %d", badIDRec.Code)
	}

	badType := url.Values{
		"type":       {"invalid"},
		"topic":      {"X"},
		"start_time": {"2026-02-25T10:05"},
		"end_time":   {"2026-02-25T10:15"},
	}
	badTypeReq := httptest.NewRequest(http.MethodPatch, "/sessions/"+id, strings.NewReader(badType.Encode()))
	badTypeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badTypeRec := httptest.NewRecorder()
	mux.ServeHTTP(badTypeRec, badTypeReq)
	if badTypeRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected bad type status: %d", badTypeRec.Code)
	}
}

func TestSQLQueryReadOnlyGuard(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	form := url.Values{"query": {"UPDATE sessions SET topic='x'"}}
	req := httptest.NewRequest(http.MethodPost, "/sql/query", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected sql status: got=%d want=%d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "read-only mode blocks mutating SQL") {
		t.Fatalf("expected read-only guard error, got: %s", rec.Body.String())
	}
}

func TestSQLQueryBranches(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	emptyReq := httptest.NewRequest(http.MethodPost, "/sql/query", strings.NewReader(url.Values{}.Encode()))
	emptyReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	emptyRec := httptest.NewRecorder()
	mux.ServeHTTP(emptyRec, emptyReq)
	if emptyRec.Code != http.StatusOK || !strings.Contains(emptyRec.Body.String(), "query is required") {
		t.Fatalf("unexpected empty query response: code=%d body=%s", emptyRec.Code, emptyRec.Body.String())
	}

	multi := url.Values{"query": {"SELECT 1; SELECT 2"}}
	multiReq := httptest.NewRequest(http.MethodPost, "/sql/query", strings.NewReader(multi.Encode()))
	multiReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	multiRec := httptest.NewRecorder()
	mux.ServeHTTP(multiRec, multiReq)
	if !strings.Contains(multiRec.Body.String(), "only one SQL statement is allowed") {
		t.Fatalf("expected single statement guard, got: %s", multiRec.Body.String())
	}

	write := url.Values{"query": {"UPDATE sessions SET topic='x'"}, "write_mode": {"1"}}
	writeReq := httptest.NewRequest(http.MethodPost, "/sql/query", strings.NewReader(write.Encode()))
	writeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	writeRec := httptest.NewRecorder()
	mux.ServeHTTP(writeRec, writeReq)
	if writeRec.Code != http.StatusOK || !strings.Contains(writeRec.Body.String(), "write query executed") {
		t.Fatalf("unexpected write query response: code=%d body=%s", writeRec.Code, writeRec.Body.String())
	}

	badMethodReq := httptest.NewRequest(http.MethodGet, "/sql/query", nil)
	badMethodRec := httptest.NewRecorder()
	mux.ServeHTTP(badMethodRec, badMethodReq)
	if badMethodRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected method status for sql/query: %d", badMethodRec.Code)
	}
}

func TestSQLQuerySelect(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	form := url.Values{"query": {"SELECT 1 as n"}}
	req := httptest.NewRequest(http.MethodPost, "/sql/query", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected sql status: got=%d want=%d", rec.Code, http.StatusOK)
	}
	body, _ := io.ReadAll(rec.Body)
	sBody := string(body)
	if !strings.Contains(sBody, "1 rows") {
		t.Fatalf("expected rows count in SQL result, got: %s", sBody)
	}
	if !strings.Contains(sBody, "<th>n</th>") {
		t.Fatalf("expected SQL column header in output, got: %s", sBody)
	}
}

func TestCalendarPlannedEventCreatePatchDelete(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	createForm := url.Values{
		"title":       {"Study Block"},
		"description": {"prep"},
		"start_time":  {"2026-02-25T11:00"},
		"end_time":    {"2026-02-25T12:00"},
	}
	createReq := httptest.NewRequest(http.MethodPost, "/calendar/events", strings.NewReader(createForm.Encode()))
	createReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("unexpected create planned status: got=%d want=%d", createRec.Code, http.StatusCreated)
	}

	var plannedID int
	if err := opened.QueryRow(`SELECT id FROM planned_events WHERE title = ?`, "Study Block").Scan(&plannedID); err != nil {
		t.Fatalf("query planned event id: %v", err)
	}

	patchForm := url.Values{
		"title":      {"Updated Study"},
		"start_time": {"2026-02-25T11:15"},
		"end_time":   {"2026-02-25T12:15"},
	}
	patchReq := httptest.NewRequest(http.MethodPatch, "/calendar/events/p-"+strconv.Itoa(plannedID), strings.NewReader(patchForm.Encode()))
	patchReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	patchRec := httptest.NewRecorder()
	mux.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusNoContent {
		t.Fatalf("unexpected patch planned status: got=%d want=%d", patchRec.Code, http.StatusNoContent)
	}

	var title string
	if err := opened.QueryRow(`SELECT title FROM planned_events WHERE id = ?`, plannedID).Scan(&title); err != nil {
		t.Fatalf("query patched planned: %v", err)
	}
	if title != "Updated Study" {
		t.Fatalf("unexpected planned title: %s", title)
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/calendar/events/p-"+strconv.Itoa(plannedID), nil)
	delRec := httptest.NewRecorder()
	mux.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("unexpected delete planned status: got=%d want=%d", delRec.Code, http.StatusNoContent)
	}
}

func TestCalendarPlannedEventCreateWithSplitTopicFields(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	createForm := url.Values{
		"domain":      {"Physics"},
		"subtopic":    {"Quantum Mechanics"},
		"description": {"prep"},
		"start_time":  {"2026-02-25T11:00"},
		"end_time":    {"2026-02-25T12:00"},
	}
	createReq := httptest.NewRequest(http.MethodPost, "/calendar/events", strings.NewReader(createForm.Encode()))
	createReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("unexpected create planned status: got=%d want=%d", createRec.Code, http.StatusCreated)
	}

	var title string
	if err := opened.QueryRow(`SELECT title FROM planned_events ORDER BY id DESC LIMIT 1`).Scan(&title); err != nil {
		t.Fatalf("query split-topic planned title failed: %v", err)
	}
	if title != "Physics::Quantum Mechanics" {
		t.Fatalf("unexpected split-topic planned title: %s", title)
	}
}

func TestCalendarPlannedEventCreateWithCombinedTopicField(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	createForm := url.Values{
		"topic":       {"Chemistry::Organic"},
		"description": {"prep"},
		"start_time":  {"2026-02-25T11:00"},
		"end_time":    {"2026-02-25T12:00"},
	}
	createReq := httptest.NewRequest(http.MethodPost, "/calendar/events", strings.NewReader(createForm.Encode()))
	createReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("unexpected create planned status: got=%d want=%d", createRec.Code, http.StatusCreated)
	}

	var title string
	if err := opened.QueryRow(`SELECT title FROM planned_events ORDER BY id DESC LIMIT 1`).Scan(&title); err != nil {
		t.Fatalf("query combined-topic planned title failed: %v", err)
	}
	if title != "Chemistry::Organic" {
		t.Fatalf("unexpected combined-topic planned title: %s", title)
	}
}

func TestRecurrenceRuleCRUDEndpoints(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	createForm := url.Values{
		"title":            {"Morning Review"},
		"domain":           {"Planning"},
		"subtopic":         {"General"},
		"start_time":       {"2026-03-01T09:00"},
		"duration_minutes": {"45"},
		"freq":             {"weekly"},
		"interval":         {"1"},
		"byday":            {"MO,WE"},
	}
	createReq := httptest.NewRequest(http.MethodPost, "/calendar/recurrence", strings.NewReader(createForm.Encode()))
	createReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("unexpected recurrence create status: got=%d want=%d", createRec.Code, http.StatusCreated)
	}

	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("parse recurrence create json failed: %v", err)
	}
	idVal, ok := created["id"].(float64)
	if !ok || idVal <= 0 {
		t.Fatalf("missing recurrence id in create response: %v", created)
	}
	id := int64(idVal)

	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/calendar/recurrence", nil))
	if listRec.Code != http.StatusOK {
		t.Fatalf("unexpected recurrence list status: %d", listRec.Code)
	}
	var listed []events.RecurrenceRule
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("parse recurrence list json failed: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != id {
		t.Fatalf("unexpected recurrence list rows: %+v", listed)
	}

	patchForm := url.Values{"title": {"Morning Deep Review"}}
	patchReq := httptest.NewRequest(http.MethodPatch, "/calendar/recurrence/"+strconv.FormatInt(id, 10), strings.NewReader(patchForm.Encode()))
	patchReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	patchRec := httptest.NewRecorder()
	mux.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusNoContent {
		t.Fatalf("unexpected recurrence patch status: got=%d want=%d", patchRec.Code, http.StatusNoContent)
	}

	var title string
	if err := opened.QueryRow(`SELECT title FROM recurrence_rules WHERE id = ?`, id).Scan(&title); err != nil {
		t.Fatalf("query patched recurrence rule failed: %v", err)
	}
	if title != "Morning Deep Review" {
		t.Fatalf("unexpected patched recurrence title: %s", title)
	}

	delRec := httptest.NewRecorder()
	mux.ServeHTTP(delRec, httptest.NewRequest(http.MethodDelete, "/calendar/recurrence/"+strconv.FormatInt(id, 10), nil))
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("unexpected recurrence delete status: got=%d want=%d", delRec.Code, http.StatusNoContent)
	}
}

func TestCalendarEventsMixedSources(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	_ = insertSession(t, opened, "focus", "ProjectX::General", "2026-03-02T10:00:00Z", "2026-03-02T10:25:00Z")
	if _, err := opened.Exec(`
		INSERT INTO planned_events(title, description, start_time, end_time, status, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"Plan Item", "desc", "2026-03-02T11:00:00Z", "2026-03-02T12:00:00Z", "planned", "manual", "2026-03-02T11:00:00Z", "2026-03-02T11:00:00Z"); err != nil {
		t.Fatalf("insert planned event failed: %v", err)
	}

	manualID, err := events.Create(context.Background(), events.Event{
		Kind:      "task",
		Title:     "Canonical Event",
		Domain:    "Math",
		Subtopic:  "General",
		StartTime: time.Date(2026, 3, 2, 13, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 3, 2, 14, 0, 0, 0, time.UTC),
		Layer:     "planned",
		Status:    "planned",
		Source:    "manual",
	})
	if err != nil {
		t.Fatalf("create canonical event failed: %v", err)
	}

	rrule, err := events.BuildRRule(events.RecurrenceSpec{Freq: "daily", Interval: 1})
	if err != nil {
		t.Fatalf("build rrule failed: %v", err)
	}
	if _, err := events.CreateRecurrenceRule(context.Background(), events.RecurrenceRule{
		Title:       "Recurring Focus",
		Domain:      "Physics",
		Subtopic:    "General",
		Kind:        "focus",
		DurationSec: 1800,
		RRule:       rrule,
		Timezone:    "UTC",
		StartDate:   time.Date(2026, 3, 2, 15, 0, 0, 0, time.UTC),
		Active:      true,
	}); err != nil {
		t.Fatalf("create recurrence rule failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/calendar/events?start=2026-03-02T00:00:00Z&end=2026-03-03T23:59:59Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected calendar events status: %d", rec.Code)
	}
	var rows []calendarEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("parse calendar events json failed: %v", err)
	}
	foundSession, foundPlanned, foundCanonical, foundRecurring := false, false, false, false
	for _, row := range rows {
		if strings.HasPrefix(row.ID, "s-") {
			foundSession = true
		}
		if strings.HasPrefix(row.ID, "p-") {
			foundPlanned = true
		}
		if row.ID == "e-"+strconv.FormatInt(manualID, 10) {
			foundCanonical = true
		}
		if strings.HasPrefix(row.ID, "e-") && strings.Contains(row.Title, "Recurring Focus") {
			foundRecurring = true
		}
	}
	if !foundSession || !foundPlanned || !foundCanonical || !foundRecurring {
		t.Fatalf("missing mixed source coverage: session=%v planned=%v canonical=%v recurring=%v rows=%+v",
			foundSession, foundPlanned, foundCanonical, foundRecurring, rows)
	}
}

func TestCalendarCanonicalEventPatchDelete(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	start := time.Date(2026, 3, 4, 9, 0, 0, 0, time.UTC)
	end := start.Add(45 * time.Minute)
	eventID, err := events.Create(context.Background(), events.Event{
		Kind:      "task",
		Title:     "Canonical Patch Target",
		Domain:    "Math",
		Subtopic:  "General",
		StartTime: start,
		EndTime:   end,
		Layer:     "planned",
		Status:    "planned",
		Source:    "manual",
	})
	if err != nil {
		t.Fatalf("create canonical event failed: %v", err)
	}

	patch := url.Values{
		"title":      {"Updated Canonical Event"},
		"start_time": {"2026-03-04T09:30:00Z"},
		"end_time":   {"2026-03-04T10:15:00Z"},
	}
	patchReq := httptest.NewRequest(http.MethodPatch, "/calendar/events/e-"+strconv.FormatInt(eventID, 10), strings.NewReader(patch.Encode()))
	patchReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	patchRec := httptest.NewRecorder()
	mux.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusNoContent {
		t.Fatalf("unexpected canonical event patch status: got=%d want=%d", patchRec.Code, http.StatusNoContent)
	}

	var title string
	var gotStart, gotEnd time.Time
	if err := opened.QueryRow(`SELECT title, start_time, end_time FROM events WHERE id = ?`, eventID).Scan(&title, &gotStart, &gotEnd); err != nil {
		t.Fatalf("query canonical event after patch failed: %v", err)
	}
	if title != "Updated Canonical Event" {
		t.Fatalf("unexpected patched canonical title: %s", title)
	}
	if gotStart.UTC().Format(time.RFC3339) != "2026-03-04T09:30:00Z" || gotEnd.UTC().Format(time.RFC3339) != "2026-03-04T10:15:00Z" {
		t.Fatalf("unexpected patched canonical times: start=%s end=%s", gotStart.UTC().Format(time.RFC3339), gotEnd.UTC().Format(time.RFC3339))
	}

	delRec := httptest.NewRecorder()
	mux.ServeHTTP(delRec, httptest.NewRequest(http.MethodDelete, "/calendar/events/e-"+strconv.FormatInt(eventID, 10), nil))
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("unexpected canonical event delete status: got=%d want=%d", delRec.Code, http.StatusNoContent)
	}

	var remaining int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM events WHERE id = ?`, eventID).Scan(&remaining); err != nil {
		t.Fatalf("count canonical event after delete failed: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected canonical event to be deleted, remaining=%d", remaining)
	}
}

func TestCalendarEventByIDMethodNotAllowed(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/calendar/events/p-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected method status: %d", rec.Code)
	}
}

func TestCalendarEventByIDValidation(t *testing.T) {
	opened := openWebTestDB(t)
	defer opened.Close()

	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	noIDReq := httptest.NewRequest(http.MethodPatch, "/calendar/events/", nil)
	noIDRec := httptest.NewRecorder()
	mux.ServeHTTP(noIDRec, noIDReq)
	if noIDRec.Code != http.StatusNotFound {
		t.Fatalf("unexpected no-id status: %d", noIDRec.Code)
	}

	badIDReq := httptest.NewRequest(http.MethodPatch, "/calendar/events/invalid", nil)
	badIDRec := httptest.NewRecorder()
	mux.ServeHTTP(badIDRec, badIDReq)
	if badIDRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected bad-id status: %d", badIDRec.Code)
	}
}

func openWebTestDB(t *testing.T) *sql.DB {
	t.Helper()

	opened, err := db.Open(filepath.Join(t.TempDir(), "pomo.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	prev := db.DB
	db.DB = opened
	t.Cleanup(func() {
		db.DB = prev
	})
	return opened
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	s, err := NewServer(ServerConfig{Host: "127.0.0.1", Port: 0})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	return s
}

func insertSession(t *testing.T, opened *sql.DB, typ, topic, start, end string) string {
	t.Helper()

	res, err := opened.Exec(`
		INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?)`,
		typ, topic, start, end, 1500, start, end)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("read inserted id: %v", err)
	}
	return strconv.Itoa(int(id))
}
