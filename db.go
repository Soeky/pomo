package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite" // SQLite driver
)

var db *sql.DB

func InitDB() {
	var err error
	db, err = sql.Open("sqlite", "pomo.db")
	if err != nil {
		log.Fatal("DB open error:", err)
	}

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS sessions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            type TEXT NOT NULL CHECK(type IN ('focus', 'break')),
            topic TEXT,
            start_time DATETIME NOT NULL,
            end_time DATETIME,
            duration INTEGER
        );
        CREATE INDEX IF NOT EXISTS idx_start_time ON sessions(start_time);
    `)
	if err != nil {
		log.Fatal("DB init error:", err)
	}
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
	res, err := db.Exec(`
        INSERT INTO sessions (type, topic, start_time, duration)
        VALUES (?, ?, ?, ?)`, sType, topic, time.Now(), int(duration.Seconds()))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func StopCurrentSession() error {
	row := db.QueryRow(`SELECT id, start_time FROM sessions WHERE end_time IS NULL ORDER BY start_time DESC LIMIT 1`)

	var id int
	var startTime time.Time

	err := row.Scan(&id, &startTime)
	if err == sql.ErrNoRows {
		return fmt.Errorf("Keine laufende Session gefunden")
	} else if err != nil {
		return err
	}

	endTime := time.Now()
	duration := int(endTime.Sub(startTime).Seconds())

	_, err = db.Exec(`
        UPDATE sessions
        SET end_time = ?, duration = ?
        WHERE id = ?
    `, endTime, duration, id)

	return err
}

func GetCurrentSession() (*Session, error) {
	row := db.QueryRow(`
        SELECT id, type, topic, start_time
        FROM sessions
        WHERE end_time IS NULL
        ORDER BY start_time DESC
        LIMIT 1
    `)

	var s Session
	err := row.Scan(&s.ID, &s.Type, &s.Topic, &s.StartTime)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return &s, nil
}
