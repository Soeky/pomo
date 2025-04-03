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

	var defaultDuration time.Duration
	var emoji string
	var typ string

	switch session.Type {
	case "focus":
		defaultDuration = 25 * time.Minute
		emoji = "🍅"
		typ = "F"
	case "break":
		defaultDuration = 5 * time.Minute
		emoji = "💤"
		typ = "B"
	}

	remaining := defaultDuration - elapsed
	formattedTime := formatShortDuration(remaining)

	// Wenn überzogen → Emoji wechseln je nach Sekunde
	if remaining < 0 {
		if now.Second()%2 == 0 {
			emoji = "💥"
		} else {
			emoji = "🍅"
		}
	}

	fmt.Printf("%s %s %s\n", emoji, typ, formattedTime)
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
