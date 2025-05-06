package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var timePattern = regexp.MustCompile(`(?i)(\d+)([smh])`)

func ParseDurationFromArg(arg string) (time.Duration, error) {
	matches := timePattern.FindAllStringSubmatch(arg, -1)
	if matches == nil {
		return 0, fmt.Errorf("invalid time format: %s", arg)
	}

	var total time.Duration
	for _, m := range matches {
		val, _ := strconv.Atoi(m[1])
		switch m[2] {
		case "s", "S":
			total += time.Duration(val) * time.Second
		case "m", "M":
			total += time.Duration(val) * time.Minute
		case "h", "H":
			total += time.Duration(val) * time.Hour
		}
	}

	return total, nil
}
