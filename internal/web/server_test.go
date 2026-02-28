package web

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	pomodb "github.com/Soeky/pomo/internal/db"
)

func TestHealthz(t *testing.T) {
	t.Parallel()

	s, err := NewServer(ServerConfig{Host: "127.0.0.1", Port: 0})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "ok" {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestCalendarMethodNotAllowed(t *testing.T) {
	t.Parallel()

	s, err := NewServer(ServerConfig{Host: "127.0.0.1", Port: 0})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/calendar", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestSessionsCreateMethodNotAllowed(t *testing.T) {
	t.Parallel()

	s, err := NewServer(ServerConfig{Host: "127.0.0.1", Port: 0})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/create", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestSQLPageWithoutDB(t *testing.T) {
	t.Parallel()

	s, err := NewServerWithDeps(ServerConfig{Host: "127.0.0.1", Port: 0}, ServerDeps{Store: storeAdapter{}, DB: nil})
	if err != nil {
		t.Fatalf("NewServerWithDeps failed: %v", err)
	}
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sql", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "database is not initialized") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestDashboardPageWithoutDBRendersShell(t *testing.T) {
	t.Parallel()

	s, err := NewServerWithDeps(ServerConfig{Host: "127.0.0.1", Port: 0}, ServerDeps{Store: storeAdapter{}, DB: nil})
	if err != nil {
		t.Fatalf("NewServerWithDeps failed: %v", err)
	}
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Loading") {
		t.Fatalf("expected dashboard shell loading placeholders, got: %s", body)
	}
}

func TestServerRunContextCancel(t *testing.T) {
	s, err := NewServer(ServerConfig{Host: "127.0.0.1", Port: 0})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Run(ctx); err != nil {
		t.Fatalf("Run with canceled context failed: %v", err)
	}
}

func TestWithRequestActivitySkipsHealthz(t *testing.T) {
	t.Parallel()

	var last atomic.Int64
	now := time.Now()
	last.Store(now.UnixNano())

	h := withRequestActivity(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), &last)

	reqHealth := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recHealth := httptest.NewRecorder()
	h.ServeHTTP(recHealth, reqHealth)

	if got := time.Unix(0, last.Load()); got.UnixNano() != now.UnixNano() {
		t.Fatalf("healthz request should not update last activity: got=%v want=%v", got, now)
	}

	reqCalendar := httptest.NewRequest(http.MethodGet, "/calendar", nil)
	recCalendar := httptest.NewRecorder()
	h.ServeHTTP(recCalendar, reqCalendar)

	if got := time.Unix(0, last.Load()); got.UnixNano() <= now.UnixNano() {
		t.Fatalf("non-health request should update last activity: got=%v base=%v", got, now)
	}
}

func TestMonitorAutoSleepCancelsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var last atomic.Int64
	last.Store(time.Now().Add(-2 * time.Second).UnixNano())

	done := make(chan struct{})
	go func() {
		monitorAutoSleep(ctx, cancel, &last, 300*time.Millisecond)
		close(done)
	}()

	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatalf("expected auto-sleep monitor to cancel context")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("auto-sleep monitor did not exit after cancel")
	}
}

func TestFindAvailablePortPreferred(t *testing.T) {
	preferred, err := FindAvailablePort(3210)
	if err != nil {
		t.Fatalf("FindAvailablePort initial failed: %v", err)
	}
	if preferred >= 3299 {
		t.Skip("preferred port at upper bound; no later fallback port exists")
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", preferred))
	if err != nil {
		t.Fatalf("listen on preferred port failed: %v", err)
	}
	defer ln.Close()

	got, err := FindAvailablePort(preferred)
	if err != nil {
		t.Fatalf("FindAvailablePort failed: %v", err)
	}
	if got == preferred {
		t.Fatalf("expected different port when preferred is occupied")
	}
}

func TestDashboardRoutesAndSessionsPage(t *testing.T) {
	opened, err := pomodb.Open(filepath.Join(t.TempDir(), "pomo.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer opened.Close()

	start := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	end := start.Add(25 * time.Minute)
	if err := seedDashboardDB(opened, start, end); err != nil {
		t.Fatalf("seedDashboardDB failed: %v", err)
	}

	s, err := NewServerWithDeps(ServerConfig{Host: "127.0.0.1", Port: 0}, ServerDeps{Store: storeAdapter{}, DB: opened})
	if err != nil {
		t.Fatalf("NewServerWithDeps failed: %v", err)
	}
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard page status: got=%d", rec.Code)
	}

	reqBadPath := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	recBadPath := httptest.NewRecorder()
	mux.ServeHTTP(recBadPath, reqBadPath)
	if recBadPath.Code != http.StatusNotFound {
		t.Fatalf("unexpected unknown path status: %d", recBadPath.Code)
	}

	reqModule := httptest.NewRequest(http.MethodGet, "/dashboard/modules/totals", nil)
	recModule := httptest.NewRecorder()
	mux.ServeHTTP(recModule, reqModule)
	if recModule.Code != http.StatusOK {
		t.Fatalf("dashboard module status: got=%d", recModule.Code)
	}

	reqModule404 := httptest.NewRequest(http.MethodGet, "/dashboard/modules/missing", nil)
	recModule404 := httptest.NewRecorder()
	mux.ServeHTTP(recModule404, reqModule404)
	if recModule404.Code != http.StatusNotFound {
		t.Fatalf("missing dashboard module status: got=%d", recModule404.Code)
	}

	reqSessions := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	recSessions := httptest.NewRecorder()
	mux.ServeHTTP(recSessions, reqSessions)
	if recSessions.Code != http.StatusOK {
		t.Fatalf("sessions page status: got=%d", recSessions.Code)
	}
}

func seedDashboardDB(opened *sql.DB, start, end time.Time) error {
	if _, err := opened.Exec(`INSERT INTO sessions(type, topic, start_time, end_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"focus", "dash", start, end, int((25 * time.Minute).Seconds()), start, start); err != nil {
		return err
	}
	if _, err := opened.Exec(`INSERT INTO planned_events(title, description, start_time, end_time, status, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"pd", "d", start, end, "done", "manual", start, start); err != nil {
		return err
	}
	return nil
}
