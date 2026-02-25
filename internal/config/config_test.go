package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigCreatesDefaultAndHandleSetCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)

	AppConfig = Config{}
	LoadConfig()
	if AppConfig.DefaultFocus != 25 || AppConfig.DefaultBreak != 5 {
		t.Fatalf("expected default config values, got %+v", AppConfig)
	}

	cfgPath := getConfigPath()
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("expected config file at %s: %v", cfgPath, err)
	}

	if err := HandleSetCommand([]string{"default_focus", "40"}); err != nil {
		t.Fatalf("HandleSetCommand default_focus failed: %v", err)
	}
	if err := HandleSetCommand([]string{"default_break", "10"}); err != nil {
		t.Fatalf("HandleSetCommand default_break failed: %v", err)
	}
	if err := HandleSetCommand([]string{"semester_start", "2026-02-01"}); err != nil {
		t.Fatalf("HandleSetCommand semester_start failed: %v", err)
	}

	AppConfig = Config{}
	LoadConfig()
	if AppConfig.DefaultFocus != 40 || AppConfig.DefaultBreak != 10 || AppConfig.SemesterStart != "2026-02-01" {
		t.Fatalf("expected persisted config values, got %+v", AppConfig)
	}
}

func TestHandleSetCommandValidation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)

	if err := HandleSetCommand([]string{"only-one"}); err == nil {
		t.Fatalf("expected arg length validation error")
	}
	if err := HandleSetCommand([]string{"unknown", "x"}); err == nil {
		t.Fatalf("expected unknown key error")
	}
	if err := HandleSetCommand([]string{"default_focus", "abc"}); err == nil {
		t.Fatalf("expected invalid number error")
	}
}

func TestLoadConfigWithInvalidJSONFallsBackToDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)

	cfgPath := getConfigPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte("{invalid-json"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	AppConfig = Config{}
	LoadConfig()
	if AppConfig.DefaultFocus != 25 || AppConfig.DefaultBreak != 5 {
		t.Fatalf("expected fallback defaults, got %+v", AppConfig)
	}
}
