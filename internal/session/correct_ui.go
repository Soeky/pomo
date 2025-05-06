package session

import (
	"fmt"
	"time"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/parse"
)

func HandleCorrectCommand(args []string) {
	sessionType := args[0]
	durationStr := args[1]

	backDuration, err := parse.ParseDurationFromArg(durationStr)
	if err != nil {
		fmt.Println("❌ invalid time format:", err)
		return
	}

	topic := "General"
	if sessionType == "start" && len(args) > 2 {
		topic = args[2]
	}

	err = CorrectSession(sessionType, backDuration, topic)
	if err != nil {
		fmt.Println("❌ there was an error while correcting:", err)
	} else {
		fmt.Println("✅ session has been corrected!")
	}
}

func CorrectSession(sessionType string, backDuration time.Duration, topic string) error {
	startTime := time.Now().Add(-backDuration)

	// Laufende Session stoppen auf Startzeit
	_ = db.StopCurrentSessionAt(startTime)

	// Sessiontyp und Basisdauer
	var sType string
	var baseDuration time.Duration

	if sessionType == "start" {
		sType = "focus"
		baseDuration = time.Duration(config.AppConfig.DefaultFocus) * time.Minute
	} else if sessionType == "break" {
		sType = "break"
		baseDuration = time.Duration(config.AppConfig.DefaultBreak) * time.Minute
	} else {
		return fmt.Errorf("invalid session type: %s", sessionType)
	}

	// WICHTIG: neue Dauer = baseDuration + backDuration
	totalDuration := baseDuration + backDuration

	_, err := db.DB.Exec(`
        INSERT INTO sessions (type, topic, start_time, duration)
        VALUES (?, ?, ?, ?)
    `, sType, topic, startTime, int(totalDuration.Seconds()))

	return err
}
