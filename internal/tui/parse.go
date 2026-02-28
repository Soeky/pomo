package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/parse"
)

func parseDateTime(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, fmt.Errorf("missing datetime")
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	var parsed time.Time
	var err error
	for _, layout := range layouts {
		parsed, err = time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("expected RFC3339 or 2006-01-02T15:04")
}

func parseOptionalDateTime(raw string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parsed, err := parseDateTime(raw)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseDurationSeconds(raw string) (int, error) {
	duration, err := parse.ParseDurationFromArg(strings.TrimSpace(raw))
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid duration")
	}
	return int(duration.Seconds()), nil
}

func parseOptionalInt(raw string, defaultValue int) (int, error) {
	token := strings.TrimSpace(raw)
	if token == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(token)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func parseRequiredInt64(raw string, field string) (int64, error) {
	token := strings.TrimSpace(raw)
	if token == "" {
		return 0, fmt.Errorf("%s is required", field)
	}
	value, err := strconv.ParseInt(token, 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid %s", field)
	}
	return value, nil
}

func parseBoolDefault(raw string, defaultValue bool) (bool, error) {
	token := strings.TrimSpace(strings.ToLower(raw))
	if token == "" {
		return defaultValue, nil
	}
	switch token {
	case "1", "true", "yes", "y", "on":
		return true, nil
	case "0", "false", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean: %s", raw)
	}
}
