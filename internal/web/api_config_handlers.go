package web

import (
	"net/http"
	"strings"

	"github.com/Soeky/pomo/internal/config"
)

type configSetRequest struct {
	Value string `json:"value"`
}

func (s *Server) apiConfigList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"keys":   config.KnownKeys(),
		"values": config.ListValues(),
	})
}

func (s *Server) apiConfigByKey(w http.ResponseWriter, r *http.Request) {
	key := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/config/"), "/")
	if key == "" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		value, err := config.GetValue(key)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"key":   key,
			"value": value,
		})
	case http.MethodPatch:
		var req configSetRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := config.SetValue(key, req.Value); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		value, _ := config.GetValue(key)
		writeJSON(w, http.StatusOK, map[string]any{
			"updated": true,
			"key":     key,
			"value":   value,
		})
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) apiConfigDescribeAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	out := make(map[string]config.KeyDescription)
	for _, key := range config.KnownKeys() {
		description, err := config.DescribeKey(key)
		if err != nil {
			continue
		}
		out[key] = description
	}
	writeJSON(w, http.StatusOK, map[string]any{"descriptions": out})
}

func (s *Server) apiConfigDescribeKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	key := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/config/describe/"), "/")
	if key == "" {
		http.NotFound(w, r)
		return
	}
	description, err := config.DescribeKey(key)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"key":         key,
		"description": description,
	})
}
