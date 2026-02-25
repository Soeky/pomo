package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
