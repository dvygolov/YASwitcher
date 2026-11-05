package win

import (
	"syscall"
	"unsafe"
)

// WH_KEYBOARD_LL lets us see key presses before the shell does. This is how the
// Alt+Tab interception works: RegisterHotKey cannot own Alt+Tab, but a low-level
// hook can swallow the key so Explorer's switcher never appears.

const (
	WH_KEYBOARD_LL  = 13
	HC_ACTION       = 0
	LLKHF_INJECTED  = 0x10
	WM_KEYDOWN_H    = 0x0100
	WM_KEYUP_H      = 0x0101
	WM_SYSKEYDOWN_H = 0x0104
	WM_SYSKEYUP_H   = 0x0105
)

// KBDLLHOOKSTRUCT is the structure passed to a WH_KEYBOARD_LL callback.
type KBDLLHOOKSTRUCT struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

// KeyFilter decides whether a key event is swallowed. It is called for every
// key down/up with the tracked modifier state. Return true to swallow.
type KeyFilter func(vk uint32, down bool, mods uint32) bool

var (
	keyHookProc = syscall.NewCallback(keyboardHookProc)
	keyFilter   KeyFilter
	keyHookH    uintptr
	trackCtrl   bool
	trackAlt    bool
	trackShift  bool
	trackWin    bool
	trackSticky bool
)

// logHook is a tiny seam so the app can log hook installation without the win
// package depending on the app's logger.
var logHook = func(format string, args ...any) {}

// SetLogHook installs a logging sink for hook diagnostics.
func SetLogHook(fn func(string, ...any)) {
	if fn != nil {
		logHook = fn
	}
}

// SetKeyFilter installs a filter and returns a function to remove it. The hook
// stays installed as long as at least one filter is set; we keep it simple with
// a single filter since the app only needs one.
func SetKeyFilter(f KeyFilter) func() {
	keyFilter = f
	seedModifiers()
	if keyHookH == 0 {
		mod := uintptr(GetModuleHandle())
		keyHookH, _, _ = pSetWindowsHookExW.Call(WH_KEYBOARD_LL, keyHookProc, mod, 0)
		logHook("SetWindowsHookEx h=%d mod=%d", keyHookH, mod)
	}
	return func() {
		keyFilter = nil
		if keyHookH != 0 {
			pUnhookWindowsHookEx.Call(keyHookH)
			keyHookH = 0
		}
	}
}

func keyboardHookProc(nCode int32, wparam, lparam uintptr) uintptr {
	if nCode == HC_ACTION && keyFilter != nil {
		kb := (*KBDLLHOOKSTRUCT)(unsafe.Pointer(lparam))
		down := wparam == WM_KEYDOWN_H || wparam == WM_SYSKEYDOWN_H
		trackModifier(kb.VkCode, down)
		mods := trackedModifiers() | Modifiers()
		if keyFilter(kb.VkCode, down, mods) {
			return 1 // swallowed
		}
	}
	r, _, _ := pCallNextHookEx.Call(keyHookH, uintptr(nCode), wparam, lparam)
	return r
}

func trackModifier(vk uint32, down bool) {
	switch vk {
	case VK_CONTROL, VK_LCONTROL, VK_RCONTROL:
		trackCtrl = down
	case VK_MENU, VK_LMENU, VK_RMENU:
		trackAlt = down
	case VK_SHIFT, VK_LSHIFT, VK_RSHIFT:
		trackShift = down
	case VK_LWIN, VK_RWIN:
		trackWin = down
	}
}

// trackedModifiers builds a RegisterHotKey-style mask from our own tracking,
// which is more accurate than GetAsyncKeyState during a hook callback.
func trackedModifiers() uint32 {
	var m uint32
	if trackCtrl {
		m |= MOD_CONTROL
	}
	if trackAlt {
		m |= MOD_ALT
	}
	if trackShift {
		m |= MOD_SHIFT
	}
	if trackWin {
		m |= MOD_WIN
	}
	return m
}

// seedModifiers initialises tracking from the current physical key state, so a
// modifier already held when the hook starts is not missed.
func seedModifiers() {
	trackModifier(VK_LCONTROL, keyDown(VK_CONTROL))
	trackModifier(VK_LMENU, keyDown(VK_MENU))
	trackModifier(VK_LSHIFT, keyDown(VK_SHIFT))
	trackModifier(VK_LWIN, keyDown(VK_LWIN))
}
