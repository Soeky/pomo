package session

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
)

func TestParseCorrectArgs(t *testing.T) {
	t.Parallel()

	req, err := ParseCorrectArgs([]string{"start", "15m", "ProjectX"})
	if err != nil {
		t.Fatalf("ParseCorrectArgs failed: %v", err)
	}
	if req.SessionType != "start" {
		t.Fatalf("unexpected session type: %s", req.SessionType)
	}
	if req.Topic != "ProjectX" {
		t.Fatalf("unexpected topic: %s", req.Topic)
	}
	if req.BackDuration != 15*time.Minute {
		t.Fatalf("unexpected duration: %v", req.BackDuration)
	}
}

func TestParseCorrectArgsInvalidDuration(t *testing.T) {
	t.Parallel()

	_, err := ParseCorrectArgs([]string{"start", "nonsense"})
	if err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestParseCorrectArgsTooShort(t *testing.T) {
	t.Parallel()

	_, err := ParseCorrectArgs([]string{"start"})
	if err == nil {
		t.Fatalf("expected arg length error")
	}
}

func TestCorrectSessionRejectsInvalidType(t *testing.T) {
	t.Parallel()

	_, err := CorrectSession(time.Now(), CorrectRequest{
		SessionType:  "invalid",
		BackDuration: 5 * time.Minute,
		Topic:        "x",
	})
	if err == nil {
		t.Fatalf("expected invalid type error")
	}
}

func TestCorrectSessionCreatesSession(t *testing.T) {
	opened := openCorrectDB(t)
	defer opened.Close()

	prevConfig := config.AppConfig
	defer func() { config.AppConfig = prevConfig }()
	config.AppConfig.DefaultFocus = 25

	now := time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)
	res, err := CorrectSession(now, CorrectRequest{
		SessionType:  "start",
		BackDuration: 10 * time.Minute,
		Topic:        "Retro",
	})
	if err != nil {
		t.Fatalf("CorrectSession failed: %v", err)
	}
	if res.SessionType != "focus" {
		t.Fatalf("unexpected session type: %s", res.SessionType)
	}

	var count int
	if err := opened.QueryRow(`SELECT COUNT(1) FROM sessions WHERE topic = ?`, "Retro").Scan(&count); err != nil {
		t.Fatalf("count corrected sessions failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one corrected session, got %d", count)
	}
}

func openCorrectDB(t *testing.T) *sql.DB {
	t.Helper()
	opened, err := db.Open(filepath.Join(t.TempDir(), "pomo.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	prev := db.DB
	db.DB = opened
	t.Cleanup(func() { db.DB = prev })
	return opened
}
