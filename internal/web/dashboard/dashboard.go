package dashboard

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

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

type Registry struct {
	modules map[string]Module
}

func NewRegistry(q Querier) *Registry {
	r := &Registry{modules: map[string]Module{}}
	r.Register(totalsModule{q: q})
	r.Register(topTopicsModule{q: q})
	r.Register(completionModule{q: q})
	return r
}

func (r *Registry) Register(m Module) {
	r.modules[m.ID()] = m
}

func (r *Registry) All(ctx context.Context, start, end time.Time) ([]Definition, error) {
	ids := make([]string, 0, len(r.modules))
	for id := range r.modules {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make([]Definition, 0, len(ids))
	for _, id := range ids {
		m := r.modules[id]
		data, err := m.Load(ctx, start, end)
		if err != nil {
			return nil, fmt.Errorf("module %s: %w", id, err)
		}
		out = append(out, Definition{ID: m.ID(), Title: m.Title(), Data: data})
	}
	return out, nil
}

func (r *Registry) ByID(id string) (Module, bool) {
	m, ok := r.modules[id]
	return m, ok
}

type totalsModule struct {
	q Querier
}

type TotalsData struct {
	FocusMinutes int
	BreakMinutes int
	Sessions     int
}

func (totalsModule) ID() string    { return "totals" }
func (totalsModule) Title() string { return "Totals" }
func (m totalsModule) Load(ctx context.Context, start, end time.Time) (any, error) {
	if m.q == nil {
		return nil, fmt.Errorf("dashboard database is not initialized")
	}

	var focusSec sql.NullInt64
	var breakSec sql.NullInt64
	var count int

	if err := m.q.QueryRowContext(ctx, `
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

type topTopicsModule struct {
	q Querier
}

type TopicRow struct {
	Topic   string
	Minutes int
	Count   int
}

func (topTopicsModule) ID() string    { return "top_topics" }
func (topTopicsModule) Title() string { return "Top Topics" }
func (m topTopicsModule) Load(ctx context.Context, start, end time.Time) (any, error) {
	if m.q == nil {
		return nil, fmt.Errorf("dashboard database is not initialized")
	}

	rows, err := m.q.QueryContext(ctx, `
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

type completionModule struct {
	q Querier
}

type CompletionData struct {
	PlannedCount int
	DoneCount    int
}

func (completionModule) ID() string    { return "planned_completion" }
func (completionModule) Title() string { return "Planned Completion" }
func (m completionModule) Load(ctx context.Context, start, end time.Time) (any, error) {
	if m.q == nil {
		return nil, fmt.Errorf("dashboard database is not initialized")
	}

	var planned int
	if err := m.q.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM planned_events WHERE start_time BETWEEN ? AND ?`, start, end).Scan(&planned); err != nil {
		return nil, err
	}
	var done int
	if err := m.q.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM planned_events WHERE status = 'done' AND start_time BETWEEN ? AND ?`, start, end).Scan(&done); err != nil {
		return nil, err
	}
	return CompletionData{PlannedCount: planned, DoneCount: done}, nil
}
