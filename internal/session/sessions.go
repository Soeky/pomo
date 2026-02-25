package session

import (
	"errors"

	"github.com/Soeky/pomo/internal/db"
)

type StopResult struct {
	Stopped bool
}

func StopSession() (StopResult, error) {
	err := db.StopCurrentSession()
	if err != nil {
		if errors.Is(err, db.ErrNoRunningSession) {
			return StopResult{Stopped: false}, nil
		}
		return StopResult{}, err
	}
	return StopResult{Stopped: true}, nil
}

func StopIfRunning() (bool, error) {
	err := db.StopCurrentSession()
	if err != nil {
		if errors.Is(err, db.ErrNoRunningSession) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
