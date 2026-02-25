package dashboard

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/Soeky/pomo/internal/db"
)

type Module interface {
	ID() string
	Title() string
	Load(ctx context.Context, start, end time.Time) (any, error)
}

type Definition struct {
	ID    string
	Title string
	Data  any
}

var registry = map[string]Module{}

func Register(m Module) {
	registry[m.ID()] = m
}

func All(ctx context.Context, start, end time.Time) ([]Definition, error) {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make([]Definition, 0, len(ids))
	for _, id := range ids {
		m := registry[id]
		data, err := m.Load(ctx, start, end)
		if err != nil {
			return nil, fmt.Errorf("module %s: %w", id, err)
		}
		out = append(out, Definition{ID: m.ID(), Title: m.Title(), Data: data})
	}
	return out, nil
}

func ByID(id string) (Module, bool) {
	m, ok := registry[id]
	return m, ok
}

type totalsModule struct{}

type TotalsData struct {
	FocusMinutes int
	BreakMinutes int
	Sessions     int
}

func (totalsModule) ID() string    { return "totals" }
func (totalsModule) Title() string { return "Totals" }
func (totalsModule) Load(ctx context.Context, start, end time.Time) (any, error) {
	var focusSec sql.NullInt64
	var breakSec sql.NullInt64
	var count int

	if err := db.DB.QueryRowContext(ctx, `
		SELECT COUNT(*),
		COALESCE(SUM(CASE WHEN type='focus' THEN duration END), 0),
		COALESCE(SUM(CASE WHEN type='break' THEN duration END), 0)
		FROM sessions
		WHERE start_time BETWEEN ? AND ?`, start, end).Scan(&count, &focusSec, &breakSec); err != nil {
		return nil, err
	}

	return TotalsData{
		FocusMinutes: int(focusSec.Int64 / 60),
		BreakMinutes: int(breakSec.Int64 / 60),
		Sessions:     count,
	}, nil
}

type topTopicsModule struct{}

type TopicRow struct {
	Topic   string
	Minutes int
	Count   int
}

func (topTopicsModule) ID() string    { return "top_topics" }
func (topTopicsModule) Title() string { return "Top Topics" }
func (topTopicsModule) Load(ctx context.Context, start, end time.Time) (any, error) {
	rows, err := db.DB.QueryContext(ctx, `
		SELECT COALESCE(topic, 'General') as topic, COUNT(*), COALESCE(SUM(duration), 0)
		FROM sessions
		WHERE type = 'focus' AND start_time BETWEEN ? AND ?
		GROUP BY topic
		ORDER BY SUM(duration) DESC
		LIMIT 5`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TopicRow
	for rows.Next() {
		var r TopicRow
		var sec int
		if err := rows.Scan(&r.Topic, &r.Count, &sec); err != nil {
			return nil, err
		}
		r.Minutes = sec / 60
		out = append(out, r)
	}
	return out, nil
}

type completionModule struct{}

type CompletionData struct {
	PlannedCount int
	DoneCount    int
}

func (completionModule) ID() string    { return "planned_completion" }
func (completionModule) Title() string { return "Planned Completion" }
func (completionModule) Load(ctx context.Context, start, end time.Time) (any, error) {
	var planned int
	if err := db.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM planned_events WHERE start_time BETWEEN ? AND ?`, start, end).Scan(&planned); err != nil {
		return nil, err
	}
	var done int
	if err := db.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM planned_events WHERE status = 'done' AND start_time BETWEEN ? AND ?`, start, end).Scan(&done); err != nil {
		return nil, err
	}
	return CompletionData{PlannedCount: planned, DoneCount: done}, nil
}

func init() {
	Register(totalsModule{})
	Register(topTopicsModule{})
	Register(completionModule{})
}
