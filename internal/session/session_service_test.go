package session

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
)

func TestStartFocusAndBreak(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	prevConfig := config.AppConfig
	defer func() { config.AppConfig = prevConfig }()
	config.AppConfig.DefaultFocus = 25
	config.AppConfig.DefaultBreak = 5

	resFocus, err := StartFocus([]string{"ProjectX"})
	if err != nil {
		t.Fatalf("StartFocus failed: %v", err)
	}
	if resFocus.Topic != "ProjectX" {
		t.Fatalf("unexpected focus topic: %s", resFocus.Topic)
	}
	if resFocus.Duration != 25*time.Minute {
		t.Fatalf("unexpected focus duration: %v", resFocus.Duration)
	}
	if resFocus.StoppedPrevious {
		t.Fatalf("did not expect previous session stop on first run")
	}

	resBreak, err := StartBreak([]string{"10m"})
	if err != nil {
		t.Fatalf("StartBreak failed: %v", err)
	}
	if resBreak.Duration != 10*time.Minute {
		t.Fatalf("unexpected break duration: %v", resBreak.Duration)
	}
	if !resBreak.StoppedPrevious {
		t.Fatalf("expected previous focus session to be stopped")
	}
}

func TestStopSessionNoActive(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	res, err := StopSession()
	if err != nil {
		t.Fatalf("StopSession failed: %v", err)
	}
	if res.Stopped {
		t.Fatalf("expected no active session")
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
