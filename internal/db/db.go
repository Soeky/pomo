package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	rt "github.com/Soeky/pomo/internal/runtime"

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
	res, err := DB.Exec(`
        INSERT INTO sessions (type, topic, start_time, duration)
        VALUES (?, ?, ?, ?)`, sType, topic, time.Now(), int(duration.Seconds()))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func StopCurrentSession() error {
	row := DB.QueryRow(`SELECT id, start_time FROM sessions WHERE end_time IS NULL ORDER BY start_time DESC LIMIT 1`)

	var id int
	var startTime time.Time

	err := row.Scan(&id, &startTime)
	if err == sql.ErrNoRows {
		return ErrNoRunningSession
	} else if err != nil {
		return err
	}

	endTime := time.Now()
	duration := int(endTime.Sub(startTime).Seconds())

	_, err = DB.Exec(`
        UPDATE sessions
        SET end_time = ?, duration = ?
        WHERE id = ?
    `, endTime, duration, id)

	return err
}

func StopCurrentSessionAt(endTime time.Time) error {
	row := DB.QueryRow(`SELECT id, start_time FROM sessions WHERE end_time IS NULL ORDER BY start_time DESC LIMIT 1`)

	var id int
	var startTime time.Time

	err := row.Scan(&id, &startTime)
	if err == sql.ErrNoRows {
		return nil
	} else if err != nil {
		return err
	}

	duration := int(endTime.Sub(startTime).Seconds())

	_, err = DB.Exec(`
        UPDATE sessions
        SET end_time = ?, duration = ?
        WHERE id = ?
    `, endTime, duration, id)

	return err
}

func GetCurrentSession() (*Session, error) {
	row := DB.QueryRow(`
        SELECT id, type, topic, start_time, duration
        FROM sessions
        WHERE end_time IS NULL
        ORDER BY start_time DESC
        LIMIT 1
    `)

	var s Session
	var durationSec sql.NullInt64
	err := row.Scan(&s.ID, &s.Type, &s.Topic, &s.StartTime, &durationSec)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	s.Duration = durationSec
	return &s, nil
}
