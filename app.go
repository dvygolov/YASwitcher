package main

import (
	"runtime"

	"github.com/dvygolov/yaswitcher/internal/win"
)

// App owns the hidden message window (hotkey + tray target), the overlay, the
// config and the switching state. Everything runs on one thread: the Win32
// message loop started from main.
type App struct {
	hwnd    win.HWND
	cfg     *Config
	overlay *Overlay
	tray    *win.Tray

	hotkeyID int32
	hotkeyOn bool
	// When RegisterHotKey cannot claim the configured shortcut (e.g. Alt+Space
	// is reserved by the system), we fall back to matching it in the keyboard
	// hook. hotkeyMods/hotkeyVK hold the parsed shortcut in that case.
	hotkeyViaHook bool
	hotkeyMods    uint32
	hotkeyVK      uint32
	hotkeyDown    bool

	// Alt+Tab mode: when the user presses Alt+Tab we swallow the key, show the
	// list and keep it open while Alt is held. Tab/Shift+Tab move the selection
	// and releasing Alt commits it. altHeld tracks the physical Alt state.
	altHeld  bool
	altMode  bool
	hookStop func()

	quitting bool
}

const (
	umHotkey = win.WM_APP + 10
)

func NewApp(cfg *Config) *App {
	a := &App{cfg: cfg, hotkeyID: 1}
	return a
}

// Start creates the hidden window, tray icon, hotkey and keyboard hook, then
// runs the message loop. Blocks until the app quits.
func (a *App) Start() {
	// Win32 windows, hooks and hotkeys are bound to the OS thread that created
	// them, so pin this goroutine to one thread for the whole message loop.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	win.SetProcessDPIAware()
	win.SetLogHook(logf)
	win.RegisterClass(className, 0)

	param := win.NewHandler(a.wndProc)
	a.hwnd = win.CreateWindowEx(0, className, appTitle,
		0, 0, 0, 1, 1, 0, 0, param)
	logf("start: hwnd=%#x exe=%s", uintptr(a.hwnd), win.ExePath())
	if a.hwnd == 0 {
		win.MsgBox("YASwitcher could not create its window.", appTitle, 0x10)
		return
	}

	a.overlay = NewOverlay(a)
	a.setupTray()
	a.registerHotkey()
	a.installHook()
	logf("start: tray=%v hotkey=%v altTab=%v", a.tray != nil, a.hotkeyOn, a.cfg.AltTabMode)

	var msg win.MSG
	for {
		r := win.GetMessage(&msg, 0, 0, 0)
		if r <= 0 {
			break
		}
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}
}

func (a *App) quit() {
	if a.quitting {
		return
	}
	a.quitting = true
	a.unregisterHotkey()
	if a.hookStop != nil {
		a.hookStop()
	}
	if a.tray != nil {
		a.tray.Remove()
	}
	win.PostQuitMessage(0)
}

// setupTray registers the notification icon with our app icon.
func (a *App) setupTray() {
	icon := win.ExeIcon(win.ExePath())
	if icon == 0 {
		// fall back to the icon embedded in our own module (resource #1), which
		// works even when ExtractIconEx is picky; keeps the tray non-blank.
		icon = win.LoadAppIcon()
	}
	mo, mv := hotkeyMods(a.cfg.Hotkey)
	a.tray = win.NewTray(a.hwnd, icon, "YASwitcher — "+win.HotkeyString(mo, mv), win.WM_TRAYICON)
	a.tray.Add()
}

// registerHotkey binds the configured hotkey. If the system already owns it
// (Alt+Space is the window menu, errno 1408), the low-level keyboard hook is
// used instead so the shortcut still works.
func (a *App) registerHotkey() {
	mods, vk, ok := win.ParseHotkey(a.cfg.Hotkey)
	if !ok {
		mods, vk = win.MOD_ALT, win.VK_SPACE
	}
	for attempt := 0; attempt < 5; attempt++ {
		if ok2, errno := win.RegisterHotKey(a.hwnd, a.hotkeyID, mods|win.MOD_NOREPEAT, vk); ok2 {
			a.hotkeyOn = true
			logf("hotkey %q registered (attempt %d)", a.cfg.Hotkey, attempt+1)
			return
		} else {
			logf("hotkey %q attempt %d failed: %v", a.cfg.Hotkey, attempt+1, errno)
		}
		win.Sleep(150)
	}
	logf("hotkey %q unavailable, using keyboard hook", a.cfg.Hotkey)
	a.hotkeyViaHook = true
	a.hotkeyMods = mods
	a.hotkeyVK = vk
}

func (a *App) unregisterHotkey() {
	if a.hotkeyOn {
		win.UnregisterHotKey(a.hwnd, a.hotkeyID)
		a.hotkeyOn = false
	}
}

// reRegisterHotkey applies a new hotkey string from the tray menu.
func (a *App) reRegisterHotkey(hotkey string) bool {
	mods, vk, ok := win.ParseHotkey(hotkey)
	if !ok {
		return false
	}
	a.unregisterHotkey()
	if ok2, _ := win.RegisterHotKey(a.hwnd, a.hotkeyID, mods|win.MOD_NOREPEAT, vk); !ok2 {
		return false
	}
	a.hotkeyOn = true
	a.cfg.Hotkey = hotkey
	a.cfg.save()
	if a.tray != nil {
		a.tray.SetTip("YASwitcher — " + win.HotkeyString(mods, vk))
	}
	return true
}

// installHook installs the low-level keyboard hook that drives Alt+Tab mode.
// The hook is only active when cfg.AltTabMode is on.
func (a *App) installHook() {
	if !a.cfg.AltTabMode && !a.hotkeyViaHook {
		return
	}
	a.hookStop = win.SetKeyFilter(a.keyFilter)
}

// keyFilter implements the Alt+Tab interception. Called for every key event.
func (a *App) keyFilter(vk uint32, down bool, mods uint32) bool {
	// Configured shortcut handled through the hook (RegisterHotKey could not
	// claim it, e.g. Alt+Space). Fire on key down when the modifiers match; the
	// matching key-up always clears the latch so a missed event cannot wedge it.
	if a.hotkeyViaHook && vk == a.hotkeyVK {
		if !down {
			if a.hotkeyDown {
				a.hotkeyDown = false
				return true
			}
			return false
		}
		const want = win.MOD_ALT | win.MOD_CONTROL | win.MOD_SHIFT
		if (mods & want) == (a.hotkeyMods & want) {
			if !a.hotkeyDown {
				a.hotkeyDown = true
				win.PostMessage(a.hwnd, umHotkey, 0, 0)
			}
			return true
		}
		return false
	}
	// Safety: if the key went up while modifiers changed, still clear the latch.
	if a.hotkeyViaHook && !down && a.hotkeyDown {
		a.hotkeyDown = false
	}

	if a.cfg.AltTabMode && vk == win.VK_TAB {
		if down && mods&win.MOD_ALT != 0 {
			// first press opens; further presses cycle
			if !a.altHeld {
				a.altHeld = true
				a.altMode = true
				a.showOverlay()
				if mods&win.MOD_SHIFT != 0 {
					a.overlay.moveSel(-1)
				}
			} else {
				if mods&win.MOD_SHIFT != 0 {
					a.overlay.moveSel(-1)
				} else {
					a.overlay.moveSel(1)
				}
			}
			return true
		}
		if !down && a.altHeld {
			return true
		}
	}
	if vk == win.VK_MENU || vk == win.VK_LMENU || vk == win.VK_RMENU {
		if !down && a.altHeld {
			// Alt released: commit the selection
			a.altHeld = false
			if a.overlay != nil && a.overlay.IsVisible() {
				a.commitAltMode()
			}
			return true
		}
	}
	if vk == win.VK_ESCAPE && down && a.altHeld {
		a.altHeld = false
		a.cancelOverlay()
		return true
	}
	return false
}

// commitAltMode switches to the selected window and closes the overlay.
func (a *App) commitAltMode() {
	if a.overlay == nil {
		return
	}
	if a.overlay.sel >= 0 && a.overlay.sel < len(a.overlay.rows) {
		w := a.overlay.rows[a.overlay.sel].Win
		a.switchTo(w)
	} else {
		a.cancelOverlay()
	}
}

// showOverlay opens the switcher panel.
func (a *App) showOverlay() {
	if a.overlay != nil {
		a.overlay.Show()
	}
}

// cancelOverlay hides the panel without switching.
func (a *App) cancelOverlay() {
	if a.overlay != nil {
		a.overlay.Hide()
	}
}

// switchTo focuses the chosen window. If it is minimized it is restored first.
func (a *App) switchTo(w win.WindowInfo) {
	a.cancelOverlay()
	h := w.HWND
	if h == 0 || !h.IsWindow() {
		return
	}
	if h.IsIconic() {
		h.Show(win.SW_RESTORE)
	}
	h.SetForeground()
}

// toggleFromHotkey opens the overlay or closes it if already open.
func (a *App) toggleFromHotkey() {
	if a.overlay != nil && a.overlay.IsVisible() {
		a.cancelOverlay()
		return
	}
	a.showOverlay()
}

func (a *App) wndProc(h win.HWND, msg uint32, wp, lp uintptr) uintptr {
	switch msg {
	case win.WM_HOTKEY:
		a.toggleFromHotkey()
		return 0
	case umHotkey:
		a.toggleFromHotkey()
		return 0
	case win.WM_TRAYICON:
		switch uint32(lp) & 0xFFFF {
		case win.WM_LBUTTONUP, win.WM_LBUTTONDBLCLK:
			a.toggleFromHotkey()
		case win.WM_RBUTTONUP, win.WM_CONTEXTMENU:
			a.showTrayMenu()
		}
		return 0
	case win.WM_DESTROY:
		a.quit()
		return 0
	}
	return win.DefWindowProc(h, msg, wp, lp)
}

// ---------------------------------------------------------------------------
// tray menu
// ---------------------------------------------------------------------------

const (
	menuOpen = 1 + iota
	menuAutoStart
	menuAltTab
	menuDark
	menuMonitor
	menuAbout
	menuExit
)

func (a *App) showTrayMenu() {
	items := []win.MenuItem{
		{ID: menuOpen, Label: "Open switcher  (" + a.cfg.Hotkey + ")"},
		{Sep: true},
		{ID: menuAutoStart, Label: "Start with Windows", Check: a.cfg.AutoStart},
		{ID: menuAltTab, Label: "Replace Alt+Tab", Check: a.cfg.AltTabMode},
		{ID: menuDark, Label: "Dark theme", Check: a.cfg.DarkTheme},
		{ID: menuMonitor, Label: "Show monitor indicator", Check: a.cfg.ShowMonitor},
		{Sep: true},
		{ID: menuAbout, Label: "About"},
		{ID: menuExit, Label: "Exit"},
	}
	hm := win.BuildMenu(items)
	defer win.DestroyMenu(hm)
	pt := win.GetCursorPos()
	win.SetForegroundWindow(a.hwnd)
	switch win.TrackMenu(hm, pt) {
	case menuOpen:
		a.showOverlay()
	case menuAutoStart:
		a.cfg.AutoStart = !a.cfg.AutoStart
		a.cfg.save()
		a.cfg.ensureAutoStart()
	case menuAltTab:
		a.cfg.AltTabMode = !a.cfg.AltTabMode
		a.cfg.save()
		a.toggleAltTabMode()
	case menuDark:
		a.cfg.DarkTheme = !a.cfg.DarkTheme
		a.cfg.save()
	case menuMonitor:
		a.cfg.ShowMonitor = !a.cfg.ShowMonitor
		a.cfg.save()
	case menuAbout:
		a.showAbout()
	case menuExit:
		a.quit()
	}
}

func (a *App) toggleAltTabMode() {
	if a.cfg.AltTabMode {
		if a.hookStop == nil {
			a.hookStop = win.SetKeyFilter(a.keyFilter)
		}
	} else {
		if a.hookStop != nil {
			a.hookStop()
			a.hookStop = nil
		}
	}
}

func (a *App) showAbout() {
	text := appTitle + " " + version + "\n\n" +
		"A lightweight window switcher for Windows.\n" +
		"Hotkey: " + a.cfg.Hotkey + "\n\n" +
		"MIT License"
	win.MsgBox(text, "About YASwitcher", 0x00000040|0x00000000)
}

// hotkeyMods is a small helper for the tray tooltip.
func hotkeyMods(s string) (uint32, uint32) {
	mods, vk, ok := win.ParseHotkey(s)
	if !ok {
		return win.MOD_ALT, win.VK_SPACE
	}
	return mods, vk
}
