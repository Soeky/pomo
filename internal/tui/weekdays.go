package tui

import (
	"fmt"
	"strings"
	"time"
)

func parseRecurrenceWeekdays(raw string) ([]time.Weekday, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	seen := make(map[time.Weekday]struct{}, len(parts))
	out := make([]time.Weekday, 0, len(parts))
	for _, part := range parts {
		token := strings.ToUpper(strings.TrimSpace(part))
		var day time.Weekday
		switch token {
		case "MO", "MON", "MONDAY":
			day = time.Monday
		case "TU", "TUE", "TUESDAY":
			day = time.Tuesday
		case "WE", "WED", "WEDNESDAY":
			day = time.Wednesday
		case "TH", "THU", "THURSDAY":
			day = time.Thursday
		case "FR", "FRI", "FRIDAY":
			day = time.Friday
		case "SA", "SAT", "SATURDAY":
			day = time.Saturday
		case "SU", "SUN", "SUNDAY":
			day = time.Sunday
		default:
			return nil, fmt.Errorf("unsupported weekday token: %s", part)
		}
		if _, exists := seen[day]; exists {
			continue
		}
		seen[day] = struct{}{}
		out = append(out, day)
	}
	return out, nil
}

func normalizeConstraintWeekdays(raw string) ([]string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, fmt.Errorf("active weekdays cannot be empty")
	}
	parts := strings.Split(value, ",")
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		token := strings.ToLower(strings.TrimSpace(part))
		switch token {
		case "mon", "monday":
			token = "mon"
		case "tue", "tues", "tuesday":
			token = "tue"
		case "wed", "wednesday":
			token = "wed"
		case "thu", "thurs", "thursday":
			token = "thu"
		case "fri", "friday":
			token = "fri"
		case "sat", "saturday":
			token = "sat"
		case "sun", "sunday":
			token = "sun"
		default:
			return nil, fmt.Errorf("unsupported weekday token: %s", part)
		}
		if _, exists := seen[token]; exists {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out, nil
}
