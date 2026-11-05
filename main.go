//go:build windows

// YASwitcher — yet another window switcher for Windows. A lightweight Alt-Tab
// replacement and "type to find a window" launcher, written in Go against raw
// Win32 with no CGO and no third-party dependencies.
package main

import (
	"os"

	"github.com/dvygolov/yaswitcher/internal/win"
)

// version is injected at build time with -ldflags "-X main.version=1.2.3".
var version = "dev"

func main() {
	// One copy per session: a second launch exits immediately. Autostart via the
	// scheduled task is the normal entry point, so a duplicate is expected.
	if !win.SingleInstance("YASwitcher.SingleInstance") {
		os.Exit(0)
	}
	cfg := loadConfig()
	cfg.ensureAutoStart()
	app := NewApp(cfg)
	app.Start()
}
