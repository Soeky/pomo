package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/store"
)

type mockStore struct {
	listSessionsErr     error
	createSessionErr    error
	updateSessionErr    error
	deleteSessionErr    error
	sessionsInRangeErr  error
	plannedInRangeErr   error
	createPlannedErr    error
	getPlannedErr       error
	updatePlannedErr    error
	deletePlannedErr    error
	getSessionErr       error
	insertAuditErr      error
	getPlannedResult    store.PlannedEvent
	getSessionResult    store.Session
	listSessionsResult  store.SessionListResult
	sessionsInRangeRows []store.Session
	plannedInRangeRows  []store.PlannedEvent
}

func (m mockStore) ListSessions(_ context.Context, _ store.SessionFilter) (store.SessionListResult, error) {
	return m.listSessionsResult, m.listSessionsErr
}
func (m mockStore) GetSessionByID(_ context.Context, _ int) (store.Session, error) {
	return m.getSessionResult, m.getSessionErr
}
func (m mockStore) CreateSession(_ context.Context, _ store.Session, _ string) (int64, error) {
	return 1, m.createSessionErr
}
func (m mockStore) UpdateSession(_ context.Context, _ int, _ store.Session, _ string) error {
	return m.updateSessionErr
}
func (m mockStore) DeleteSession(_ context.Context, _ int, _ string) error {
	return m.deleteSessionErr
}
func (m mockStore) SessionsInRange(_ context.Context, _, _ time.Time) ([]store.Session, error) {
	return m.sessionsInRangeRows, m.sessionsInRangeErr
}
func (m mockStore) PlannedEventsInRange(_ context.Context, _, _ time.Time) ([]store.PlannedEvent, error) {
	return m.plannedInRangeRows, m.plannedInRangeErr
}
func (m mockStore) CreatePlannedEvent(_ context.Context, _ store.PlannedEvent, _ string) (int64, error) {
	return 1, m.createPlannedErr
}
func (m mockStore) GetPlannedEventByID(_ context.Context, _ int) (store.PlannedEvent, error) {
	return m.getPlannedResult, m.getPlannedErr
}
func (m mockStore) UpdatePlannedEvent(_ context.Context, _ int, _ store.PlannedEvent, _ string) error {
	return m.updatePlannedErr
}
func (m mockStore) DeletePlannedEvent(_ context.Context, _ int, _ string) error {
	return m.deletePlannedErr
}
func (m mockStore) InsertAuditLog(_ context.Context, _ string, _ int, _ string, _ any, _ any, _ string) error {
	return m.insertAuditErr
}

func TestSessionsHandlersErrorBranches(t *testing.T) {
	s, err := NewServerWithDeps(ServerConfig{}, ServerDeps{
		Store: mockStore{
			listSessionsErr:  errors.New("list failed"),
			createSessionErr: errors.New("create failed"),
			deleteSessionErr: errors.New("delete failed"),
			updateSessionErr: errors.New("update failed"),
		},
	})
	if err != nil {
		t.Fatalf("NewServerWithDeps failed: %v", err)
	}
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	recList := httptest.NewRecorder()
	mux.ServeHTTP(recList, httptest.NewRequest(http.MethodGet, "/sessions/table", nil))
	if recList.Code != http.StatusInternalServerError {
		t.Fatalf("expected list 500, got %d", recList.Code)
	}

	form := url.Values{"type": {"focus"}, "topic": {"x"}, "start_time": {"2026-02-25T10:00"}, "end_time": {"2026-02-25T10:25"}}
	reqCreate := httptest.NewRequest(http.MethodPost, "/sessions/create", strings.NewReader(form.Encode()))
	reqCreate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recCreate := httptest.NewRecorder()
	mux.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusInternalServerError {
		t.Fatalf("expected create 500, got %d", recCreate.Code)
	}

	recDelete := httptest.NewRecorder()
	mux.ServeHTTP(recDelete, httptest.NewRequest(http.MethodDelete, "/sessions/1", nil))
	if recDelete.Code != http.StatusInternalServerError {
		t.Fatalf("expected delete 500, got %d", recDelete.Code)
	}

	patch := url.Values{"type": {"focus"}, "topic": {"x"}, "start_time": {"2026-02-25T10:00"}, "end_time": {"2026-02-25T10:25"}}
	reqPatch := httptest.NewRequest(http.MethodPatch, "/sessions/1", strings.NewReader(patch.Encode()))
	reqPatch.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recPatch := httptest.NewRecorder()
	mux.ServeHTTP(recPatch, reqPatch)
	if recPatch.Code != http.StatusInternalServerError {
		t.Fatalf("expected patch 500, got %d", recPatch.Code)
	}
}

func TestCalendarHandlersErrorBranches(t *testing.T) {
	s, err := NewServerWithDeps(ServerConfig{}, ServerDeps{
		Store: mockStore{
			sessionsInRangeErr: errors.New("sessions failed"),
			plannedInRangeErr:  errors.New("planned failed"),
			createPlannedErr:   errors.New("create planned failed"),
			getPlannedErr:      errors.New("planned missing"),
			getSessionErr:      errors.New("session missing"),
			updatePlannedErr:   errors.New("update planned failed"),
			updateSessionErr:   errors.New("update session failed"),
			deletePlannedErr:   errors.New("delete planned failed"),
			deleteSessionErr:   errors.New("delete session failed"),
		},
	})
	if err != nil {
		t.Fatalf("NewServerWithDeps failed: %v", err)
	}
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	recGet := httptest.NewRecorder()
	mux.ServeHTTP(recGet, httptest.NewRequest(http.MethodGet, "/calendar/events", nil))
	if recGet.Code != http.StatusInternalServerError {
		t.Fatalf("expected calendar get 500, got %d", recGet.Code)
	}

	// Missing title branch
	noTitle := url.Values{"start_time": {"2026-02-25T10:00"}, "end_time": {"2026-02-25T10:25"}}
	reqNoTitle := httptest.NewRequest(http.MethodPost, "/calendar/events", strings.NewReader(noTitle.Encode()))
	reqNoTitle.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recNoTitle := httptest.NewRecorder()
	mux.ServeHTTP(recNoTitle, reqNoTitle)
	if recNoTitle.Code != http.StatusBadRequest {
		t.Fatalf("expected no-title 400, got %d", recNoTitle.Code)
	}

	// Create planned event store error
	withTitle := url.Values{"title": {"x"}, "start_time": {"2026-02-25T10:00"}, "end_time": {"2026-02-25T10:25"}}
	reqCreate := httptest.NewRequest(http.MethodPost, "/calendar/events", strings.NewReader(withTitle.Encode()))
	reqCreate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recCreate := httptest.NewRecorder()
	mux.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusInternalServerError {
		t.Fatalf("expected create planned 500, got %d", recCreate.Code)
	}

	patch := url.Values{"start_time": {"2026-02-25T10:00"}, "end_time": {"2026-02-25T10:25"}}
	reqPatchPlanned := httptest.NewRequest(http.MethodPatch, "/calendar/events/p-1", strings.NewReader(patch.Encode()))
	reqPatchPlanned.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recPatchPlanned := httptest.NewRecorder()
	mux.ServeHTTP(recPatchPlanned, reqPatchPlanned)
	if recPatchPlanned.Code != http.StatusBadRequest {
		t.Fatalf("expected planned legacy-id 400, got %d", recPatchPlanned.Code)
	}

	reqPatchSession := httptest.NewRequest(http.MethodPatch, "/calendar/events/s-1", strings.NewReader(patch.Encode()))
	reqPatchSession.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recPatchSession := httptest.NewRecorder()
	mux.ServeHTTP(recPatchSession, reqPatchSession)
	if recPatchSession.Code != http.StatusBadRequest {
		t.Fatalf("expected session legacy-id 400, got %d", recPatchSession.Code)
	}

	recDeletePlanned := httptest.NewRecorder()
	mux.ServeHTTP(recDeletePlanned, httptest.NewRequest(http.MethodDelete, "/calendar/events/p-1", nil))
	if recDeletePlanned.Code != http.StatusBadRequest {
		t.Fatalf("expected delete planned legacy-id 400, got %d", recDeletePlanned.Code)
	}
	recDeleteSession := httptest.NewRecorder()
	mux.ServeHTTP(recDeleteSession, httptest.NewRequest(http.MethodDelete, "/calendar/events/s-1", nil))
	if recDeleteSession.Code != http.StatusBadRequest {
		t.Fatalf("expected delete session legacy-id 400, got %d", recDeleteSession.Code)
	}
}
