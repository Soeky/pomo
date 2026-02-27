package events

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/topics"
)

type Event struct {
	ID          int64
	Kind        string
	Title       string
	Domain      string
	Subtopic    string
	Description string
	StartTime   time.Time
	EndTime     time.Time
	DurationSec int
	Layer       string
	Status      string
	Source      string
}

var allowedKinds = map[string]struct{}{
	"focus":    {},
	"break":    {},
	"task":     {},
	"class":    {},
	"exercise": {},
	"meal":     {},
	"other":    {},
}

var allowedLayers = map[string]struct{}{
	"planned": {},
	"done":    {},
}

var allowedStatuses = map[string]struct{}{
	"planned":     {},
	"in_progress": {},
	"done":        {},
	"canceled":    {},
	"blocked":     {},
}

var allowedSources = map[string]struct{}{
	"manual":    {},
	"tracked":   {},
	"recurring": {},
	"scheduler": {},
}

func Create(ctx context.Context, e Event) (int64, error) {
	if err := normalizeAndValidate(&e); err != nil {
		return 0, err
	}

	now := time.Now()
	res, err := db.DB.ExecContext(ctx, `
		INSERT INTO events(kind, title, domain, subtopic, description, start_time, end_time, duration, layer, status, source, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Kind, e.Title, e.Domain, e.Subtopic, e.Description, e.StartTime, e.EndTime, e.DurationSec, e.Layer, e.Status, e.Source, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func ListInRange(ctx context.Context, from, to time.Time) ([]Event, error) {
	rows, err := db.DB.QueryContext(ctx, `
		SELECT id, kind, title, domain, subtopic, COALESCE(description, ''), start_time, end_time, duration, layer, status, source
		FROM events
		WHERE start_time < ? AND end_time > ?
		ORDER BY start_time ASC`, to, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Kind, &e.Title, &e.Domain, &e.Subtopic, &e.Description, &e.StartTime, &e.EndTime, &e.DurationSec, &e.Layer, &e.Status, &e.Source); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func normalizeAndValidate(e *Event) error {
	e.Kind = strings.ToLower(strings.TrimSpace(e.Kind))
	if e.Kind == "" {
		e.Kind = "task"
	}
	if _, ok := allowedKinds[e.Kind]; !ok {
		return fmt.Errorf("invalid kind: %s", e.Kind)
	}

	e.Layer = strings.ToLower(strings.TrimSpace(e.Layer))
	if e.Layer == "" {
		e.Layer = "planned"
	}
	if _, ok := allowedLayers[e.Layer]; !ok {
		return fmt.Errorf("invalid layer: %s", e.Layer)
	}

	e.Status = strings.ToLower(strings.TrimSpace(e.Status))
	if e.Status == "" {
		if e.Layer == "done" {
			e.Status = "done"
		} else {
			e.Status = "planned"
		}
	}
	if _, ok := allowedStatuses[e.Status]; !ok {
		return fmt.Errorf("invalid status: %s", e.Status)
	}

	e.Source = strings.ToLower(strings.TrimSpace(e.Source))
	if e.Source == "" {
		e.Source = "manual"
	}
	if _, ok := allowedSources[e.Source]; !ok {
		return fmt.Errorf("invalid source: %s", e.Source)
	}

	e.Title = strings.TrimSpace(e.Title)
	if e.Title == "" {
		return fmt.Errorf("title is required")
	}

	path, err := topics.ParseParts(e.Domain, e.Subtopic)
	if err != nil {
		return err
	}
	e.Domain = path.Domain
	e.Subtopic = path.Subtopic

	if e.EndTime.Before(e.StartTime) || e.EndTime.Equal(e.StartTime) {
		return fmt.Errorf("end time must be after start time")
	}
	if e.DurationSec <= 0 {
		e.DurationSec = int(e.EndTime.Sub(e.StartTime).Seconds())
	}

	return nil
}
