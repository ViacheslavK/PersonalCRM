package config_test

import (
	"path/filepath"
	"testing"

	"personalcrm/internal/config"
	"personalcrm/internal/platform"
)

func TestConfigDefaultLoadSave(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	// Non-existent file yields defaults cleanly.
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load missing config: %v", err)
	}
	if !cfg.Refinement.Enabled || cfg.Refinement.Weekday != "sunday" || cfg.Refinement.Time != "18:00" {
		t.Fatalf("unexpected refinement defaults: %+v", cfg.Refinement)
	}
	if !cfg.Notifications.Refinement || cfg.Notifications.Routines || cfg.Notifications.TaskDeadlines {
		t.Fatalf("unexpected notification defaults: %+v", cfg.Notifications)
	}

	// Modify and save.
	cfg.Editor = "nvim"
	cfg.Autostart = true
	cfg.Refinement.Weekday = "Friday"
	cfg.Refinement.Time = "17:30"
	cfg.Notifications.Routines = true
	cfg.Plugins["people"] = true
	cfg.Plugins["reports"] = false

	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load saved config: %v", err)
	}
	if loaded.Editor != "nvim" || !loaded.Autostart || !loaded.Notifications.Routines {
		t.Fatalf("round-trip mismatch: %+v", loaded)
	}
	if !loaded.Plugins["people"] || loaded.Plugins["reports"] {
		t.Fatalf("plugin flags mismatch: %+v", loaded.Plugins)
	}
}

func TestConfigValidation(t *testing.T) {
	bad := config.Default()
	bad.Refinement.Weekday = "funday"
	if err := bad.Validate(); err == nil {
		t.Fatal("expected error for invalid weekday")
	}

	bad = config.Default()
	bad.Refinement.Time = "25:99"
	if err := bad.Validate(); err == nil {
		t.Fatal("expected error for invalid time")
	}

	bad = config.Default()
	bad.Refinement.SnoozeHours = 0
	if err := bad.Validate(); err == nil {
		t.Fatal("expected error for snooze_hours < 1")
	}
}

func TestResolvePathsPrecedence(t *testing.T) {
	overrideDir := filepath.Join(t.TempDir(), "custom-data")
	t.Setenv(platform.EnvDataDir, filepath.Join(t.TempDir(), "env-data"))

	p, err := platform.ResolvePaths(overrideDir)
	if err != nil {
		t.Fatalf("ResolvePaths override: %v", err)
	}
	if p.DataDir != overrideDir || p.DBPath != filepath.Join(overrideDir, platform.DBFileName) {
		t.Fatalf("expected override dir %s, got %+v", overrideDir, p)
	}

	if err := platform.EnsureDirs(p); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
}
