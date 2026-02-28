package web

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/store"
	"github.com/Soeky/pomo/internal/web/dashboard"
)

//go:embed templates/*.html
var templatesFS embed.FS

type ServerConfig struct {
	Host string
	Port int
}

type Server struct {
	cfg       ServerConfig
	tmpl      *template.Template
	store     DataStore
	sqlDB     *sql.DB
	dashboard *dashboard.Registry
}

type DataStore interface {
	ListSessions(ctx context.Context, f store.SessionFilter) (store.SessionListResult, error)
	GetSessionByID(ctx context.Context, id int) (store.Session, error)
	CreateSession(ctx context.Context, s store.Session, origin string) (int64, error)
	UpdateSession(ctx context.Context, id int, s store.Session, origin string) error
	DeleteSession(ctx context.Context, id int, origin string) error
	SessionsInRange(ctx context.Context, from, to time.Time) ([]store.Session, error)
	PlannedEventsInRange(ctx context.Context, from, to time.Time) ([]store.PlannedEvent, error)
	CreatePlannedEvent(ctx context.Context, e store.PlannedEvent, origin string) (int64, error)
	GetPlannedEventByID(ctx context.Context, id int) (store.PlannedEvent, error)
	UpdatePlannedEvent(ctx context.Context, id int, e store.PlannedEvent, origin string) error
	DeletePlannedEvent(ctx context.Context, id int, origin string) error
	InsertAuditLog(ctx context.Context, entityType string, entityID int, action string, before any, after any, origin string) error
}

type storeAdapter struct{}

func (storeAdapter) ListSessions(ctx context.Context, f store.SessionFilter) (store.SessionListResult, error) {
	return store.ListSessions(ctx, f)
}
func (storeAdapter) GetSessionByID(ctx context.Context, id int) (store.Session, error) {
	return store.GetSessionByID(ctx, id)
}
func (storeAdapter) CreateSession(ctx context.Context, s store.Session, origin string) (int64, error) {
	return store.CreateSession(ctx, s, origin)
}
func (storeAdapter) UpdateSession(ctx context.Context, id int, s store.Session, origin string) error {
	return store.UpdateSession(ctx, id, s, origin)
}
func (storeAdapter) DeleteSession(ctx context.Context, id int, origin string) error {
	return store.DeleteSession(ctx, id, origin)
}
func (storeAdapter) SessionsInRange(ctx context.Context, from, to time.Time) ([]store.Session, error) {
	return store.SessionsInRange(ctx, from, to)
}
func (storeAdapter) PlannedEventsInRange(ctx context.Context, from, to time.Time) ([]store.PlannedEvent, error) {
	return store.PlannedEventsInRange(ctx, from, to)
}
func (storeAdapter) CreatePlannedEvent(ctx context.Context, e store.PlannedEvent, origin string) (int64, error) {
	return store.CreatePlannedEvent(ctx, e, origin)
}
func (storeAdapter) GetPlannedEventByID(ctx context.Context, id int) (store.PlannedEvent, error) {
	return store.GetPlannedEventByID(ctx, id)
}
func (storeAdapter) UpdatePlannedEvent(ctx context.Context, id int, e store.PlannedEvent, origin string) error {
	return store.UpdatePlannedEvent(ctx, id, e, origin)
}
func (storeAdapter) DeletePlannedEvent(ctx context.Context, id int, origin string) error {
	return store.DeletePlannedEvent(ctx, id, origin)
}
func (storeAdapter) InsertAuditLog(ctx context.Context, entityType string, entityID int, action string, before any, after any, origin string) error {
	return store.InsertAuditLog(ctx, entityType, entityID, action, before, after, origin)
}

type ServerDeps struct {
	Store     DataStore
	DB        *sql.DB
	Dashboard *dashboard.Registry
}

func NewServer(cfg ServerConfig) (*Server, error) {
	return NewServerWithDeps(cfg, ServerDeps{})
}

func NewServerWithDeps(cfg ServerConfig, deps ServerDeps) (*Server, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	st := deps.Store
	if st == nil {
		st = storeAdapter{}
	}
	dbh := deps.DB
	if dbh == nil {
		dbh = db.DB
	}
	dash := deps.Dashboard
	if dash == nil {
		dash = dashboard.NewRegistry(dbh)
	}

	return &Server{
		cfg:       cfg,
		tmpl:      tmpl,
		store:     st,
		sqlDB:     dbh,
		dashboard: dash,
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port),
		Handler: mux,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- httpServer.ListenAndServe() }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", s.healthz)
	mux.HandleFunc("/", s.dashboardPage)
	mux.HandleFunc("/dashboard/modules/", s.dashboardModule)
	mux.HandleFunc("/calendar", s.calendarPage)
	mux.HandleFunc("/calendar/events", s.calendarEvents)
	mux.HandleFunc("/calendar/events/", s.calendarEventByID)
	mux.HandleFunc("/calendar/recurrence", s.recurrenceRules)
	mux.HandleFunc("/calendar/recurrence/", s.recurrenceRuleByID)
	mux.HandleFunc("/sessions", s.sessionsPage)
	mux.HandleFunc("/sessions/table", s.sessionsTable)
	mux.HandleFunc("/sessions/", s.sessionByID)
	mux.HandleFunc("/sessions/create", s.createSession)
	mux.HandleFunc("/sql", s.sqlPage)
	mux.HandleFunc("/sql/query", s.sqlQuery)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
