package stats

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/topics"
)

const DefaultBreakCreditThresholdMinutes = 10

type EffectiveSession struct {
	Type            string
	Topic           string
	DurationSeconds int
}

type EffectiveFocusTotals struct {
	RawFocusMinutes       int
	CreditedBreakMinutes  int
	EffectiveFocusMinutes int
}

func QueryEffectiveFocusTotals(start, end time.Time, thresholdMinutes int) (EffectiveFocusTotals, error) {
	if db.DB == nil {
		return EffectiveFocusTotals{}, fmt.Errorf("database is not initialized")
	}

	sessions, err := queryEffectiveSessions(db.DB, start, end)
	if err != nil {
		return EffectiveFocusTotals{}, err
	}
	return ComputeEffectiveFocusTotals(sessions, thresholdMinutes), nil
}

func ComputeEffectiveFocusTotals(sessions []EffectiveSession, thresholdMinutes int) EffectiveFocusTotals {
	if thresholdMinutes <= 0 {
		thresholdMinutes = DefaultBreakCreditThresholdMinutes
	}
	thresholdSeconds := thresholdMinutes * 60

	rawFocusSeconds := 0
	creditedBreakSeconds := 0

	for _, s := range sessions {
		if s.Type == "focus" && s.DurationSeconds > 0 {
			rawFocusSeconds += s.DurationSeconds
		}
	}

	for i := range sessions {
		s := sessions[i]
		if s.Type != "break" || s.DurationSeconds <= 0 || s.DurationSeconds > thresholdSeconds {
			continue
		}
		if i == 0 || i == len(sessions)-1 {
			continue
		}

		prev := sessions[i-1]
		next := sessions[i+1]
		if prev.Type != "focus" || next.Type != "focus" {
			continue
		}
		if focusSessionDomain(prev.Topic) != focusSessionDomain(next.Topic) {
			continue
		}

		creditedBreakSeconds += s.DurationSeconds
	}

	rawFocusMinutes := rawFocusSeconds / 60
	creditedBreakMinutes := creditedBreakSeconds / 60

	return EffectiveFocusTotals{
		RawFocusMinutes:       rawFocusMinutes,
		CreditedBreakMinutes:  creditedBreakMinutes,
		EffectiveFocusMinutes: rawFocusMinutes + creditedBreakMinutes,
	}
}

func queryEffectiveSessions(q interface {
	Query(query string, args ...any) (*sql.Rows, error)
}, start, end time.Time) ([]EffectiveSession, error) {
	rows, err := q.Query(`
		SELECT type, COALESCE(topic, ''), COALESCE(duration, 0)
		FROM sessions
		WHERE start_time BETWEEN ? AND ?
		ORDER BY start_time ASC, id ASC
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]EffectiveSession, 0)
	for rows.Next() {
		var s EffectiveSession
		if err := rows.Scan(&s.Type, &s.Topic, &s.DurationSeconds); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func focusSessionDomain(topic string) string {
	trimmed := strings.TrimSpace(topic)
	if trimmed == "" {
		return topics.DefaultDomain
	}

	path, err := topics.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	return path.Domain
}
