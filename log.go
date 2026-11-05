package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// logf writes a line to %TEMP%\YASwitcher.log when YASWITCHER_DEBUG is set.
// Otherwise it is a no-op so the release build stays silent and fast.
var debugLog = os.Getenv("YASWITCHER_DEBUG") != ""

func logf(format string, args ...any) {
	if !debugLog {
		return
	}
	path := filepath.Join(os.TempDir(), "YASwitcher.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
}
