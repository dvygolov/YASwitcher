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
	// one copy per session; a second launch just opens the existing switcher
	if !win.SingleInstance("YASwitcher.SingleInstance") {
		// find the running instance's hidden window by class name and ask it to
		// open. A FindWindow + custom message would be cleaner, but a tray click
		// is enough for the common "double launch" case.
		os.Exit(0)
	}
	cfg := loadConfig()
	cfg.ensureAutoStart()
	app := NewApp(cfg)
	app.Start()
}
