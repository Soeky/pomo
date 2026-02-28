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
	if resFocus.Topic != "ProjectX::General" {
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

func TestStartFocusDurationParsingBranches(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	prevConfig := config.AppConfig
	defer func() { config.AppConfig = prevConfig }()
	config.AppConfig.DefaultFocus = 30

	resWithDuration, err := StartFocus([]string{"15m", "TopicA"})
	if err != nil {
		t.Fatalf("StartFocus with duration failed: %v", err)
	}
	if resWithDuration.Duration != 15*time.Minute || resWithDuration.Topic != "TopicA::General" {
		t.Fatalf("unexpected parsed start focus result: %+v", resWithDuration)
	}

	resFallback, err := StartFocus([]string{"TopicAsFirstArg"})
	if err != nil {
		t.Fatalf("StartFocus fallback failed: %v", err)
	}
	if resFallback.Duration != 30*time.Minute || resFallback.Topic != "TopicAsFirstArg::General" {
		t.Fatalf("unexpected fallback start focus result: %+v", resFallback)
	}
}

func TestStartFocusTopicHierarchy(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	prevConfig := config.AppConfig
	defer func() { config.AppConfig = prevConfig }()
	config.AppConfig.DefaultFocus = 25

	res, err := StartFocus([]string{"Applied Mathematics::Numerical Analysis"})
	if err != nil {
		t.Fatalf("StartFocus failed: %v", err)
	}
	if res.Topic != "Applied Mathematics::Numerical Analysis" {
		t.Fatalf("unexpected canonical topic: %s", res.Topic)
	}
}

func TestStartFocusEscapedDelimiterTopic(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	prevConfig := config.AppConfig
	defer func() { config.AppConfig = prevConfig }()
	config.AppConfig.DefaultFocus = 25

	res, err := StartFocus([]string{`Math\::History::Week 1`})
	if err != nil {
		t.Fatalf("StartFocus failed: %v", err)
	}
	if res.Topic != `Math\::History::Week 1` {
		t.Fatalf("unexpected escaped canonical topic: %s", res.Topic)
	}
}

func TestStartBreakFallbackToDefault(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	prevConfig := config.AppConfig
	defer func() { config.AppConfig = prevConfig }()
	config.AppConfig.DefaultBreak = 7

	res, err := StartBreak([]string{"not-a-duration"})
	if err != nil {
		t.Fatalf("StartBreak failed: %v", err)
	}
	if res.Duration != 7*time.Minute {
		t.Fatalf("unexpected default break duration: %v", res.Duration)
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

func TestStopIfRunningAndFormatShortDuration(t *testing.T) {
	opened := openTestDB(t)
	defer opened.Close()

	if stopped, err := StopIfRunning(); err != nil || stopped {
		t.Fatalf("expected no running session initially, stopped=%v err=%v", stopped, err)
	}

	start := time.Now().UTC().Add(-10 * time.Minute)
	if _, err := db.DB.Exec(`INSERT INTO sessions(type, topic, start_time, duration, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"focus", "X", start, 1500, start, start); err != nil {
		t.Fatalf("seed session failed: %v", err)
	}
	if stopped, err := StopIfRunning(); err != nil || !stopped {
		t.Fatalf("expected running session to be stopped, stopped=%v err=%v", stopped, err)
	}

	if got := FormatShortDuration(-90 * time.Second); got != "-01:30" {
		t.Fatalf("unexpected formatted negative duration: %s", got)
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
