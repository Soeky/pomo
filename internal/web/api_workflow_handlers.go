package web

import "net/http"

func (s *Server) apiWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"title": "Recommended Daily Workflow",
		"steps": []map[string]any{
			{
				"title":       "Quick plan review",
				"description": "Review completion and blocked counts in your active horizon.",
				"cli_example": "pomo plan status --from 2026-03-01T00:00 --to 2026-03-08T00:00",
			},
			{
				"title":       "Set or adjust workload targets",
				"description": "Keep weekly/monthly goals aligned with your current priorities.",
				"cli_example": "pomo plan target add --domain Math --subtopic Discrete --cadence weekly --hours 8",
			},
			{
				"title":       "Verify scheduler constraints",
				"description": "Ensure weekdays, windows, meal blocks, and day-cap are correct.",
				"cli_example": "pomo plan constraint show",
			},
			{
				"title":       "Preview and apply schedule generation",
				"description": "Run dry-run first, then apply scheduler output.",
				"cli_example": "pomo plan generate --from 2026-03-01T00:00 --to 2026-03-08T00:00 --dry-run",
			},
			{
				"title":       "Execute sessions",
				"description": "Start focus and break sessions during the day.",
				"cli_example": "pomo start 50m Math::DiscreteProbability",
			},
			{
				"title":       "Inspect end-of-day metrics",
				"description": "Review stats and plan execution quality.",
				"cli_example": "pomo stat day",
			},
		},
	})
}
