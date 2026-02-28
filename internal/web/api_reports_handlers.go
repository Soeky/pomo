package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/stats"
)

func (s *Server) apiReportStat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	args := reportArgsFromQuery(r)
	report, err := stats.BuildReport(args, time.Now())
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"args":     args,
		"report":   report,
		"rendered": stats.RenderReport(report),
	})
}

func (s *Server) apiReportAdherence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	args := reportArgsFromQuery(r)
	report, err := stats.BuildAdherenceReport(args, time.Now())
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"args":     args,
		"report":   report,
		"rendered": stats.RenderAdherenceReport(report),
	})
}

func (s *Server) apiReportPlanVsActual(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	args := reportArgsFromQuery(r)
	report, err := stats.BuildPlanVsActualReport(args, time.Now())
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"args":     args,
		"report":   report,
		"rendered": stats.RenderPlanVsActualReport(report),
	})
}

func reportArgsFromQuery(r *http.Request) []string {
	arg1 := strings.TrimSpace(r.URL.Query().Get("arg1"))
	arg2 := strings.TrimSpace(r.URL.Query().Get("arg2"))
	if arg1 != "" && arg2 != "" {
		return []string{arg1, arg2}
	}
	if arg1 != "" {
		return []string{arg1}
	}

	timeframe := strings.TrimSpace(r.URL.Query().Get("timeframe"))
	if timeframe != "" {
		return []string{timeframe}
	}
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	if date != "" {
		return []string{date}
	}
	from := strings.TrimSpace(r.URL.Query().Get("from"))
	to := strings.TrimSpace(r.URL.Query().Get("to"))
	if from != "" && to != "" {
		return []string{from, to}
	}
	return nil
}
