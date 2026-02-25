package web

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"
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
	cfg  ServerConfig
	tmpl *template.Template
}

func NewServer(cfg ServerConfig) (*Server, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{cfg: cfg, tmpl: tmpl}, nil
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

func (s *Server) dashboardPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	start := time.Now().AddDate(0, 0, -7)
	end := time.Now()
	modules, err := dashboard.All(r.Context(), start, end)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.tmpl.ExecuteTemplate(w, "dashboard.html", map[string]any{"Modules": modules}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) dashboardModule(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/dashboard/modules/")
	m, ok := dashboard.ByID(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	start := time.Now().AddDate(0, 0, -7)
	end := time.Now()
	data, err := m.Load(r.Context(), start, end)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.tmpl.ExecuteTemplate(w, "dashboard-module.html", map[string]any{"ID": m.ID(), "Title": m.Title(), "Data": data}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) calendarPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.tmpl.ExecuteTemplate(w, "calendar.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type calendarEvent struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Start    string `json:"start"`
	End      string `json:"end,omitempty"`
	Color    string `json:"color,omitempty"`
	Editable bool   `json:"editable"`
}

func (s *Server) calendarEvents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		from := parseTimeOrDefault(r.URL.Query().Get("start"), time.Now().AddDate(0, -1, 0))
		to := parseTimeOrDefault(r.URL.Query().Get("end"), time.Now().AddDate(0, 1, 0))
		sessions, err := store.SessionsInRange(r.Context(), from, to)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		plans, err := store.PlannedEventsInRange(r.Context(), from, to)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		events := make([]calendarEvent, 0, len(sessions)+len(plans))
		for _, sess := range sessions {
			end := sess.StartTime.Add(time.Duration(sess.DurationSec) * time.Second)
			if sess.EndTime != nil {
				end = *sess.EndTime
			}
			title := sess.Topic
			if title == "" {
				title = "break"
			}
			color := "#2f855a"
			if sess.Type == "break" {
				color = "#718096"
			}
			events = append(events, calendarEvent{
				ID:       fmt.Sprintf("s-%d", sess.ID),
				Title:    title,
				Start:    sess.StartTime.Format(time.RFC3339),
				End:      end.Format(time.RFC3339),
				Color:    color,
				Editable: true,
			})
		}
		for _, p := range plans {
			events = append(events, calendarEvent{
				ID:       fmt.Sprintf("p-%d", p.ID),
				Title:    p.Title,
				Start:    p.StartTime.Format(time.RFC3339),
				End:      p.EndTime.Format(time.RFC3339),
				Color:    "#2b6cb0",
				Editable: true,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(events)
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
		title := strings.TrimSpace(r.FormValue("title"))
		if title == "" {
			http.Error(w, "title required", http.StatusBadRequest)
			return
		}
		_, err = store.CreatePlannedEvent(r.Context(), store.PlannedEvent{
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
		if kind == "p" {
			existing, err := store.GetPlannedEventByID(r.Context(), numericID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			title := r.FormValue("title")
			if title == "" {
				title = existing.Title
			}
			existing.Title = title
			existing.StartTime = start
			existing.EndTime = end
			if err := store.UpdatePlannedEvent(r.Context(), numericID, existing, "web"); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		existing, err := store.GetSessionByID(r.Context(), numericID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		existing.StartTime = start
		existing.EndTime = &end
		title := strings.TrimSpace(r.FormValue("title"))
		if title != "" {
			existing.Topic = title
		}
		if err := store.UpdateSession(r.Context(), numericID, existing, "web"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if kind == "p" {
			if err := store.DeletePlannedEvent(r.Context(), numericID, "web"); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			if err := store.DeleteSession(r.Context(), numericID, "web"); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) sessionsPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.tmpl.ExecuteTemplate(w, "sessions.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) sessionsTable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	res, err := store.ListSessions(r.Context(), store.SessionFilter{
		Query:    r.URL.Query().Get("q"),
		Type:     r.URL.Query().Get("type"),
		SortBy:   r.URL.Query().Get("sort"),
		Order:    r.URL.Query().Get("order"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.tmpl.ExecuteTemplate(w, "sessions-table.html", res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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
	_, err = store.CreateSession(r.Context(), store.Session{
		Type:      r.FormValue("type"),
		Topic:     r.FormValue("topic"),
		StartTime: start,
		EndTime:   &end,
	}, "web")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) sessionByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/sessions/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodDelete:
		if err := store.DeleteSession(r.Context(), id, "web"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
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
		sessionType := r.FormValue("type")
		if sessionType != "focus" && sessionType != "break" {
			http.Error(w, "type must be focus or break", http.StatusBadRequest)
			return
		}
		if err := store.UpdateSession(r.Context(), id, store.Session{
			ID:        id,
			Type:      sessionType,
			Topic:     r.FormValue("topic"),
			StartTime: start,
			EndTime:   &end,
		}, "web"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) sqlPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tables, err := listTables(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.tmpl.ExecuteTemplate(w, "sql.html", map[string]any{"Tables": tables}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type sqlResult struct {
	Columns []string
	Rows    [][]string
	Message string
	Error   string
}

func (s *Server) sqlQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	query := strings.TrimSpace(r.FormValue("query"))
	writeMode := r.FormValue("write_mode") == "1"
	result := sqlResult{}

	if query == "" {
		result.Error = "query is required"
		s.renderSQLResult(w, result)
		return
	}
	if strings.Count(query, ";") > 1 || (strings.Count(query, ";") == 1 && !strings.HasSuffix(query, ";")) {
		result.Error = "only one SQL statement is allowed"
		s.renderSQLResult(w, result)
		return
	}
	query = strings.TrimSuffix(query, ";")

	if !writeMode && !isReadOnlySQL(query) {
		result.Error = "read-only mode blocks mutating SQL; enable write mode to continue"
		s.renderSQLResult(w, result)
		return
	}

	if isReadOnlySQL(query) {
		rows, err := db.DB.QueryContext(r.Context(), query)
		if err != nil {
			result.Error = err.Error()
			s.renderSQLResult(w, result)
			return
		}
		defer rows.Close()
		cols, err := rows.Columns()
		if err != nil {
			result.Error = err.Error()
			s.renderSQLResult(w, result)
			return
		}
		result.Columns = cols
		for rows.Next() {
			scans := make([]any, len(cols))
			vals := make([]sql.NullString, len(cols))
			for i := range vals {
				scans[i] = &vals[i]
			}
			if err := rows.Scan(scans...); err != nil {
				result.Error = err.Error()
				s.renderSQLResult(w, result)
				return
			}
			line := make([]string, len(cols))
			for i := range vals {
				if vals[i].Valid {
					line[i] = vals[i].String
				} else {
					line[i] = "NULL"
				}
			}
			result.Rows = append(result.Rows, line)
		}
		result.Message = fmt.Sprintf("%d rows", len(result.Rows))
		s.renderSQLResult(w, result)
		return
	}

	execRes, err := db.DB.ExecContext(r.Context(), query)
	if err != nil {
		result.Error = err.Error()
		s.renderSQLResult(w, result)
		return
	}
	affected, _ := execRes.RowsAffected()
	result.Message = fmt.Sprintf("write query executed: %d row(s) affected", affected)
	_ = store.InsertAuditLog(r.Context(), "sql", 0, "execute", map[string]string{"query": query}, map[string]any{"rows_affected": affected}, "web")
	s.renderSQLResult(w, result)
}

func (s *Server) renderSQLResult(w http.ResponseWriter, r sqlResult) {
	if err := s.tmpl.ExecuteTemplate(w, "sql-result.html", r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func parseTimeOrDefault(v string, fallback time.Time) time.Time {
	t, err := parseAnyTime(v)
	if err != nil {
		return fallback
	}
	return t
}

func parseAnyTime(v string) (time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	layouts := []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, v); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time: %s", v)
}

func parsePrefixedID(id string) (string, int, error) {
	parts := strings.SplitN(id, "-", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("id must be prefixed like p-1 or s-2")
	}
	if parts[0] != "p" && parts[0] != "s" {
		return "", 0, fmt.Errorf("unsupported id prefix")
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid numeric id")
	}
	return parts[0], n, nil
}

var readOnlyPrefix = regexp.MustCompile(`(?i)^\s*(select|with|explain|pragma)\b`)
var blockedMutating = regexp.MustCompile(`(?i)\b(insert|update|delete|alter|drop|create|replace|truncate|attach|vacuum|reindex)\b`)

func isReadOnlySQL(q string) bool {
	if !readOnlyPrefix.MatchString(q) {
		return false
	}
	return !blockedMutating.MatchString(q)
}

func listTables(ctx context.Context) ([]string, error) {
	rows, err := db.DB.QueryContext(ctx, `
		SELECT name
		FROM sqlite_master
		WHERE type='table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}
