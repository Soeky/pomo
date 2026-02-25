package status

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
)

func TestCurrentStatus_NoActiveSession(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	res, err := CurrentStatus(time.Now())
	if err != nil {
		t.Fatalf("CurrentStatus failed: %v", err)
	}
	if res.Active {
		t.Fatalf("expected no active session")
	}
}

func TestCurrentStatus_ActiveFocusAndOverdue(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	prevConfig := config.AppConfig
	defer func() { config.AppConfig = prevConfig }()
	config.AppConfig.DefaultFocus = 25

	start := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	if _, err := opened.Exec(`INSERT INTO sessions(type, topic, start_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"focus", "Deep Work", start, int((20 * time.Minute).Seconds()), start, start); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	normalNow := start.Add(10 * time.Minute)
	res, err := CurrentStatus(normalNow)
	if err != nil {
		t.Fatalf("CurrentStatus failed: %v", err)
	}
	if !res.Active || res.Emoji != "🍅" {
		t.Fatalf("unexpected active result: %+v", res)
	}

	overdueEvenSecond := start.Add(21*time.Minute + 10*time.Second)
	resOverdue, err := CurrentStatus(overdueEvenSecond)
	if err != nil {
		t.Fatalf("CurrentStatus overdue failed: %v", err)
	}
	if resOverdue.Emoji != "💥" {
		t.Fatalf("expected overdue emoji explosion, got %q", resOverdue.Emoji)
	}
}

func TestCurrentStatus_BreakTypeUsesSleepEmoji(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	prevConfig := config.AppConfig
	defer func() { config.AppConfig = prevConfig }()
	config.AppConfig.DefaultBreak = 5

	start := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	if _, err := opened.Exec(`INSERT INTO sessions(type, topic, start_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"break", "", start, int((5 * time.Minute).Seconds()), start, start); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	res, err := CurrentStatus(start.Add(2 * time.Minute))
	if err != nil {
		t.Fatalf("CurrentStatus failed: %v", err)
	}
	if !res.Active || res.Emoji != "💤" {
		t.Fatalf("unexpected break status result: %+v", res)
	}
}

func TestCurrentStatus_OverdueOddSecondUsesTomatoEmoji(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	prevConfig := config.AppConfig
	defer func() { config.AppConfig = prevConfig }()
	config.AppConfig.DefaultFocus = 1

	start := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	if _, err := opened.Exec(`INSERT INTO sessions(type, topic, start_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"focus", "X", start, int((1 * time.Minute).Seconds()), start, start); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	overdueOddSecond := start.Add(2*time.Minute + 11*time.Second)
	res, err := CurrentStatus(overdueOddSecond)
	if err != nil {
		t.Fatalf("CurrentStatus failed: %v", err)
	}
	if res.Emoji != "🍅" {
		t.Fatalf("expected tomato emoji for odd overdue second, got %q", res.Emoji)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	opened, err := db.Open(filepath.Join(t.TempDir(), "pomo.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}

	prev := db.DB
	db.DB = opened
	t.Cleanup(func() {
		db.DB = prev
	})
	return opened
}
