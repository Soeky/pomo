package web

import (
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

	"github.com/Soeky/pomo/internal/db"
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
	if gotType != "break" || gotTopic != "UpdatedTopic" {
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
