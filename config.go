package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	DefaultFocus int `json:"default_focus"` // in Minuten
	DefaultBreak int `json:"default_break"`
}

var AppConfig Config

func LoadConfig() {
	configPath := getConfigPath()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// config.json existiert nicht → erstellen
		AppConfig = Config{DefaultFocus: 25, DefaultBreak: 5}
		err := saveDefaultConfig(configPath)
		if err != nil {
			fmt.Println("⚠️  Konnte default config nicht schreiben:", err)
		} else {
			fmt.Println("📁 Default config erstellt unter:", configPath)
		}
		return
	}

	// Config laden
	file, err := os.Open(configPath)
	if err != nil {
		fmt.Println("⚠️  Fehler beim Öffnen von config.json:", err)
		AppConfig = Config{DefaultFocus: 25, DefaultBreak: 5}
		return
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&AppConfig)
	if err != nil {
		fmt.Println("⚠️  Fehler beim Parsen von config.json:", err)
		AppConfig = Config{DefaultFocus: 25, DefaultBreak: 5}
	}
}

func getConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		// fallback: HOME/.config/pomo/
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
