package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/db"
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
	case "start_time", "duration", "topic", "type":
		sortBy = f.SortBy
	}
	order := "DESC"
	if strings.EqualFold(f.Order, "asc") {
		order = "ASC"
	}

	where := []string{"1=1"}
	args := []any{}
	if f.Query != "" {
		where = append(where, "(topic LIKE ?)")
		args = append(args, "%"+f.Query+"%")
	}
	if f.Type == "focus" || f.Type == "break" {
		where = append(where, "type = ?")
		args = append(args, f.Type)
	}

	countQ := "SELECT COUNT(1) FROM sessions WHERE " + strings.Join(where, " AND ")
	var total int
	if err := db.DB.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return SessionListResult{}, err
	}

	offset := (f.Page - 1) * f.PageSize
	q := fmt.Sprintf(`
		SELECT id, type, topic, start_time, end_time, duration, planned_event_id
		FROM sessions
		WHERE %s
		ORDER BY %s %s
		LIMIT ? OFFSET ?`, strings.Join(where, " AND "), sortBy, order)
	args = append(args, f.PageSize, offset)

	rows, err := db.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return SessionListResult{}, err
	}
	defer rows.Close()

	result := SessionListResult{Page: f.Page, PageSize: f.PageSize, TotalRows: total}
	for rows.Next() {
		var s Session
		var end sql.NullTime
		var dur sql.NullInt64
		var planned sql.NullInt64
		if err := rows.Scan(&s.ID, &s.Type, &s.Topic, &s.StartTime, &end, &dur, &planned); err != nil {
			return SessionListResult{}, err
		}
		if end.Valid {
			e := end.Time
			s.EndTime = &e
		}
		if dur.Valid {
			s.DurationSec = int(dur.Int64)
		}
		if planned.Valid {
			v := int(planned.Int64)
			s.PlannedEventID = &v
		}
		result.Rows = append(result.Rows, s)
	}
	if err := rows.Err(); err != nil {
		return SessionListResult{}, err
	}

	return result, nil
}

func GetSessionByID(ctx context.Context, id int) (Session, error) {
	var s Session
	var end sql.NullTime
	var dur sql.NullInt64
	var planned sql.NullInt64
	err := db.DB.QueryRowContext(ctx, `
		SELECT id, type, topic, start_time, end_time, duration, planned_event_id
		FROM sessions WHERE id = ?`, id).
		Scan(&s.ID, &s.Type, &s.Topic, &s.StartTime, &end, &dur, &planned)
	if err != nil {
		return Session{}, err
	}
	if end.Valid {
		e := end.Time
		s.EndTime = &e
	}
	if dur.Valid {
		s.DurationSec = int(dur.Int64)
	}
	if planned.Valid {
		v := int(planned.Int64)
		s.PlannedEventID = &v
	}
	return s, nil
}

func CreateSession(ctx context.Context, s Session, origin string) (int64, error) {
	if s.DurationSec <= 0 {
		if s.EndTime != nil {
			s.DurationSec = int(s.EndTime.Sub(s.StartTime).Seconds())
		}
	}

	res, err := db.DB.ExecContext(ctx, `
		INSERT INTO sessions(type, topic, start_time, end_time, duration, planned_event_id, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		s.Type, s.Topic, s.StartTime, s.EndTime, s.DurationSec, s.PlannedEventID, time.Now(), time.Now())
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	s.ID = int(id)
	_ = InsertAuditLog(ctx, "session", int(id), "create", nil, s, origin)
	return id, nil
}

func UpdateSession(ctx context.Context, id int, s Session, origin string) error {
	before, err := GetSessionByID(ctx, id)
	if err != nil {
		return err
	}

	if s.EndTime != nil {
		s.DurationSec = int(s.EndTime.Sub(s.StartTime).Seconds())
	}

	_, err = db.DB.ExecContext(ctx, `
		UPDATE sessions
		SET type = ?, topic = ?, start_time = ?, end_time = ?, duration = ?, updated_at = ?
		WHERE id = ?`,
		s.Type, s.Topic, s.StartTime, s.EndTime, s.DurationSec, time.Now(), id)
	if err != nil {
		return err
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
	if _, err := db.DB.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id); err != nil {
		return err
	}
	_ = InsertAuditLog(ctx, "session", id, "delete", before, nil, origin)
	return nil
}

func PlannedEventsInRange(ctx context.Context, from, to time.Time) ([]PlannedEvent, error) {
	rows, err := db.DB.QueryContext(ctx, `
		SELECT id, title, description, start_time, end_time, status, source
		FROM planned_events
		WHERE start_time < ? AND end_time > ?
		ORDER BY start_time ASC`, to, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PlannedEvent
	for rows.Next() {
		var e PlannedEvent
		if err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.StartTime, &e.EndTime, &e.Status, &e.Source); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func CreatePlannedEvent(ctx context.Context, e PlannedEvent, origin string) (int64, error) {
	if e.Status == "" {
		e.Status = "planned"
	}
	if e.Source == "" {
		e.Source = "manual"
	}

	res, err := db.DB.ExecContext(ctx, `
		INSERT INTO planned_events(title, description, start_time, end_time, status, source, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Title, e.Description, e.StartTime, e.EndTime, e.Status, e.Source, time.Now(), time.Now())
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
		SELECT id, title, description, start_time, end_time, status, source
		FROM planned_events
		WHERE id = ?`, id).
		Scan(&e.ID, &e.Title, &e.Description, &e.StartTime, &e.EndTime, &e.Status, &e.Source)
	if err != nil {
		return PlannedEvent{}, err
	}
	return e, nil
}

func UpdatePlannedEvent(ctx context.Context, id int, e PlannedEvent, origin string) error {
	before, err := GetPlannedEventByID(ctx, id)
	if err != nil {
		return err
	}
	_, err = db.DB.ExecContext(ctx, `
		UPDATE planned_events
		SET title = ?, description = ?, start_time = ?, end_time = ?, status = ?, source = ?, updated_at = ?
		WHERE id = ?`,
		e.Title, e.Description, e.StartTime, e.EndTime, e.Status, e.Source, time.Now(), id)
	if err != nil {
		return err
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
	if _, err := db.DB.ExecContext(ctx, `DELETE FROM planned_events WHERE id = ?`, id); err != nil {
		return err
	}
	_ = InsertAuditLog(ctx, "planned_event", id, "delete", before, nil, origin)
	return nil
}

func SessionsInRange(ctx context.Context, from, to time.Time) ([]Session, error) {
	rows, err := db.DB.QueryContext(ctx, `
		SELECT id, type, topic, start_time, end_time, duration, planned_event_id
		FROM sessions
		WHERE start_time < ? AND COALESCE(end_time, datetime(start_time, '+' || duration || ' seconds')) > ?
		ORDER BY start_time ASC`, to, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Session
	for rows.Next() {
		var s Session
		var end sql.NullTime
		var dur sql.NullInt64
		var planned sql.NullInt64
		if err := rows.Scan(&s.ID, &s.Type, &s.Topic, &s.StartTime, &end, &dur, &planned); err != nil {
			return nil, err
		}
		if end.Valid {
			e := end.Time
			s.EndTime = &e
		}
		if dur.Valid {
			s.DurationSec = int(dur.Int64)
		}
		if planned.Valid {
			v := int(planned.Int64)
			s.PlannedEventID = &v
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
