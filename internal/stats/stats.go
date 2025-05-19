package stats

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
)

type FocusStat struct {
	Topic        string
	Count        int
	TotalMinutes int
}

type BreakStat struct {
	Count        int
	TotalMinutes int
}

type Session struct {
	Type     string
	Duration int
	Start    time.Time
}

func QueryStats(start, end time.Time) ([]FocusStat, BreakStat, error) {
	rows, err := db.DB.Query(`
        SELECT topic, COUNT(*), SUM(duration)
        FROM sessions
        WHERE type = 'focus' AND start_time BETWEEN ? AND ?
        GROUP BY topic
        ORDER BY SUM(duration) DESC
        LIMIT 10
    `, start, end)
	if err != nil {
		return nil, BreakStat{}, err
	}
	defer rows.Close()

	var focusStats []FocusStat
	for rows.Next() {
		var topic string
		var count int
		var durSec int
		rows.Scan(&topic, &count, &durSec)
		focusStats = append(focusStats, FocusStat{
			Topic:        topic,
			Count:        count,
			TotalMinutes: durSec / 60,
		})
	}

	var breakStat BreakStat
	row := db.DB.QueryRow(`
        SELECT COUNT(*), SUM(duration)
        FROM sessions
        WHERE type = 'break' AND start_time BETWEEN ? AND ?
    `, start, end)
	var durSec sql.NullInt64
	row.Scan(&breakStat.Count, &durSec)
	if durSec.Valid {
		breakStat.TotalMinutes = int(durSec.Int64) / 60
	}

	return focusStats, breakStat, nil
}

func QuerySessionBlocks(start, end time.Time) ([]Session, error) {
	rows, err := db.DB.Query(`
        SELECT type, duration, start_time
        FROM sessions
        WHERE start_time BETWEEN ? AND ?
        ORDER BY start_time ASC
    `, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		var startTime time.Time
		rows.Scan(&s.Type, &s.Duration, &startTime)
		s.Start = startTime
		sessions = append(sessions, s)
	}

	var blocks []Session
	if len(sessions) == 0 {
		return blocks, nil
	}

	curr := sessions[0]
	for i := 1; i < len(sessions); i++ {
		next := sessions[i]
		if next.Type == curr.Type {
			curr.Duration += next.Duration
		} else {
			blocks = append(blocks, curr)
			curr = next
		}
	}
	blocks = append(blocks, curr)
	return blocks, nil
}

func GetTimeRange(view string) (time.Time, time.Time) {
	now := time.Now()
	var start time.Time

	switch view {
	case "day":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		offset := int(now.Weekday())
		if offset == 0 {
			offset = 7
		}
		start = now.AddDate(0, 0, -offset+1)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, now.Location())
	case "month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	case "year":
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	case "all":
		start = time.Date(2000, 1, 1, 0, 0, 0, 0, now.Location())
	case "sem":
		t, err := time.Parse("2006-01-02", config.AppConfig.SemesterStart)
		if err != nil || t.IsZero() {
			t = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		}
		start = t
	default:
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}

	return start, now
}

func FormatRangeName(view string) string {
	switch view {
	case "day":
		return "Today"
	case "week":
		return "This Week"
	case "month":
		return "This Month"
	case "year":
		return "This Year"
	case "all":
		return "All Time"
	default:
		return view
	}
}

func FormatMinutesToHM(mins int) string {
	h := mins / 60
	m := mins % 60
	return fmt.Sprintf("%02d:%02d", h, m)
}
