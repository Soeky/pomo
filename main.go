package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	LoadConfig()
	InitDB()

	if len(os.Args) < 2 {
		fmt.Println("Usage: pomo <start|break|stop|status|stat>")
		return
	}

	command := os.Args[1]

	switch command {
	case "start":
		startFocus(os.Args[2:])
	case "break":
		startBreak(os.Args[2:])
	case "stop":
		stopSession()
	case "status":
		showStatus()
	case "stat":
		showStats(os.Args[2:])
	default:
		fmt.Println("Unbekannter Befehl. Nutze: start, break, stop, status, stat")
	}
}

func showStatus() {
	session, err := GetCurrentSession()
	if err != nil {
		fmt.Println("Fehler beim Abrufen der Session:", err)
		return
	}

	if session == nil {
		fmt.Println("📭 Keine aktive Session.")
		return
	}

	now := time.Now()
	elapsed := now.Sub(session.StartTime)

	var duration time.Duration
	var emoji string

	// Dauer aus DB (fallback auf config)
	if session.Duration.Valid {
		duration = time.Duration(session.Duration.Int64) * time.Second
	} else {
		switch session.Type {
		case "focus":
			duration = time.Duration(AppConfig.DefaultFocus) * time.Minute
		case "break":
			duration = time.Duration(AppConfig.DefaultBreak) * time.Minute
		}
	}

	switch session.Type {
	case "focus":
		emoji = "🍅"
	case "break":
		emoji = "💤"
	}

	remaining := duration - elapsed
	formattedTime := formatShortDuration(remaining)

	// Überziehung: Emoji wechselt
	if remaining < 0 {
		if now.Second()%2 == 0 {
			emoji = "💥"
		} else {
			emoji = "🍅"
		}
	}

	fmt.Printf("%s %s\n", emoji, formattedTime)
}

func formatShortDuration(d time.Duration) string {
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
