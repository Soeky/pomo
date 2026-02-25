package status

import (
	"time"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/session"
)

type StatusResult struct {
	Active    bool
	Emoji     string
	Formatted string
}

func CurrentStatus(now time.Time) (StatusResult, error) {
	currentSession, err := db.GetCurrentSession()
	if err != nil {
		return StatusResult{}, err
	}

	if currentSession == nil {
		return StatusResult{Active: false}, nil
	}

	elapsed := now.Sub(currentSession.StartTime)

	var duration time.Duration
	var emoji string

	if currentSession.Duration.Valid {
		duration = time.Duration(currentSession.Duration.Int64) * time.Second
	} else {
		switch currentSession.Type {
		case "focus":
			duration = time.Duration(config.AppConfig.DefaultFocus) * time.Minute
		case "break":
			duration = time.Duration(config.AppConfig.DefaultBreak) * time.Minute
		}
	}

	switch currentSession.Type {
	case "focus":
		emoji = "🍅"
	case "break":
		emoji = "💤"
	}

	remaining := duration - elapsed
	formatted := session.FormatShortDuration(remaining)

	if remaining < 0 {
		if now.Second()%2 == 0 {
			emoji = "💥"
		} else {
			emoji = "🍅"
		}
	}

	return StatusResult{
		Active:    true,
		Emoji:     emoji,
		Formatted: formatted,
	}, nil
}
