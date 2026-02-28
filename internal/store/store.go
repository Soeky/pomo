package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/topics"
)

type Session struct {
	ID             int        `json:"id"`
	Type           string     `json:"type"`
	Topic          string     `json:"topic"`
	StartTime      time.Time  `json:"start_time"`
	EndTime        *time.Time `json:"end_time,omitempty"`
	DurationSec    int        `json:"duration_sec"`
	PlannedEventID *int       `json:"planned_event_id,omitempty"`
}

type PlannedEvent struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Domain      string    `json:"domain"`
	Subtopic    string    `json:"subtopic"`
	Description string    `json:"description"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Status      string    `json:"status"`
	Source      string    `json:"source"`
}

type SessionFilter struct {
	Query    string
	Type     string
	SortBy   string
	Order    string
	Page     int
	PageSize int
}

type SessionListResult struct {
	Rows      []Session
	Page      int
	PageSize  int
	TotalRows int
}

func ListSessions(ctx context.Context, f SessionFilter) (SessionListResult, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 200 {
		f.PageSize = 20
	}

	sortBy := "start_time"
	switch f.SortBy {
	case "start_time":
		sortBy = "start_time"
	case "duration":
		sortBy = "duration"
	case "topic":
		sortBy = "domain, subtopic"
	case "type":
		sortBy = "kind"
	}

	order := "DESC"
	if strings.EqualFold(f.Order, "asc") {
		order = "ASC"
	}

	where := []string{trackedSessionWhereClause}
	args := make([]any, 0)

	if query := strings.TrimSpace(f.Query); query != "" {
		where = append(where, `(COALESCE(NULLIF(TRIM(title), ''), '') LIKE ? OR COALESCE(NULLIF(TRIM(domain), ''), '') LIKE ? OR COALESCE(NULLIF(TRIM(subtopic), ''), '') LIKE ?)`)
		like := "%" + query + "%"
		args = append(args, like, like, like)
	}

	if typ := strings.ToLower(strings.TrimSpace(f.Type)); typ == "focus" || typ == "break" {
		where = append(where, `kind = ?`)
		args = append(args, typ)
	}

	countQuery := `SELECT COUNT(1) FROM events WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := db.DB.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return SessionListResult{}, err
	}

	offset := (f.Page - 1) * f.PageSize
	query := fmt.Sprintf(`
		SELECT id, kind, COALESCE(title, ''), COALESCE(domain, ''), COALESCE(subtopic, ''), start_time, end_time, COALESCE(duration, 0)
		FROM events
		WHERE %s
		ORDER BY %s %s
		LIMIT ? OFFSET ?`, strings.Join(where, " AND "), sortBy, order)
	args = append(args, f.PageSize, offset)

	rows, err := db.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return SessionListResult{}, err
	}
	defer rows.Close()

	result := SessionListResult{Page: f.Page, PageSize: f.PageSize, TotalRows: total}
	for rows.Next() {
		s, err := scanSessionFromEventRow(rows)
		if err != nil {
			return SessionListResult{}, err
		}
		result.Rows = append(result.Rows, s)
	}
	if err := rows.Err(); err != nil {
		return SessionListResult{}, err
	}

	return result, nil
}

func GetSessionByID(ctx context.Context, id int) (Session, error) {
	row := db.DB.QueryRowContext(ctx, `
		SELECT id, kind, COALESCE(title, ''), COALESCE(domain, ''), COALESCE(subtopic, ''), start_time, end_time, COALESCE(duration, 0)
		FROM events
		WHERE id = ?
		  AND `+trackedSessionWhereClause, id)

	var (
		s                             Session
		kind, title, domain, subtopic string
		end                           time.Time
		duration                      int
	)
	err := row.Scan(&s.ID, &kind, &title, &domain, &subtopic, &s.StartTime, &end, &duration)
	if err != nil {
		return Session{}, err
	}
	s.Type = sessionTypeFromKind(kind)
	s.Topic = topicForTrackedEvent(kind, title, domain, subtopic)
	s.EndTime = &end
	s.DurationSec = duration
	return s, nil
}

func CreateSession(ctx context.Context, s Session, origin string) (int64, error) {
	kind := sessionKind(s.Type)
	title, domain, subtopic := canonicalSessionTopic(s.Topic, kind)
	endTime, durationSec, status := normalizeSessionTiming(s.StartTime, s.EndTime, s.DurationSec)

	now := time.Now()
	res, err := db.DB.ExecContext(ctx, `
		INSERT INTO events(
			kind, title, domain, subtopic, description,
			start_time, end_time, duration,
			layer, status, source,
			created_at, updated_at
		)
		VALUES(?, ?, ?, ?, NULL, ?, ?, ?, 'done', ?, 'tracked', ?, ?)
	`, kind, title, domain, subtopic, s.StartTime, endTime, durationSec, status, now, now)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	s.ID = int(id)
	s.Type = sessionTypeFromKind(kind)
	s.Topic = topicForTrackedEvent(kind, title, domain, subtopic)
	s.DurationSec = durationSec
	s.EndTime = &endTime
	_ = InsertAuditLog(ctx, "session", int(id), "create", nil, s, origin)
	return id, nil
}

func UpdateSession(ctx context.Context, id int, s Session, origin string) error {
	before, err := GetSessionByID(ctx, id)
	if err != nil {
		return err
	}

	kind := sessionKind(s.Type)
	title, domain, subtopic := canonicalSessionTopic(s.Topic, kind)
	endTime, durationSec, status := normalizeSessionTiming(s.StartTime, s.EndTime, s.DurationSec)

	res, err := db.DB.ExecContext(ctx, `
		UPDATE events
		SET kind = ?, title = ?, domain = ?, subtopic = ?,
			start_time = ?, end_time = ?, duration = ?,
			status = ?, updated_at = ?
		WHERE id = ?
		  AND `+trackedSessionWhereClause,
		kind, title, domain, subtopic,
		s.StartTime, endTime, durationSec,
		status, time.Now(), id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return sql.ErrNoRows
	}

	after, _ := GetSessionByID(ctx, id)
	_ = InsertAuditLog(ctx, "session", id, "update", before, after, origin)
	return nil
}

func DeleteSession(ctx context.Context, id int, origin string) error {
	before, err := GetSessionByID(ctx, id)
	if err != nil {
		return err
	}

	res, err := db.DB.ExecContext(ctx, `
		DELETE FROM events
		WHERE id = ?
		  AND `+trackedSessionWhereClause, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return sql.ErrNoRows
	}

	_ = InsertAuditLog(ctx, "session", id, "delete", before, nil, origin)
	return nil
}

func PlannedEventsInRange(ctx context.Context, from, to time.Time) ([]PlannedEvent, error) {
	rows, err := db.DB.QueryContext(ctx, `
		SELECT id, COALESCE(title, ''), COALESCE(domain, ''), COALESCE(subtopic, ''), COALESCE(description, ''), start_time, end_time, COALESCE(status, 'planned'), COALESCE(source, 'manual')
		FROM events
		WHERE `+plannedEventWhereClause+`
		  AND start_time < ?
		  AND end_time > ?
		ORDER BY start_time ASC, id ASC`, to, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]PlannedEvent, 0)
	for rows.Next() {
		var e PlannedEvent
		if err := rows.Scan(&e.ID, &e.Title, &e.Domain, &e.Subtopic, &e.Description, &e.StartTime, &e.EndTime, &e.Status, &e.Source); err != nil {
			return nil, err
		}
		e.Source = normalizePlannedSource(e.Source)
		e.Status = normalizePlannedStatus(e.Status)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func CreatePlannedEvent(ctx context.Context, e PlannedEvent, origin string) (int64, error) {
	e.Status = normalizePlannedStatus(e.Status)
	e.Source = normalizePlannedSource(e.Source)
	if err := fillPlannedEventTopic(&e); err != nil {
		return 0, err
	}
	durationSec := int(e.EndTime.Sub(e.StartTime).Seconds())
	if durationSec < 0 {
		durationSec = 0
	}

	now := time.Now()
	res, err := db.DB.ExecContext(ctx, `
		INSERT INTO events(
			kind, title, domain, subtopic, description,
			start_time, end_time, duration,
			layer, status, source,
			created_at, updated_at
		)
		VALUES('task', ?, ?, ?, ?, ?, ?, ?, 'planned', ?, ?, ?, ?)
	`, e.Title, e.Domain, e.Subtopic, e.Description, e.StartTime, e.EndTime, durationSec, e.Status, e.Source, now, now)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	e.ID = int(id)
	_ = InsertAuditLog(ctx, "planned_event", int(id), "create", nil, e, origin)
	return id, nil
}

func GetPlannedEventByID(ctx context.Context, id int) (PlannedEvent, error) {
	var e PlannedEvent
	err := db.DB.QueryRowContext(ctx, `
		SELECT id, COALESCE(title, ''), COALESCE(domain, ''), COALESCE(subtopic, ''), COALESCE(description, ''), start_time, end_time, COALESCE(status, 'planned'), COALESCE(source, 'manual')
		FROM events
		WHERE id = ?
		  AND `+plannedEventWhereClause, id).
		Scan(&e.ID, &e.Title, &e.Domain, &e.Subtopic, &e.Description, &e.StartTime, &e.EndTime, &e.Status, &e.Source)
	if err != nil {
		return PlannedEvent{}, err
	}
	e.Source = normalizePlannedSource(e.Source)
	e.Status = normalizePlannedStatus(e.Status)
	return e, nil
}

func UpdatePlannedEvent(ctx context.Context, id int, e PlannedEvent, origin string) error {
	before, err := GetPlannedEventByID(ctx, id)
	if err != nil {
		return err
	}

	e.Status = normalizePlannedStatus(e.Status)
	e.Source = normalizePlannedSource(e.Source)
	if err := fillPlannedEventTopic(&e); err != nil {
		return err
	}
	durationSec := int(e.EndTime.Sub(e.StartTime).Seconds())
	if durationSec < 0 {
		durationSec = 0
	}

	res, err := db.DB.ExecContext(ctx, `
		UPDATE events
		SET kind = 'task', title = ?, domain = ?, subtopic = ?, description = ?,
			start_time = ?, end_time = ?, duration = ?,
			status = ?, source = ?, updated_at = ?
		WHERE id = ?
		  AND `+plannedEventWhereClause,
		e.Title, e.Domain, e.Subtopic, e.Description,
		e.StartTime, e.EndTime, durationSec,
		e.Status, e.Source, time.Now(), id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return sql.ErrNoRows
	}

	after, _ := GetPlannedEventByID(ctx, id)
	_ = InsertAuditLog(ctx, "planned_event", id, "update", before, after, origin)
	return nil
}

func DeletePlannedEvent(ctx context.Context, id int, origin string) error {
	before, err := GetPlannedEventByID(ctx, id)
	if err != nil {
		return err
	}

	res, err := db.DB.ExecContext(ctx, `
		DELETE FROM events
		WHERE id = ?
		  AND `+plannedEventWhereClause, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return sql.ErrNoRows
	}

	_ = InsertAuditLog(ctx, "planned_event", id, "delete", before, nil, origin)
	return nil
}

func SessionsInRange(ctx context.Context, from, to time.Time) ([]Session, error) {
	rows, err := db.DB.QueryContext(ctx, `
		SELECT id, kind, COALESCE(title, ''), COALESCE(domain, ''), COALESCE(subtopic, ''), start_time, end_time, COALESCE(duration, 0)
		FROM events
		WHERE `+trackedSessionWhereClause+`
		  AND start_time < ?
		  AND end_time > ?
		ORDER BY start_time ASC, id ASC`, to, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Session, 0)
	for rows.Next() {
		s, err := scanSessionFromEventRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func InsertAuditLog(ctx context.Context, entityType string, entityID int, action string, before any, after any, origin string) error {
	beforeJSON, err := marshalMaybeJSON(before)
	if err != nil {
		return err
	}
	afterJSON, err := marshalMaybeJSON(after)
	if err != nil {
		return err
	}
	_, err = db.DB.ExecContext(ctx, `
		INSERT INTO audit_log(entity_type, entity_id, action, before_json, after_json, changed_at, origin)
		VALUES(?, ?, ?, ?, ?, ?, ?)`,
		entityType, entityID, action, beforeJSON, afterJSON, time.Now(), origin)
	return err
}

func marshalMaybeJSON(v any) (sql.NullString, error) {
	if v == nil {
		return sql.NullString{}, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(b), Valid: true}, nil
}

func fillPlannedEventTopic(e *PlannedEvent) error {
	path, err := topics.Parse(e.Title)
	if err != nil {
		trimmed := strings.TrimSpace(e.Title)
		if trimmed == "" {
			e.Domain = topics.DefaultDomain
			e.Subtopic = topics.DefaultSubtopic
			return nil
		}
		path, err = topics.ParseParts(trimmed, topics.DefaultSubtopic)
		if err != nil {
			return err
		}
	}
	e.Domain = path.Domain
	e.Subtopic = path.Subtopic
	return nil
}

const (
	trackedSessionWhereClause = `source = 'tracked' AND layer = 'done' AND kind IN ('focus', 'break')`
	plannedEventWhereClause   = `layer = 'planned' AND source IN ('manual', 'scheduler')`
)

func scanSessionFromEventRow(scanner interface {
	Scan(dest ...any) error
}) (Session, error) {
	var (
		s                             Session
		kind, title, domain, subtopic string
		end                           time.Time
	)
	if err := scanner.Scan(&s.ID, &kind, &title, &domain, &subtopic, &s.StartTime, &end, &s.DurationSec); err != nil {
		return Session{}, err
	}
	s.Type = sessionTypeFromKind(kind)
	s.Topic = topicForTrackedEvent(kind, title, domain, subtopic)
	s.EndTime = &end
	return s, nil
}

func sessionKind(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), "break") {
		return "break"
	}
	return "focus"
}

func sessionTypeFromKind(kind string) string {
	if strings.EqualFold(strings.TrimSpace(kind), "break") {
		return "break"
	}
	return "focus"
}

func canonicalSessionTopic(rawTopic, kind string) (title, domain, subtopic string) {
	if sessionTypeFromKind(kind) == "break" {
		return "Break", "Break", topics.DefaultSubtopic
	}
	path, err := topics.Parse(rawTopic)
	if err != nil {
		path = topics.Path{Domain: topics.DefaultDomain, Subtopic: topics.DefaultSubtopic}
	}
	return path.Canonical(), path.Domain, path.Subtopic
}

func topicForTrackedEvent(kind, title, domain, subtopic string) string {
	if sessionTypeFromKind(kind) == "break" {
		return ""
	}
	if parsed, err := topics.Parse(strings.TrimSpace(title)); err == nil {
		return parsed.Canonical()
	}
	if parsed, err := topics.ParseParts(domain, subtopic); err == nil {
		return parsed.Canonical()
	}
	return topics.Path{Domain: topics.DefaultDomain, Subtopic: topics.DefaultSubtopic}.Canonical()
}

func normalizeSessionTiming(start time.Time, end *time.Time, duration int) (time.Time, int, string) {
	durationSec := duration
	status := "done"
	var endTime time.Time

	if end != nil {
		endTime = *end
		durationSec = int(endTime.Sub(start).Seconds())
	} else {
		if durationSec <= 0 {
			durationSec = 0
		}
		endTime = start.Add(time.Duration(durationSec) * time.Second)
		status = "in_progress"
	}

	if durationSec < 0 {
		durationSec = 0
	}
	return endTime, durationSec, status
}

func normalizePlannedSource(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), "scheduler") {
		return "scheduler"
	}
	return "manual"
}

func normalizePlannedStatus(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "done", "canceled", "blocked", "in_progress", "planned":
		return s
	default:
		return "planned"
	}
}
