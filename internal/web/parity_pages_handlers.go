package web

import "net/http"

func (s *Server) eventsPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.tmpl.ExecuteTemplate(w, "events.html", map[string]any{
		"ActivePage":            "events",
		"PageTitle":             "Events",
		"IncludeCalendarAssets": false,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) dependenciesPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.tmpl.ExecuteTemplate(w, "dependencies.html", map[string]any{
		"ActivePage":            "dependencies",
		"PageTitle":             "Dependencies",
		"IncludeCalendarAssets": false,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) plannerPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.tmpl.ExecuteTemplate(w, "planner.html", map[string]any{
		"ActivePage":            "planner",
		"PageTitle":             "Planner",
		"IncludeCalendarAssets": false,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) reportsPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.tmpl.ExecuteTemplate(w, "reports.html", map[string]any{
		"ActivePage":            "reports",
		"PageTitle":             "Reports",
		"IncludeCalendarAssets": false,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) configPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.tmpl.ExecuteTemplate(w, "config.html", map[string]any{
		"ActivePage":            "config",
		"PageTitle":             "Config",
		"IncludeCalendarAssets": false,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) deletePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.tmpl.ExecuteTemplate(w, "delete_sessions.html", map[string]any{
		"ActivePage":            "delete",
		"PageTitle":             "Delete Sessions",
		"IncludeCalendarAssets": false,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) workflowPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.tmpl.ExecuteTemplate(w, "workflow.html", map[string]any{
		"ActivePage":            "workflow",
		"PageTitle":             "Workflow",
		"IncludeCalendarAssets": false,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
