package config

import (
	"testing"
)

func TestSetValueAndGetValueForExtendedKeys(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)

	AppConfig = Config{}
	LoadConfig()

	if err := SetValue("active_weekdays", "mon,wed,fri"); err != nil {
		t.Fatalf("SetValue active_weekdays failed: %v", err)
	}
	if err := SetValue("day_start", "07:30"); err != nil {
		t.Fatalf("SetValue day_start failed: %v", err)
	}
	if err := SetValue("web_mode", "on_demand"); err != nil {
		t.Fatalf("SetValue web_mode failed: %v", err)
	}

	gotDays, err := GetValue("active_weekdays")
	if err != nil {
		t.Fatalf("GetValue active_weekdays failed: %v", err)
	}
	if gotDays != "mon,wed,fri" {
		t.Fatalf("unexpected weekdays: %s", gotDays)
	}
	gotStart, err := GetValue("day_start")
	if err != nil {
		t.Fatalf("GetValue day_start failed: %v", err)
	}
	if gotStart != "07:30" {
		t.Fatalf("unexpected day_start: %s", gotStart)
	}
	gotMode, err := GetValue("web_mode")
	if err != nil {
		t.Fatalf("GetValue web_mode failed: %v", err)
	}
	if gotMode != "on_demand" {
		t.Fatalf("unexpected web_mode: %s", gotMode)
	}
}

func TestSetValueValidationForExtendedKeys(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)

	AppConfig = Config{}
	LoadConfig()

	if err := SetValue("day_start", "nope"); err == nil {
		t.Fatalf("expected day_start validation error")
	}
	if err := SetValue("active_weekdays", "xx,yy"); err == nil {
		t.Fatalf("expected active_weekdays validation error")
	}
	if err := SetValue("web_mode", "invalid"); err == nil {
		t.Fatalf("expected web_mode validation error")
	}
}
