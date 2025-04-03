package main

import (
	"fmt"
	"time"
)

func startFocus(args []string) {
	stopIfRunning()

	var duration time.Duration
	var topic string = "General"

	if len(args) > 0 {
		parsed, err := ParseDurationFromArg(args[0])
		if err == nil {
			duration = parsed
			if len(args) > 1 {
				topic = args[1]
			}
		} else {
			duration = time.Duration(AppConfig.DefaultFocus) * time.Minute
			topic = args[0]
		}
	} else {
		duration = time.Duration(AppConfig.DefaultFocus) * time.Minute
	}

	id, err := InsertSession("focus", topic, duration)
	if err != nil {
		fmt.Println("❌ Fehler beim Starten der Fokus-Session:", err)
		return
	}

	fmt.Printf("🍅 Fokus gestartet: \"%s\" für %s (ID %d)\n", topic, formatShortDuration(duration), id)
}

func startBreak(args []string) {
	stopIfRunning()

	var duration time.Duration

	if len(args) > 0 {
		parsed, err := ParseDurationFromArg(args[0])
		if err == nil {
			duration = parsed
		} else {
			duration = time.Duration(AppConfig.DefaultBreak) * time.Minute
		}
	} else {
		duration = time.Duration(AppConfig.DefaultBreak) * time.Minute
	}

	id, err := InsertSession("break", "", duration)
	if err != nil {
		fmt.Println("❌ Fehler beim Starten der Pause:", err)
		return
	}

	fmt.Printf("💤 Pause gestartet für %s (ID %d)\n", formatShortDuration(duration), id)
}

func stopSession() {
	err := StopCurrentSession()
	if err != nil {
		fmt.Println("❌ Fehler beim Stoppen:", err)
		return
	}
	fmt.Println("🛑 Session gestoppt")
}

func stopIfRunning() {
	err := StopCurrentSession()
	if err == nil {
		fmt.Println("vorherige session gestoppt")
	}
}
