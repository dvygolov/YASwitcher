package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/dvygolov/yaswitcher/internal/win"
)

const (
	appName    = "YASwitcher"
	appTitle   = "Yet Another Switcher"
	runKeyName = "YASwitcher"
	className  = "YASwitcherWindow"
)

// Config lives in %APPDATA%\YASwitcher\config.json.
type Config struct {
	Hotkey      string `json:"hotkey"`      // "Alt+Space"
	AltTabMode  bool   `json:"altTabMode"`  // replace the system Alt+Tab with our list
	AutoStart   bool   `json:"autoStart"`   // start with Windows (HKCU\...\Run)
	AutoUpdate  bool   `json:"autoUpdate"`  // check for updates in the background
	DarkTheme   bool   `json:"darkTheme"`   // dark or light overlay
	ShowMonitor bool   `json:"showMonitor"` // show "monitor N" hint in the header
	MaxResults  int    `json:"maxResults"`  // how many windows to display

	path string
}

func defaultConfig() *Config {
	return &Config{
		Hotkey:      "Alt+Space",
		AltTabMode:  false,
		AutoStart:   false,
		AutoUpdate:  true,
		DarkTheme:   true,
		ShowMonitor: true,
		MaxResults:  40,
	}
}

func configPath() string {
	dir := os.Getenv("APPDATA")
	if dir == "" {
		dir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
	}
	return filepath.Join(dir, appName, "config.json")
}

func loadConfig() *Config {
	cfg := defaultConfig()
	cfg.path = configPath()
	data, err := os.ReadFile(cfg.path)
	if err != nil {
		cfg.save()
		return cfg
	}
	_ = json.Unmarshal(data, cfg)
	if cfg.MaxResults <= 0 {
		cfg.MaxResults = 40
	}
	if cfg.Hotkey == "" || cfg.Hotkey == "Ctrl+Shift+Space" || cfg.Hotkey == "Ctrl+Alt+Space" {
		// Older builds defaulted to these; Alt+Space is now handled by the
		// keyboard hook even though the system owns it.
		cfg.Hotkey = "Alt+Space"
	}
	return cfg
}

func (c *Config) save() {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(c.path), 0o755)
	_ = os.WriteFile(c.path, data, 0o644)
}

// ensureAutoStart applies cfg.AutoStart to the registry. The default install
// autostarts through the "Startup Admin - YASwitcher" scheduled task, so
// AutoStart is off by default to avoid a double launch. If the user enables the
// Run-key option explicitly, it is written; otherwise any stale Run entry that
// points at this exe is removed.
func (c *Config) ensureAutoStart() {
	if c.AutoStart {
		win.SetAutostart(runKeyName, win.ExePath())
	} else {
		win.RemoveAutostart(runKeyName)
	}
}
