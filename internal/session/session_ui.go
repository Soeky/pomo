package session

import (
	"fmt"
	"time"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/parse"
)

type StartResult struct {
	Type            string
	Topic           string
	Duration        time.Duration
	ID              int64
	StoppedPrevious bool
}

func StartFocus(args []string) (StartResult, error) {
	stoppedPrevious, err := StopIfRunning()
	if err != nil {
		return StartResult{}, err
	}

	var duration time.Duration
	topic := "General"

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
		return StartResult{}, err
	}

	return StartResult{
		Type:            "focus",
		Topic:           topic,
		Duration:        duration,
		ID:              id,
		StoppedPrevious: stoppedPrevious,
	}, nil
}

func StartBreak(args []string) (StartResult, error) {
	stoppedPrevious, err := StopIfRunning()
	if err != nil {
		return StartResult{}, err
	}

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
		return StartResult{}, err
	}

	return StartResult{
		Type:            "break",
		Duration:        duration,
		ID:              id,
		StoppedPrevious: stoppedPrevious,
	}, nil
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
