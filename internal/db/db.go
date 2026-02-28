package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	rt "github.com/Soeky/pomo/internal/runtime"
	"github.com/Soeky/pomo/internal/topics"

	_ "modernc.org/sqlite" // SQLite driver
)

var DB *sql.DB
var ErrNoRunningSession = errors.New("no running session")

func InitDB() error {
	opened, err := Open(GetDBPath())
	if err != nil {
		return err
	}
	DB = opened
	return nil
}

func Open(path string) (*sql.DB, error) {
	opened, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := RunMigrations(context.Background(), opened); err != nil {
		_ = opened.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	return opened, nil
}

func GetDBPath() string {
	dir := rt.DataDir()
	if dir == "." {
		return "pomo.db"
	}
	return filepath.Join(dir, "pomo.db")
}

type Session struct {
	ID        int
	Type      string
	Topic     string
	StartTime time.Time
	EndTime   sql.NullTime
	Duration  sql.NullInt64
}

func InsertSession(sType, topic string, duration time.Duration) (int64, error) {
	return insertSessionAt(sType, topic, time.Now(), duration)
}

func InsertSessionAt(sType, topic string, startTime time.Time, duration time.Duration) (int64, error) {
	return insertSessionAt(sType, topic, startTime, duration)
}

func insertSessionAt(sType, topic string, startTime time.Time, duration time.Duration) (int64, error) {
	kind, title, domain, subtopic := canonicalSessionFields(sType, topic)
	durationSec := int(duration.Seconds())
	if durationSec < 0 {
		durationSec = 0
	}
	endTime := startTime.Add(time.Duration(durationSec) * time.Second)
	now := time.Now()

	res, err := DB.Exec(`
		INSERT INTO events (
			kind, title, domain, subtopic, description,
			start_time, end_time, duration,
			layer, status, source,
			created_at, updated_at
		)
		VALUES (?, ?, ?, ?, NULL, ?, ?, ?, 'done', 'in_progress', 'tracked', ?, ?)
	`, kind, title, domain, subtopic, startTime, endTime, durationSec, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func StopCurrentSession() error {
	return stopCurrentSessionAt(time.Now(), true)
}

func StopCurrentSessionAt(endTime time.Time) error {
	return stopCurrentSessionAt(endTime, false)
}

func stopCurrentSessionAt(endTime time.Time, returnErrWhenMissing bool) error {
	row := DB.QueryRow(`
		SELECT id, start_time
		FROM events
		WHERE source = 'tracked'
		  AND layer = 'done'
		  AND status = 'in_progress'
		ORDER BY start_time DESC, id DESC
		LIMIT 1`)

	var id int
	var startTime time.Time
	err := row.Scan(&id, &startTime)
	if err == sql.ErrNoRows {
		if returnErrWhenMissing {
			return ErrNoRunningSession
		}
		return nil
	}
	if err != nil {
		return err
	}

	duration := int(endTime.Sub(startTime).Seconds())
	if duration < 0 {
		duration = 0
	}

	_, err = DB.Exec(`
		UPDATE events
		SET end_time = ?, duration = ?, status = 'done', updated_at = ?
		WHERE id = ?
	`, endTime, duration, time.Now(), id)
	return err
}

func GetCurrentSession() (*Session, error) {
	row := DB.QueryRow(`
		SELECT id, kind, COALESCE(title, ''), COALESCE(domain, ''), COALESCE(subtopic, ''), start_time, COALESCE(duration, 0)
		FROM events
		WHERE source = 'tracked'
		  AND layer = 'done'
		  AND status = 'in_progress'
		ORDER BY start_time DESC, id DESC
		LIMIT 1
	`)

	var s Session
	var kind, title, domain, subtopic string
	var durationSec int64
	err := row.Scan(&s.ID, &kind, &title, &domain, &subtopic, &s.StartTime, &durationSec)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	s.Type = trackedSessionTypeFromKind(kind)
	s.Topic = trackedSessionTopic(kind, title, domain, subtopic)
	s.Duration = sql.NullInt64{Int64: durationSec, Valid: true}
	return &s, nil
}

func canonicalSessionFields(sType, topic string) (kind, title, domain, subtopic string) {
	if strings.EqualFold(strings.TrimSpace(sType), "break") {
		return "break", "Break", "Break", topics.DefaultSubtopic
	}

	path, err := topics.Parse(topic)
	if err != nil {
		path = topics.Path{Domain: topics.DefaultDomain, Subtopic: topics.DefaultSubtopic}
	}

	canonical := path.Canonical()
	if strings.TrimSpace(canonical) == "" {
		canonical = topics.Path{Domain: topics.DefaultDomain, Subtopic: topics.DefaultSubtopic}.Canonical()
	}

	return "focus", canonical, path.Domain, path.Subtopic
}

func trackedSessionTypeFromKind(kind string) string {
	if strings.EqualFold(strings.TrimSpace(kind), "break") {
		return "break"
	}
	return "focus"
}

func trackedSessionTopic(kind, title, domain, subtopic string) string {
	if trackedSessionTypeFromKind(kind) == "break" {
		return ""
	}

	candidate := strings.TrimSpace(title)
	if candidate != "" {
		if parsed, err := topics.Parse(candidate); err == nil {
			return parsed.Canonical()
		}
	}

	parsed, err := topics.ParseParts(domain, subtopic)
	if err == nil {
		return parsed.Canonical()
	}

	trimmedDomain := strings.TrimSpace(domain)
	if trimmedDomain == "" {
		trimmedDomain = topics.DefaultDomain
	}
	return topics.Path{Domain: trimmedDomain, Subtopic: topics.DefaultSubtopic}.Canonical()
}
