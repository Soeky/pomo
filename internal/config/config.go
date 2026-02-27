package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DefaultFocus                int      `json:"default_focus"`
	DefaultBreak                int      `json:"default_break"`
	SemesterStart               string   `json:"semester_start"`
	BreakCreditThresholdMinutes int      `json:"break_credit_threshold_minutes"`
	DayStart                    string   `json:"day_start"`
	DayEnd                      string   `json:"day_end"`
	ActiveWeekdays              []string `json:"active_weekdays"`
	LunchStart                  string   `json:"lunch_start"`
	LunchDurationMinutes        int      `json:"lunch_duration_minutes"`
	DinnerStart                 string   `json:"dinner_start"`
	DinnerDurationMinutes       int      `json:"dinner_duration_minutes"`
	WebMode                     string   `json:"web_mode"`
}

type KeyDescription struct {
	Description string
	Example     string
}

var AppConfig Config

var keyDescriptions = map[string]KeyDescription{
	"active_weekdays": {
		Description: "Comma-separated weekdays used by planning/scheduling.",
		Example:     "mon,tue,wed,thu,fri",
	},
	"break_credit_threshold_minutes": {
		Description: "Max short break duration counted into effective focus metrics.",
		Example:     "10",
	},
	"day_end": {
		Description: "Default end of work/study day in HH:MM.",
		Example:     "22:00",
	},
	"day_start": {
		Description: "Default start of work/study day in HH:MM.",
		Example:     "08:00",
	},
	"default_break": {
		Description: "Default break session duration in minutes.",
		Example:     "5",
	},
	"default_focus": {
		Description: "Default focus session duration in minutes.",
		Example:     "25",
	},
	"dinner_duration_minutes": {
		Description: "Default dinner break duration in minutes.",
		Example:     "60",
	},
	"dinner_start": {
		Description: "Default dinner break start in HH:MM.",
		Example:     "19:00",
	},
	"lunch_duration_minutes": {
		Description: "Default lunch break duration in minutes.",
		Example:     "60",
	},
	"lunch_start": {
		Description: "Default lunch break start in HH:MM.",
		Example:     "12:30",
	},
	"semester_start": {
		Description: "Date used as start of semester stats in YYYY-MM-DD.",
		Example:     "2026-02-10",
	},
	"web_mode": {
		Description: "Preferred web runtime mode: daemon or on_demand.",
		Example:     "daemon",
	},
}

func defaultConfig() Config {
	return Config{
		DefaultFocus:                25,
		DefaultBreak:                5,
		SemesterStart:               "2000-01-01",
		BreakCreditThresholdMinutes: 10,
		DayStart:                    "08:00",
		DayEnd:                      "22:00",
		ActiveWeekdays:              []string{"mon", "tue", "wed", "thu", "fri"},
		LunchStart:                  "12:30",
		LunchDurationMinutes:        60,
		DinnerStart:                 "19:00",
		DinnerDurationMinutes:       60,
		WebMode:                     "daemon",
	}
}

func LoadConfig() {
	configPath := getConfigPath()
	defaults := defaultConfig()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		AppConfig = defaults
		err := saveDefaultConfig(configPath)
		if err != nil {
			fmt.Println("⚠️  default config couldn't be written to:", err)
		} else {
			fmt.Println("📁 default config created at:", configPath)
		}
		return
	}

	file, err := os.Open(configPath)
	if err != nil {
		fmt.Println("⚠️  error opening config.json:", err)
		AppConfig = defaults
		return
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&AppConfig); err != nil {
		fmt.Println("⚠️  error parsing config.json:", err)
		AppConfig = defaults
		return
	}

	normalizeConfig(&AppConfig)
}

func normalizeConfig(cfg *Config) {
	d := defaultConfig()

	if cfg.DefaultFocus <= 0 {
		cfg.DefaultFocus = d.DefaultFocus
	}
	if cfg.DefaultBreak <= 0 {
		cfg.DefaultBreak = d.DefaultBreak
	}
	if !isDate(cfg.SemesterStart) {
		cfg.SemesterStart = d.SemesterStart
	}
	if cfg.BreakCreditThresholdMinutes <= 0 {
		cfg.BreakCreditThresholdMinutes = d.BreakCreditThresholdMinutes
	}
	if !isClock(cfg.DayStart) {
		cfg.DayStart = d.DayStart
	}
	if !isClock(cfg.DayEnd) {
		cfg.DayEnd = d.DayEnd
	}
	if !isClock(cfg.LunchStart) {
		cfg.LunchStart = d.LunchStart
	}
	if cfg.LunchDurationMinutes <= 0 {
		cfg.LunchDurationMinutes = d.LunchDurationMinutes
	}
	if !isClock(cfg.DinnerStart) {
		cfg.DinnerStart = d.DinnerStart
	}
	if cfg.DinnerDurationMinutes <= 0 {
		cfg.DinnerDurationMinutes = d.DinnerDurationMinutes
	}
	cfg.ActiveWeekdays = normalizeWeekdaySlice(cfg.ActiveWeekdays)
	if len(cfg.ActiveWeekdays) == 0 {
		cfg.ActiveWeekdays = d.ActiveWeekdays
	}
	if cfg.WebMode != "daemon" && cfg.WebMode != "on_demand" {
		cfg.WebMode = d.WebMode
	}
}

func isClock(v string) bool {
	_, err := time.Parse("15:04", strings.TrimSpace(v))
	return err == nil
}

func isDate(v string) bool {
	_, err := time.Parse("2006-01-02", strings.TrimSpace(v))
	return err == nil
}

func normalizeWeekdaySlice(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, day := range in {
		if v, ok := normalizeWeekday(day); ok {
			if _, exists := seen[v]; !exists {
				out = append(out, v)
				seen[v] = struct{}{}
			}
		}
	}
	return out
}

func normalizeWeekday(token string) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(token))
	switch s {
	case "mon", "monday":
		return "mon", true
	case "tue", "tues", "tuesday":
		return "tue", true
	case "wed", "wednesday":
		return "wed", true
	case "thu", "thurs", "thursday":
		return "thu", true
	case "fri", "friday":
		return "fri", true
	case "sat", "saturday":
		return "sat", true
	case "sun", "sunday":
		return "sun", true
	default:
		return "", false
	}
}

func getConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}
	pomoDir := filepath.Join(configDir, "pomo")
	os.MkdirAll(pomoDir, 0755)
	return filepath.Join(pomoDir, "config.json")
}

func saveDefaultConfig(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(AppConfig)
}

func SaveConfig() error {
	path := getConfigPath()
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(AppConfig)
}

func KnownKeys() []string {
	keys := make([]string, 0, len(keyDescriptions))
	for k := range keyDescriptions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func DescribeKey(key string) (KeyDescription, error) {
	normalized := strings.ToLower(strings.TrimSpace(key))
	d, ok := keyDescriptions[normalized]
	if !ok {
		return KeyDescription{}, fmt.Errorf("unknown config key: %s", key)
	}
	return d, nil
}

func HandleSetCommand(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("expected <key> <value>")
	}
	return SetValue(args[0], args[1])
}

func SetValue(key, value string) error {
	normalized := strings.ToLower(strings.TrimSpace(key))
	value = strings.TrimSpace(value)

	switch normalized {
	case "default_focus":
		minutes, err := strconv.Atoi(value)
		if err != nil || minutes <= 0 {
			return fmt.Errorf("invalid number for default_focus: %s", value)
		}
		AppConfig.DefaultFocus = minutes
	case "default_break":
		minutes, err := strconv.Atoi(value)
		if err != nil || minutes <= 0 {
			return fmt.Errorf("invalid number for default_break: %s", value)
		}
		AppConfig.DefaultBreak = minutes
	case "semester_start":
		if !isDate(value) {
			return fmt.Errorf("invalid date for semester_start: %s", value)
		}
		AppConfig.SemesterStart = value
	case "break_credit_threshold_minutes":
		minutes, err := strconv.Atoi(value)
		if err != nil || minutes <= 0 {
			return fmt.Errorf("invalid number for break_credit_threshold_minutes: %s", value)
		}
		AppConfig.BreakCreditThresholdMinutes = minutes
	case "day_start":
		if !isClock(value) {
			return fmt.Errorf("invalid time for day_start (HH:MM): %s", value)
		}
		AppConfig.DayStart = value
	case "day_end":
		if !isClock(value) {
			return fmt.Errorf("invalid time for day_end (HH:MM): %s", value)
		}
		AppConfig.DayEnd = value
	case "active_weekdays":
		days := normalizeWeekdaySlice(strings.Split(value, ","))
		if len(days) == 0 {
			return fmt.Errorf("invalid active_weekdays value: %s", value)
		}
		AppConfig.ActiveWeekdays = days
	case "lunch_start":
		if !isClock(value) {
			return fmt.Errorf("invalid time for lunch_start (HH:MM): %s", value)
		}
		AppConfig.LunchStart = value
	case "lunch_duration_minutes":
		minutes, err := strconv.Atoi(value)
		if err != nil || minutes <= 0 {
			return fmt.Errorf("invalid number for lunch_duration_minutes: %s", value)
		}
		AppConfig.LunchDurationMinutes = minutes
	case "dinner_start":
		if !isClock(value) {
			return fmt.Errorf("invalid time for dinner_start (HH:MM): %s", value)
		}
		AppConfig.DinnerStart = value
	case "dinner_duration_minutes":
		minutes, err := strconv.Atoi(value)
		if err != nil || minutes <= 0 {
			return fmt.Errorf("invalid number for dinner_duration_minutes: %s", value)
		}
		AppConfig.DinnerDurationMinutes = minutes
	case "web_mode":
		if value != "daemon" && value != "on_demand" {
			return fmt.Errorf("web_mode must be daemon or on_demand")
		}
		AppConfig.WebMode = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	normalizeConfig(&AppConfig)
	if err := SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✔ %s set to %s\n", normalized, value)
	return nil
}

func GetValue(key string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(key))

	switch normalized {
	case "default_focus":
		return strconv.Itoa(AppConfig.DefaultFocus), nil
	case "default_break":
		return strconv.Itoa(AppConfig.DefaultBreak), nil
	case "semester_start":
		return AppConfig.SemesterStart, nil
	case "break_credit_threshold_minutes":
		return strconv.Itoa(AppConfig.BreakCreditThresholdMinutes), nil
	case "day_start":
		return AppConfig.DayStart, nil
	case "day_end":
		return AppConfig.DayEnd, nil
	case "active_weekdays":
		return strings.Join(AppConfig.ActiveWeekdays, ","), nil
	case "lunch_start":
		return AppConfig.LunchStart, nil
	case "lunch_duration_minutes":
		return strconv.Itoa(AppConfig.LunchDurationMinutes), nil
	case "dinner_start":
		return AppConfig.DinnerStart, nil
	case "dinner_duration_minutes":
		return strconv.Itoa(AppConfig.DinnerDurationMinutes), nil
	case "web_mode":
		return AppConfig.WebMode, nil
	default:
		return "", fmt.Errorf("unknown config key: %s", key)
	}
}

func ListValues() map[string]string {
	out := make(map[string]string, len(keyDescriptions))
	for _, key := range KnownKeys() {
		v, err := GetValue(key)
		if err == nil {
			out[key] = v
		}
	}
	return out
}
