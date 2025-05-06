package status

import (
	"fmt"
	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/session"
	"time"
)

func ShowStatus() {
	currentSession, err := db.GetCurrentSession()
	if err != nil {
		fmt.Println("error finding session:", err)
		return
	}

	if currentSession == nil {
		fmt.Println("📭 no active session.")
		return
	}

	now := time.Now()
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
	formattedTime := session.FormatShortDuration(remaining)

	if remaining < 0 {
		if now.Second()%2 == 0 {
			emoji = "💥"
		} else {
			emoji = "🍅"
		}
	}

	fmt.Printf("%s %s\n", emoji, formattedTime)
}
