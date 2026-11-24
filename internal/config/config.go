package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Config stores persistent user settings in TOML format.
type Config struct {
	Editor        string              `toml:"editor"`
	Terminal      string              `toml:"terminal"`
	Autostart     bool                `toml:"autostart"`
	Refinement    RefinementConfig    `toml:"refinement"`
	Notifications NotificationsConfig `toml:"notifications"`
	Plugins       map[string]bool     `toml:"plugins"`
}

// RefinementConfig controls the weekly refinement schedule.
type RefinementConfig struct {
	Enabled     bool   `toml:"enabled"`
	Weekday     string `toml:"weekday"`      // sunday, monday, ..., saturday
	Time        string `toml:"time"`         // HH:MM (24-hour local time)
	SnoozeHours int    `toml:"snooze_hours"` // Default snooze interval in hours
}

// NotificationsConfig controls desktop notification categories.
type NotificationsConfig struct {
	Refinement      bool `toml:"refinement"`        // Enabled by default
	Routines        bool `toml:"routines"`          // Opt-in
	TaskDeadlines   bool `toml:"task_deadlines"`    // Opt-in
	TaskLeadMinutes int  `toml:"task_lead_minutes"` // Notify this many minutes before due_at
}

// Default returns sensible out-of-the-box defaults.
func Default() Config {
	return Config{
		Editor:    "",
		Terminal:  "",
		Autostart: false,
		Refinement: RefinementConfig{
			Enabled:     true,
			Weekday:     "sunday",
			Time:        "18:00",
			SnoozeHours: 24,
		},
		Notifications: NotificationsConfig{
			Refinement:      true,
			Routines:        false,
			TaskDeadlines:   false,
			TaskLeadMinutes: 60,
		},
		Plugins: map[string]bool{},
	}
}

// Load reads a TOML config file, applying defaults for any omitted fields.
// If the file does not exist, Default() is returned with no error.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.Plugins == nil {
		cfg.Plugins = map[string]bool{}
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Save writes the config to disk atomically (write temp + rename).
func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

// Validate checks that schedule and notification settings are well-formed.
func (c Config) Validate() error {
	if _, err := ParseWeekday(c.Refinement.Weekday); err != nil {
		return fmt.Errorf("refinement.weekday: %w", err)
	}
	if _, _, err := ParseClockTime(c.Refinement.Time); err != nil {
		return fmt.Errorf("refinement.time: %w", err)
	}
	if c.Refinement.SnoozeHours < 1 {
		return errors.New("refinement.snooze_hours must be >= 1")
	}
	if c.Notifications.TaskLeadMinutes < 0 {
		return errors.New("notifications.task_lead_minutes must be >= 0")
	}
	return nil
}

// ParseWeekday converts a weekday string (case-insensitive, full or 3-letter) to time.Weekday.
func ParseWeekday(s string) (time.Weekday, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "sunday", "sun":
		return time.Sunday, nil
	case "monday", "mon":
		return time.Monday, nil
	case "tuesday", "tue":
		return time.Tuesday, nil
	case "wednesday", "wed":
		return time.Wednesday, nil
	case "thursday", "thu":
		return time.Thursday, nil
	case "friday", "fri":
		return time.Friday, nil
	case "saturday", "sat":
		return time.Saturday, nil
	default:
		return time.Sunday, fmt.Errorf("invalid weekday %q", s)
	}
}

// ParseClockTime parses a 24-hour "HH:MM" string.
func ParseClockTime(s string) (hour int, minute int, err error) {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid HH:MM time %q", s)
	}
	return t.Hour(), t.Minute(), nil
}
