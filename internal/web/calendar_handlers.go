package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/store"
)

type calendarEvent struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Start    string `json:"start"`
	End      string `json:"end,omitempty"`
	Color    string `json:"color,omitempty"`
	Editable bool   `json:"editable"`
}

func (s *Server) calendarPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.tmpl.ExecuteTemplate(w, "calendar.html", map[string]any{
		"ActivePage":            "calendar",
		"PageTitle":             "Calendar",
		"IncludeCalendarAssets": true,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) calendarEvents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		from := parseTimeOrDefault(r.URL.Query().Get("start"), time.Now().AddDate(0, -1, 0))
		to := parseTimeOrDefault(r.URL.Query().Get("end"), time.Now().AddDate(0, 1, 0))
		sessions, err := s.store.SessionsInRange(r.Context(), from, to)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		plans, err := s.store.PlannedEventsInRange(r.Context(), from, to)
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
		_, err = s.store.CreatePlannedEvent(r.Context(), store.PlannedEvent{
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
			existing, err := s.store.GetPlannedEventByID(r.Context(), numericID)
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
			if err := s.store.UpdatePlannedEvent(r.Context(), numericID, existing, "web"); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		existing, err := s.store.GetSessionByID(r.Context(), numericID)
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
		if err := s.store.UpdateSession(r.Context(), numericID, existing, "web"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if kind == "p" {
			if err := s.store.DeletePlannedEvent(r.Context(), numericID, "web"); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			if err := s.store.DeleteSession(r.Context(), numericID, "web"); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
