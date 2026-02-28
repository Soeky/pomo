package web

import (
	"net/http"
	"strings"
	"time"
)

func (s *Server) dashboardPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	modules := s.dashboard.Definitions()
	if err := s.tmpl.ExecuteTemplate(w, "dashboard.html", map[string]any{
		"Modules":               modules,
		"ModuleWindows":         map[string]string{"upcoming_schedule": "upcoming"},
		"ActivePage":            "dashboard",
		"PageTitle":             "Dashboard",
		"IncludeCalendarAssets": false,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) dashboardModule(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/dashboard/modules/")
	m, ok := s.dashboard.ByID(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	start, end := dashboardWindow(r.URL.Query().Get("window"))
	data, err := m.Load(r.Context(), start, end)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.tmpl.ExecuteTemplate(w, "dashboard-module.html", map[string]any{"ID": m.ID(), "Title": m.Title(), "Data": data}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func dashboardWindow(window string) (time.Time, time.Time) {
	now := time.Now()
	switch strings.ToLower(strings.TrimSpace(window)) {
	case "upcoming":
		return now, now.AddDate(0, 0, 7)
	default:
		return now.AddDate(0, 0, -7), now
	}
}
