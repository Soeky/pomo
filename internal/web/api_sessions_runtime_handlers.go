package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/parse"
	"github.com/Soeky/pomo/internal/session"
	"github.com/Soeky/pomo/internal/status"
	"github.com/Soeky/pomo/internal/topics"
)

type startSessionRequest struct {
	Duration string `json:"duration"`
	Topic    string `json:"topic"`
	Domain   string `json:"domain"`
	Subtopic string `json:"subtopic"`
}

type breakSessionRequest struct {
	Duration string `json:"duration"`
}

type correctSessionRequest struct {
	SessionType  string `json:"session_type"`
	BackDuration string `json:"back_duration"`
	Topic        string `json:"topic"`
	Domain       string `json:"domain"`
	Subtopic     string `json:"subtopic"`
}

func (s *Server) apiStartSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req startSessionRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	args := make([]string, 0, 2)
	if strings.TrimSpace(req.Duration) != "" {
		if _, err := parse.ParseDurationFromArg(req.Duration); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid duration")
			return
		}
		args = append(args, strings.TrimSpace(req.Duration))
	}
	topic := normalizeTopicInput(req.Topic, req.Domain, req.Subtopic)
	if strings.TrimSpace(topic) != "" {
		args = append(args, topic)
	}

	result, err := session.StartFocus(args)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) apiStartBreak(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req breakSessionRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	args := make([]string, 0, 1)
	if strings.TrimSpace(req.Duration) != "" {
		if _, err := parse.ParseDurationFromArg(req.Duration); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid duration")
			return
		}
		args = append(args, strings.TrimSpace(req.Duration))
	}

	result, err := session.StartBreak(args)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) apiStopSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	result, err := session.StopSession()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) apiSessionStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	result, err := status.CurrentStatus(time.Now())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) apiCorrectSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req correctSessionRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	backDuration, err := parse.ParseDurationFromArg(req.BackDuration)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid back_duration")
		return
	}
	correctReq := session.CorrectRequest{
		SessionType:  strings.TrimSpace(req.SessionType),
		BackDuration: backDuration,
		Topic:        normalizeTopicInput(req.Topic, req.Domain, req.Subtopic),
	}

	result, err := session.CorrectSession(time.Now(), correctReq)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func normalizeTopicInput(topic, domain, subtopic string) string {
	topic = strings.TrimSpace(topic)
	if topic != "" {
		if parsed, err := topics.Parse(topic); err == nil {
			return parsed.Canonical()
		}
	}
	if parsed, err := topics.ParseParts(strings.TrimSpace(domain), strings.TrimSpace(subtopic)); err == nil {
		return parsed.Canonical()
	}
	return ""
}
