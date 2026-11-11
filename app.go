package main

import (
	"runtime"

	"github.com/dvygolov/yaswitcher/internal/gesture"
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

	// Alt+Tab mode: when the user presses Alt+Tab we swallow the key, show the
	// list and keep it open while Alt is held. Tab/Shift+Tab move the selection
	// and releasing Alt commits it. altHeld tracks the physical Alt state.
	altHeld  bool
	altMode  bool
	hookStop func()

	// ges is the pure hold-to-cycle state machine for the main hotkey.
	ges gesture.State

	quitting bool
}

const (
	umHotkey     = win.WM_APP + 10
	umOpen       = win.WM_APP + 11
	umCycle      = win.WM_APP + 12
	umClose      = win.WM_APP + 13
	umCommit     = win.WM_APP + 14
	umAltTabOpen = win.WM_APP + 15
	umAltTabStep = win.WM_APP + 16
)

func NewApp(cfg *Config) *App {
	a := &App{cfg: cfg, hotkeyID: 1}
	// Seed the gesture machine with whether the hotkey uses Alt.
	mods, _, _ := win.ParseHotkey(cfg.Hotkey)
	a.ges = gesture.New(mods&win.MOD_ALT != 0)
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
// registerHotkey parses the configured shortcut and prepares it to be handled
// through the low-level keyboard hook. The hook (rather than RegisterHotKey) is
// used so we can observe the Alt modifier being released, which drives the
// hold-to-cycle gesture. A best-effort RegisterHotKey is still attempted as a
// fallback for the case where the hook cannot be installed.
func (a *App) registerHotkey() {
	mods, vk, ok := win.ParseHotkey(a.cfg.Hotkey)
	if !ok {
		mods, vk = win.MOD_ALT, win.VK_SPACE
	}
	a.hotkeyMods = mods
	a.hotkeyVK = vk
	a.hotkeyViaHook = true
	if a.hookStop == nil {
		a.hookStop = win.SetKeyFilter(a.keyFilter)
	}
	logf("hotkey %q handled via keyboard hook (mods=%#x vk=%#x)", a.cfg.Hotkey, mods, vk)
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
	a.hotkeyMods = mods
	a.hotkeyVK = vk
	a.hotkeyViaHook = true
	// Recreate the gesture machine so its wantAlt matches the new hotkey.
	a.ges = gesture.New(mods&win.MOD_ALT != 0)
	if a.hookStop == nil {
		a.hookStop = win.SetKeyFilter(a.keyFilter)
	}
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
	// The hook is always installed for the main hotkey; Alt+Tab mode just adds
	// behaviour to the same filter.
	if a.hookStop == nil {
		a.hookStop = win.SetKeyFilter(a.keyFilter)
	}
}

// keyFilter implements the Alt+Tab interception. Called for every key event.
func (a *App) keyFilter(vk uint32, down bool, mods uint32) bool {
	// IMPORTANT: this runs inside a WH_KEYBOARD_LL callback. Windows silently
	// removes a low-level hook whose callback takes too long (LowLevelHooksTimeout,
	// ~300ms), so it must stay cheap: only update state and post a message. All
	// heavy work happens on the message loop in wndProc.
	//
	// Division of labour:
	//   overlay CLOSED -> the hook detects the hotkey combo and opens it.
	//   overlay OPEN   -> the overlay window itself has focus and handles
	//                     Alt+Space (cycle), Alt release (commit) and Esc via its
	//                     own window messages. The hook then ignores the hotkey so
	//                     the key reaches the window and is not double handled.
	isAlt := vk == win.VK_MENU || vk == win.VK_LMENU || vk == win.VK_RMENU
	if isAlt {
		a.ges.SetAlt(down)
		if !down {
			if a.altHeld {
				a.altHeld = false
				return true // Alt+Tab commit handled in wndProc
			}
			// Only the hook commits when the overlay is closed (nothing to do);
			// when open, the overlay's WM_SYSKEYUP handles it.
			if !a.ges.OverlayOpen() {
				a.ges.AltReleased()
			}
			return false
		}
	}

	// Escape closes a hook-opened overlay even if it somehow lost focus.
	if a.ges.OverlayOpen() && vk == win.VK_ESCAPE && down {
		win.PostMessage(a.hwnd, umClose, 0, 0)
		a.ges.SetOpen(false)
		return true
	}

	// --- main hotkey (e.g. Alt+Space) ---
	if a.hotkeyViaHook && vk == a.hotkeyVK {
		const want = win.MOD_ALT | win.MOD_CONTROL | win.MOD_SHIFT
		matches := (mods & want) == (a.hotkeyMods & want)

		if a.ges.OverlayOpen() {
			// Overlay is open: let its window handle the key. Alt+Space arrives
			// there as WM_SYSKEYDOWN. Do not swallow, so it reaches the window.
			return false
		}
		if !down {
			return false
		}
		if matches {
			a.ges.SetAlt(a.hotkeyMods&win.MOD_ALT != 0 && mods&win.MOD_ALT != 0)
			if a.ges.KeyDown() == gesture.Open {
				win.PostMessage(a.hwnd, umOpen, 0, 0)
			}
			a.ges.KeyUp()
			return true
		}
		return false
	}

	// --- optional Alt+Tab replacement ---
	if a.cfg.AltTabMode && vk == win.VK_TAB {
		if down && mods&win.MOD_ALT != 0 {
			if !a.altHeld {
				a.altHeld = true
				a.altMode = true
				win.PostMessage(a.hwnd, umAltTabOpen, 0, uintptr(mods))
			} else {
				win.PostMessage(a.hwnd, umAltTabStep, 0, uintptr(mods))
			}
			return true
		}
		if !down && a.altHeld {
			return true
		}
	}
	return false
}

// handleOpen opens the overlay on the message loop.
func (a *App) handleOpen() {
	if a.overlay != nil && a.overlay.IsVisible() {
		return
	}
	a.showOverlay()
}

// handleCycle moves the selection one step down, wrapping.
func (a *App) handleCycle() {
	if a.overlay == nil {
		return
	}
	if a.overlay.sel >= a.overlay.RowCount()-1 {
		a.overlay.setSel(0)
	} else {
		a.overlay.moveSel(1)
	}
}

// cycleStep advances the hold gesture by one step (called from the overlay's
// own WM_SYSKEYDOWN while Alt is held). The matching key-up clears the latch.
func (a *App) cycleStep() {
	a.ges.SetAlt(true)
	switch a.ges.KeyDown() {
	case gesture.Cycle:
		a.handleCycle()
	case gesture.Open:
		a.handleOpen()
	}
}

// releaseHotkeyKey clears the press latch on the main key release.
func (a *App) releaseHotkeyKey() { a.ges.KeyUp() }

// holdAltReleased commits the hold gesture when the overlay handles Alt release.
func (a *App) holdAltReleased() {
	a.ges.SetAlt(false)
	switch a.ges.AltReleased() {
	case gesture.Commit:
		a.commitHoldMode()
	}
}

// inHoldGesture reports whether a hold-to-cycle gesture is in progress.
func (a *App) inHoldGesture() bool { return a.ges.Cycling() && a.ges.AltHeld() }

// altShiftHeld reports whether Shift is down (for Alt+Tab direction).
func (a *App) altShiftHeld() bool { return win.Modifiers()&win.MOD_SHIFT != 0 }

// commitHoldMode switches to the currently selected window and closes.
func (a *App) commitHoldMode() {
	if a.overlay == nil || !a.overlay.IsVisible() {
		a.ges.SetOpen(false)
		return
	}
	a.ges.SetOpen(false)
	a.overlay.activateSelected()
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
		a.ges.SetOpen(true)
	}
}

// cancelOverlay hides the panel without switching.
func (a *App) cancelOverlay() {
	if a.overlay != nil {
		a.overlay.Hide()
	}
	a.ges.SetOpen(false)
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
	case umOpen:
		a.handleOpen()
		return 0
	case umCycle:
		a.handleCycle()
		return 0
	case umClose:
		a.cancelOverlay()
		return 0
	case umCommit:
		a.commitHoldMode()
		return 0
	case umAltTabOpen:
		a.showOverlay()
		if wp&uintptr(win.MOD_SHIFT) != 0 && a.overlay != nil {
			// not used; shift handled via message param
		}
		if a.overlay != nil && uint32(lp)&win.MOD_SHIFT != 0 {
			a.overlay.moveSel(-1)
		}
		return 0
	case umAltTabStep:
		if a.overlay != nil {
			if uint32(lp)&win.MOD_SHIFT != 0 {
				a.overlay.moveSel(-1)
			} else {
				a.overlay.moveSel(1)
			}
		}
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
