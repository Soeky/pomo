package session

import (
	"fmt"
	"time"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/parse"
)

func StartFocus(args []string) {
	StopIfRunning()

	var duration time.Duration
	var topic string = "General"

	if len(args) > 0 {
		parsed, err := parse.ParseDurationFromArg(args[0])
		if err == nil {
			duration = parsed
			if len(args) > 1 {
				topic = args[1]
			}
		} else {
			duration = time.Duration(config.AppConfig.DefaultFocus) * time.Minute
			topic = args[0]
		}
	} else {
		duration = time.Duration(config.AppConfig.DefaultFocus) * time.Minute
	}

	id, err := db.InsertSession("focus", topic, duration)
	if err != nil {
		fmt.Println("❌ Fehler beim Starten der Fokus-Session:", err)
		return
	}

	fmt.Printf("🍅 Fokus gestartet: \"%s\" für %s (ID %d)\n", topic, FormatShortDuration(duration), id)
}

func StartBreak(args []string) {
	StopIfRunning()

	var duration time.Duration

	if len(args) > 0 {
		parsed, err := parse.ParseDurationFromArg(args[0])
		if err == nil {
			duration = parsed
		} else {
			duration = time.Duration(config.AppConfig.DefaultBreak) * time.Minute
		}
	} else {
		duration = time.Duration(config.AppConfig.DefaultBreak) * time.Minute
	}

	id, err := db.InsertSession("break", "", duration)
	if err != nil {
		fmt.Println("❌ Fehler beim Starten der Pause:", err)
		return
	}

	fmt.Printf("💤 Pause gestartet für %s (ID %d)\n", FormatShortDuration(duration), id)
}

func FormatShortDuration(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	sign := ""
	if totalSeconds < 0 {
		sign = "-"
		totalSeconds = -totalSeconds
	}
	m := totalSeconds / 60
	s := totalSeconds % 60
	return fmt.Sprintf("%s%02d:%02d", sign, m, s)
}
