package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config is persisted at ~/.config/mkm/config.json. Missing/malformed files
// fall back to defaults silently — mkm must keep running even if the user's
// config is hosed. On first run we write defaults out so users have a
// template to edit.
type Config struct {
	Theme        string `json:"theme"`
	WriteHistory bool   `json:"write_history"`
	ShellHistory bool   `json:"shell_history"`
	CheckUpdates bool   `json:"check_updates"`
}

func defaultConfig() Config {
	return Config{
		Theme:        "nord",
		WriteHistory: true,
		ShellHistory: true,
		CheckUpdates: true,
	}
}

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "mkm", "config.json")
}

func loadConfig() Config {
	path := configPath()
	if path == "" {
		return defaultConfig()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		cfg := defaultConfig()
		writeConfig(path, cfg)
		return cfg
	}
	cfg := defaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultConfig()
	}
	return cfg
}

func writeConfig(path string, cfg Config) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}
