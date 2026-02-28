package web

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func parseTimeOrDefault(v string, fallback time.Time) time.Time {
	t, err := parseAnyTime(v)
	if err != nil {
		return fallback
	}
	return t
}

func parseAnyTime(v string) (time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	layouts := []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, v); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time: %s", v)
}

func parsePrefixedID(id string) (string, int, error) {
	parts := strings.SplitN(id, "-", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("id must be prefixed like e-3 (legacy s-/p- forms are deprecated compatibility aliases)")
	}
	if parts[0] != "p" && parts[0] != "s" && parts[0] != "e" {
		return "", 0, fmt.Errorf("unsupported id prefix")
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid numeric id")
	}
	return parts[0], n, nil
}
