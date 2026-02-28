package events

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/topics"
)

type Event struct {
	ID               int64
	Kind             string
	Title            string
	Domain           string
	Subtopic         string
	Description      string
	StartTime        time.Time
	EndTime          time.Time
	DurationSec      int
	Layer            string
	Status           string
	Source           string
	RecurrenceRuleID *int64
	BlockedReason    string
	BlockedOverride  bool
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
	override := 0
	if e.BlockedOverride {
		override = 1
	}
	res, err := db.DB.ExecContext(ctx, `
		INSERT INTO events(kind, title, domain, subtopic, description, start_time, end_time, duration, layer, status, source, blocked_reason, blocked_override, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Kind, e.Title, e.Domain, e.Subtopic, e.Description, e.StartTime, e.EndTime, e.DurationSec, e.Layer, e.Status, e.Source, nullableText(e.BlockedReason), override, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func GetByID(ctx context.Context, id int64) (Event, error) {
	row := db.DB.QueryRowContext(ctx, `
		SELECT id, kind, title, domain, subtopic, COALESCE(description, ''), start_time, end_time, duration, layer, status, source, recurrence_rule_id, COALESCE(blocked_reason, ''), COALESCE(blocked_override, 0)
		FROM events
		WHERE id = ?`, id)
	var e Event
	var ruleID sql.NullInt64
	var blockedOverride int
	if err := row.Scan(&e.ID, &e.Kind, &e.Title, &e.Domain, &e.Subtopic, &e.Description, &e.StartTime, &e.EndTime, &e.DurationSec, &e.Layer, &e.Status, &e.Source, &ruleID, &e.BlockedReason, &blockedOverride); err != nil {
		return Event{}, err
	}
	if ruleID.Valid {
		v := ruleID.Int64
		e.RecurrenceRuleID = &v
	}
	e.BlockedOverride = blockedOverride == 1
	return e, nil
}

func Update(ctx context.Context, id int64, e Event) error {
	if err := normalizeAndValidate(&e); err != nil {
		return err
	}
	override := 0
	if e.BlockedOverride {
		override = 1
	}
	_, err := db.DB.ExecContext(ctx, `
		UPDATE events
		SET kind = ?, title = ?, domain = ?, subtopic = ?, description = ?,
		    start_time = ?, end_time = ?, duration = ?, layer = ?, status = ?, source = ?, blocked_reason = ?, blocked_override = ?, updated_at = ?
		WHERE id = ?`,
		e.Kind, e.Title, e.Domain, e.Subtopic, e.Description,
		e.StartTime, e.EndTime, e.DurationSec, e.Layer, e.Status, e.Source, nullableText(e.BlockedReason), override, time.Now(), id)
	if err != nil {
		return err
	}
	_, err = ReconcileBlockedStatusesForEventAndDependents(ctx, id)
	return err
}

func Delete(ctx context.Context, id int64) error {
	if _, err := db.DB.ExecContext(ctx, `DELETE FROM event_dependencies WHERE event_id = ? OR depends_on_event_id = ?`, id, id); err != nil {
		return err
	}
	_, err := db.DB.ExecContext(ctx, `DELETE FROM events WHERE id = ?`, id)
	return err
}

func ListInRange(ctx context.Context, from, to time.Time) ([]Event, error) {
	return listWithWhere(ctx, from, to, "")
}

func ListCanonicalInRange(ctx context.Context, from, to time.Time) ([]Event, error) {
	return listWithWhere(ctx, from, to, "AND legacy_source IS NULL")
}

func listWithWhere(ctx context.Context, from, to time.Time, extraWhere string) ([]Event, error) {
	rows, err := db.DB.QueryContext(ctx, `
		SELECT id, kind, title, domain, subtopic, COALESCE(description, ''), start_time, end_time, duration, layer, status, source, recurrence_rule_id, COALESCE(blocked_reason, ''), COALESCE(blocked_override, 0)
		FROM events
		WHERE start_time < ? AND end_time > ?
		`+extraWhere+`
		ORDER BY start_time ASC`, to, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var e Event
		var ruleID sql.NullInt64
		var blockedOverride int
		if err := rows.Scan(&e.ID, &e.Kind, &e.Title, &e.Domain, &e.Subtopic, &e.Description, &e.StartTime, &e.EndTime, &e.DurationSec, &e.Layer, &e.Status, &e.Source, &ruleID, &e.BlockedReason, &blockedOverride); err != nil {
			return nil, err
		}
		if ruleID.Valid {
			v := ruleID.Int64
			e.RecurrenceRuleID = &v
		}
		e.BlockedOverride = blockedOverride == 1
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

func nullableText(v string) any {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return nil
	}
	return trimmed
}
