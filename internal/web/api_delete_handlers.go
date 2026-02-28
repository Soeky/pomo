package web

import (
	"net/http"
	"strconv"

	"github.com/Soeky/pomo/internal/store"
)

type bulkDeleteRequest struct {
	IDs []int `json:"ids"`
}

func (s *Server) apiDeleteRecentSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 || value > 200 {
			writeJSONError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = value
	}

	result, err := s.store.ListSessions(r.Context(), store.SessionFilter{
		SortBy:   "start_time",
		Order:    "desc",
		Page:     1,
		PageSize: limit,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) apiDeleteSessionsBulk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req bulkDeleteRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.IDs) == 0 {
		writeJSONError(w, http.StatusBadRequest, "ids are required")
		return
	}

	deleted := 0
	for _, id := range req.IDs {
		if id <= 0 {
			continue
		}
		if err := s.store.DeleteSession(r.Context(), id, "web-delete"); err == nil {
			deleted++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deleted": deleted,
	})
}
