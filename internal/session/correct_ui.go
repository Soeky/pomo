package session

import (
	"fmt"
	"time"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/parse"
)

type CorrectRequest struct {
	SessionType  string
	BackDuration time.Duration
	Topic        string
}

type CorrectResult struct {
	SessionType string
	Topic       string
	StartTime   time.Time
	Duration    time.Duration
}

func ParseCorrectArgs(args []string) (CorrectRequest, error) {
	if len(args) < 2 {
		return CorrectRequest{}, fmt.Errorf("expected at least 2 args")
	}

	sessionType := args[0]
	durationStr := args[1]

	backDuration, err := parse.ParseDurationFromArg(durationStr)
	if err != nil {
		return CorrectRequest{}, err
	}

	topic := "General"
	if sessionType == "start" && len(args) > 2 {
		topic = args[2]
	}

	return CorrectRequest{
		SessionType:  sessionType,
		BackDuration: backDuration,
		Topic:        topic,
	}, nil
}

func CorrectSession(now time.Time, req CorrectRequest) (CorrectResult, error) {
	var sType string
	var baseDuration time.Duration

	if req.SessionType == "start" {
		sType = "focus"
		baseDuration = time.Duration(config.AppConfig.DefaultFocus) * time.Minute
	} else if req.SessionType == "break" {
		sType = "break"
		baseDuration = time.Duration(config.AppConfig.DefaultBreak) * time.Minute
	} else {
		return CorrectResult{}, fmt.Errorf("invalid session type: %s", req.SessionType)
	}

	startTime := now.Add(-req.BackDuration)
	_ = db.StopCurrentSessionAt(startTime)

	totalDuration := baseDuration + req.BackDuration

	_, err := db.DB.Exec(`
        INSERT INTO sessions (type, topic, start_time, duration)
        VALUES (?, ?, ?, ?)
    `, sType, req.Topic, startTime, int(totalDuration.Seconds()))
	if err != nil {
		return CorrectResult{}, err
	}

	return CorrectResult{
		SessionType: sType,
		Topic:       req.Topic,
		StartTime:   startTime,
		Duration:    totalDuration,
	}, nil
}
