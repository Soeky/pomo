package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Soeky/pomo/internal/store"
)

func (s *Server) sessionsPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.tmpl.ExecuteTemplate(w, "sessions.html", map[string]any{
		"ActivePage":            "sessions",
		"PageTitle":             "Sessions",
		"IncludeCalendarAssets": false,
	}); err != nil {
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
	res, err := s.store.ListSessions(r.Context(), store.SessionFilter{
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
	sessionType := r.FormValue("type")
	if sessionType != "focus" && sessionType != "break" {
		http.Error(w, "type must be focus or break", http.StatusBadRequest)
		return
	}
	topic := ""
	if sessionType == "focus" {
		parsed, err := parseTopicForm(r, "topic")
		if err != nil {
			http.Error(w, "invalid topic format", http.StatusBadRequest)
			return
		}
		if parsed.Provided {
			topic = parsed.Path.Canonical()
		} else {
			topic = "General::General"
		}
	}
	_, err = s.store.CreateSession(r.Context(), store.Session{
		Type:      sessionType,
		Topic:     topic,
		StartTime: start,
		EndTime:   &end,
	}, "web")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "sessionsChanged")
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
		if err := s.store.DeleteSession(r.Context(), id, "web"); err != nil {
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
		topic := ""
		if sessionType == "focus" {
			parsed, err := parseTopicForm(r, "topic")
			if err != nil {
				http.Error(w, "invalid topic format", http.StatusBadRequest)
				return
			}
			if parsed.Provided {
				topic = parsed.Path.Canonical()
			} else {
				topic = "General::General"
			}
		}
		if err := s.store.UpdateSession(r.Context(), id, store.Session{
			ID:        id,
			Type:      sessionType,
			Topic:     topic,
			StartTime: start,
			EndTime:   &end,
		}, "web"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("HX-Trigger", "sessionsChanged")
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
