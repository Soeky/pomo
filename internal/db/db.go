package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // SQLite driver
)

var DB *sql.DB

func InitDB() {
	dbPath := getDBPath()

	var err error
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal("DB open error:", err)
	}

	if err := runMigrations(DB); err != nil {
		log.Fatal("DB migration error:", err)
	}
}

func getDBPath() string {
	return GetDBPath()
}

func GetDBPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Println("❌ error finding home directory. Saving pomo.db in current working directory.")
		return "pomo.db"
	}

	pomoDir := filepath.Join(homeDir, ".local", "share", "pomo")
	err = os.MkdirAll(pomoDir, 0755)
	if err != nil {
		log.Printf("❌ could not create %s : %v\nsaving pomo.db in current working directory.", pomoDir, err)
		return "pomo.db"
	}

	return filepath.Join(pomoDir, "pomo.db")
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
		return fmt.Errorf("could not find running session")
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
		return nil // Keine offene Session, kein Fehler
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

func runMigrations(db *sql.DB) error {
	if _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS schema_migrations (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL UNIQUE,
            applied_at DATETIME NOT NULL
        );
    `); err != nil {
		return err
	}

	migrations := []struct {
		name string
		sql  string
	}{
		{
			name: "001_base_sessions",
			sql: `
                CREATE TABLE IF NOT EXISTS sessions (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    type TEXT NOT NULL CHECK(type IN ('focus', 'break')),
                    topic TEXT,
                    start_time DATETIME NOT NULL,
                    end_time DATETIME,
                    duration INTEGER
                );
                CREATE INDEX IF NOT EXISTS idx_start_time ON sessions(start_time);
            `,
		},
		{
			name: "002_sessions_metadata",
			sql: `
                ALTER TABLE sessions ADD COLUMN planned_event_id INTEGER;
                ALTER TABLE sessions ADD COLUMN created_at DATETIME;
                ALTER TABLE sessions ADD COLUMN updated_at DATETIME;
            `,
		},
		{
			name: "003_planned_events",
			sql: `
                CREATE TABLE IF NOT EXISTS planned_events (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    title TEXT NOT NULL,
                    description TEXT,
                    start_time DATETIME NOT NULL,
                    end_time DATETIME NOT NULL,
                    status TEXT NOT NULL DEFAULT 'planned' CHECK(status IN ('planned','done','canceled')),
                    source TEXT NOT NULL DEFAULT 'manual' CHECK(source IN ('manual','scheduler')),
                    created_at DATETIME NOT NULL,
                    updated_at DATETIME NOT NULL
                );
                CREATE INDEX IF NOT EXISTS idx_planned_events_time ON planned_events(start_time, end_time);
            `,
		},
		{
			name: "004_audit_log",
			sql: `
                CREATE TABLE IF NOT EXISTS audit_log (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    entity_type TEXT NOT NULL,
                    entity_id INTEGER NOT NULL,
                    action TEXT NOT NULL,
                    before_json TEXT,
                    after_json TEXT,
                    changed_at DATETIME NOT NULL,
                    origin TEXT NOT NULL
                );
                CREATE INDEX IF NOT EXISTS idx_audit_entity ON audit_log(entity_type, entity_id);
            `,
		},
		{
			name: "005_sessions_indexes",
			sql: `
                CREATE INDEX IF NOT EXISTS idx_sessions_planned_event ON sessions(planned_event_id);
            `,
		},
	}

	for _, m := range migrations {
		applied, err := migrationApplied(db, m.name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		if m.name == "002_sessions_metadata" {
			if err := addColumnIfMissing(db, "sessions", "planned_event_id", "INTEGER"); err != nil {
				return err
			}
			if err := addColumnIfMissing(db, "sessions", "created_at", "DATETIME"); err != nil {
				return err
			}
			if err := addColumnIfMissing(db, "sessions", "updated_at", "DATETIME"); err != nil {
				return err
			}
		} else {
			if _, err := db.Exec(m.sql); err != nil {
				return fmt.Errorf("%s: %w", m.name, err)
			}
		}

		if _, err := db.Exec(`INSERT INTO schema_migrations(name, applied_at) VALUES (?, ?)`, m.name, time.Now()); err != nil {
			return err
		}
	}

	if _, err := db.Exec(`
        UPDATE sessions
        SET created_at = COALESCE(created_at, start_time),
            updated_at = COALESCE(updated_at, COALESCE(end_time, start_time))
    `); err != nil {
		return err
	}

	return nil
}

func migrationApplied(db *sql.DB, name string) (bool, error) {
	var count int
	if err := db.QueryRow(`SELECT COUNT(1) FROM schema_migrations WHERE name = ?`, name).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func addColumnIfMissing(db *sql.DB, tableName, columnName, columnType string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == columnName {
			return nil
		}
	}

	_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, columnType))
	return err
}
