package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	DefaultFocus  int    `json:"default_focus"`
	DefaultBreak  int    `json:"default_break"`
	SemesterStart string `json:"semester_start"`
}

var AppConfig Config

func LoadConfig() {
	configPath := getConfigPath()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		AppConfig = Config{
			DefaultFocus:  25,
			DefaultBreak:  5,
			SemesterStart: "2000-01-01",
		}
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
		AppConfig = Config{
			DefaultFocus:  25,
			DefaultBreak:  5,
			SemesterStart: "2000-01-01",
		}
		return
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&AppConfig)
	if err != nil {
		fmt.Println("⚠️  error parsing config.json:", err)
		AppConfig = Config{
			DefaultFocus:  25,
			DefaultBreak:  5,
			SemesterStart: "2000-01-01",
		}
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

func HandleSetCommand(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("expected <key> <value>")
	}

	key := args[0]
	value := args[1]

	switch key {
	case "default_focus":
		minutes, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid number for default_focus: %s", value)
		}
		AppConfig.DefaultFocus = minutes
	case "default_break":
		minutes, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid number for default_break: %s", value)
		}
		AppConfig.DefaultBreak = minutes
	case "semester_start":
		AppConfig.SemesterStart = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	err := SaveConfig()
	if err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✔ %s set to %s\n", key, value)
	return nil
}
