package stats

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/topics"
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

type TopicGroupStat struct {
	Name         string
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
		if err := rows.Scan(&topic, &count, &durSec); err != nil {
			return nil, BreakStat{}, err
		}
		focusStats = append(focusStats, FocusStat{
			Topic:        topic,
			Count:        count,
			TotalMinutes: durSec / 60,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, BreakStat{}, err
	}

	var breakStat BreakStat
	row := db.DB.QueryRow(`
        SELECT COUNT(*), SUM(duration)
        FROM sessions
        WHERE type = 'break' AND start_time BETWEEN ? AND ?
    `, start, end)
	var durSec sql.NullInt64
	if err := row.Scan(&breakStat.Count, &durSec); err != nil {
		return nil, BreakStat{}, err
	}
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
		if err := rows.Scan(&s.Type, &s.Duration, &startTime); err != nil {
			return nil, err
		}
		s.Start = startTime
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
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

func QueryTopicHierarchyStats(start, end time.Time) ([]TopicGroupStat, []TopicGroupStat, error) {
	rows, err := db.DB.Query(`
		SELECT COALESCE(topic, ''), COUNT(*), COALESCE(SUM(duration), 0)
		FROM sessions
		WHERE type = 'focus' AND start_time BETWEEN ? AND ?
		GROUP BY topic
	`, start, end)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	domainAgg := map[string]*TopicGroupStat{}
	subtopicAgg := map[string]*TopicGroupStat{}

	for rows.Next() {
		var rawTopic string
		var count int
		var durationSec int
		if err := rows.Scan(&rawTopic, &count, &durationSec); err != nil {
			return nil, nil, err
		}
		minutes := durationSec / 60

		path, err := topics.Parse(rawTopic)
		if err != nil {
			trimmed := strings.TrimSpace(rawTopic)
			if trimmed == "" {
				trimmed = topics.DefaultDomain
			}
			path = topics.Path{Domain: trimmed, Subtopic: topics.DefaultSubtopic}
		}

		if _, ok := domainAgg[path.Domain]; !ok {
			domainAgg[path.Domain] = &TopicGroupStat{Name: path.Domain}
		}
		domainAgg[path.Domain].Count += count
		domainAgg[path.Domain].TotalMinutes += minutes

		if _, ok := subtopicAgg[path.Subtopic]; !ok {
			subtopicAgg[path.Subtopic] = &TopicGroupStat{Name: path.Subtopic}
		}
		subtopicAgg[path.Subtopic].Count += count
		subtopicAgg[path.Subtopic].TotalMinutes += minutes
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	domains := flattenTopicGroups(domainAgg)
	subtopics := flattenTopicGroups(subtopicAgg)
	return domains, subtopics, nil
}

func flattenTopicGroups(in map[string]*TopicGroupStat) []TopicGroupStat {
	out := make([]TopicGroupStat, 0, len(in))
	for _, s := range in {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalMinutes != out[j].TotalMinutes {
			return out[i].TotalMinutes > out[j].TotalMinutes
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func GetTimeRange(view string) (time.Time, time.Time) {
	now := time.Now()
	return getTimeRangeAt(view, now)
}

func getTimeRangeAt(view string, now time.Time) (time.Time, time.Time) {
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
	now := time.Now()
	return formatRangeNameAt(view, now)
}

func formatRangeNameAt(view string, now time.Time) string {
	switch view {
	case "day":
		return now.Format("2006-01-02")
	case "week":
		weekday := int(now.Weekday())
		var daysToMonday int
		if weekday == 0 {
			daysToMonday = 6
		} else {
			daysToMonday = weekday - 1
		}

		monday := now.AddDate(0, 0, -daysToMonday)
		sunday := monday.AddDate(0, 0, 6)

		return fmt.Sprintf("%s – %s", monday.Format("2006-01-02"), sunday.Format("2006-01-02"))

	case "month":
		return now.Format("2006-01")

	case "year":
		return now.Format("2006")

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
