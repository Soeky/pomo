package config

import (
	"fmt"
	"os"
	"strconv"
)

func SetConfigValue(args []string) {
	if len(args) < 2 {
		fmt.Printf("Error: not enough args for set expected 2, was %v\n", len(args))
		os.Exit(1)
	}

	key := args[0]
	value := args[1]

	switch key {
	case "default_focus":
		minutes, err := strconv.Atoi(value)
		if err != nil {
			_ = fmt.Errorf("invalid number for default_focus: %s", value)
			os.Exit(1)
		}
		AppConfig.DefaultFocus = minutes
	case "default_break":
		minutes, err := strconv.Atoi(value)
		if err != nil {
			_ = fmt.Errorf("invalid number for default_break: %s", value)
			os.Exit(1)
		}
		AppConfig.DefaultBreak = minutes
	case "semester_start":
		AppConfig.SemesterStart = value
	default:
		_ = fmt.Errorf("unknown config key: %s", key)
		os.Exit(1)
	}
}
