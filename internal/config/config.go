package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	DefaultFocus int `json:"default_focus"`
	DefaultBreak int `json:"default_break"`
}

var AppConfig Config

func LoadConfig() {
	configPath := getConfigPath()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		AppConfig = Config{DefaultFocus: 25, DefaultBreak: 5}
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
		AppConfig = Config{DefaultFocus: 25, DefaultBreak: 5}
		return
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&AppConfig)
	if err != nil {
		fmt.Println("⚠️  error parsing config.json:", err)
		AppConfig = Config{DefaultFocus: 25, DefaultBreak: 5}
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
