//go:build windows

package win

import (
	"syscall"
)

// RegisterHotKey wraps the Win32 call. Returns ok and the last error code.
func RegisterHotKey(h HWND, id int32, mods, vk uint32) (bool, syscall.Errno) {
	r, _, err := pRegisterHotKey.Call(uintptr(h), uintptr(id), uintptr(mods), uintptr(vk))
	en, _ := err.(syscall.Errno)
	return r != 0, en
}

func UnregisterHotKey(h HWND, id int32) {
	pUnregisterHotKey.Call(uintptr(h), uintptr(id))
}

// ModifierKeyDown reports whether the modifier is currently held.
func keyDown(vk uint32) bool {
	r, _, _ := pGetAsyncKeyState.Call(uintptr(vk))
	return int16(r&0xFFFF) < 0
}

var pGetAsyncKeyState = dllUser32.NewProc("GetAsyncKeyState")

// Modifiers returns the pressed modifiers in RegisterHotKey terms.
func Modifiers() uint32 {
	var m uint32
	if keyDown(VK_CONTROL) || keyDown(VK_LCONTROL) || keyDown(VK_RCONTROL) {
		m |= MOD_CONTROL
	}
	if keyDown(VK_MENU) || keyDown(VK_LMENU) || keyDown(VK_RMENU) {
		m |= MOD_ALT
	}
	if keyDown(VK_SHIFT) || keyDown(VK_LSHIFT) || keyDown(VK_RSHIFT) {
		m |= MOD_SHIFT
	}
	if keyDown(VK_LWIN) || keyDown(VK_RWIN) {
		m |= MOD_WIN
	}
	return m
}

// VKName returns a display name for a virtual key ("Alt", "Space", "Tab", "A",
// "F4", ...). Unknown keys fall back to "0xNN".
func VKName(vk uint32) string {
	if s, ok := vkNames[vk]; ok {
		return s
	}
	if vk >= 0x30 && vk <= 0x39 {
		return string(rune(vk))
	}
	if vk >= 0x41 && vk <= 0x5A {
		return string(rune(vk))
	}
	if vk >= VK_F1 && vk <= VK_F24 {
		return "F" + itoa(int(vk-VK_F1)+1)
	}
	return "0x" + hex2(vk)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func hex2(v uint32) string {
	const digits = "0123456789ABCDEF"
	return string([]byte{digits[(v>>4)&0xF], digits[v&0xF]})
}

var vkNames = map[uint32]string{
	VK_BACK: "Backspace", VK_TAB: "Tab", VK_RETURN: "Enter", VK_ESCAPE: "Esc",
	VK_SPACE: "Space", VK_PRIOR: "PgUp", VK_NEXT: "PgDn", VK_END: "End",
	VK_HOME: "Home", VK_LEFT: "Left", VK_UP: "Up", VK_RIGHT: "Right",
	VK_DOWN: "Down", VK_INSERT: "Ins", VK_DELETE: "Del", VK_SNAPSHOT: "PrintScreen",
	VK_OEM_1: ";", VK_OEM_PLUS: "+", VK_OEM_COMMA: ",", VK_OEM_MINUS: "-",
	VK_OEM_PERIOD: ".", VK_OEM_2: "/", VK_OEM_3: "`", VK_OEM_4: "[",
	VK_OEM_5: "\\", VK_OEM_6: "]", VK_OEM_7: "'",
}

// ModsName renders a modifier mask as "Ctrl+Alt+Shift+Win".
func ModsName(mods uint32) string {
	var parts []string
	if mods&MOD_CONTROL != 0 {
		parts = append(parts, "Ctrl")
	}
	if mods&MOD_ALT != 0 {
		parts = append(parts, "Alt")
	}
	if mods&MOD_SHIFT != 0 {
		parts = append(parts, "Shift")
	}
	if mods&MOD_WIN != 0 {
		parts = append(parts, "Win")
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "+"
		}
		out += p
	}
	return out
}

// HotkeyString formats a hotkey for display.
func HotkeyString(mods, vk uint32) string {
	m := ModsName(mods)
	k := VKName(vk)
	if m == "" {
		return k
	}
	return m + "+" + k
}

// ParseHotkey accepts "Ctrl+Alt+S", "Alt+Space", "F4", "PrintScreen" and returns
// the modifier mask and virtual key. Unknown tokens make ok=false.
func ParseHotkey(s string) (mods, vk uint32, ok bool) {
	toks := splitPlus(s)
	for _, t := range toks {
		switch lower(t) {
		case "ctrl", "control":
			mods |= MOD_CONTROL
		case "alt":
			mods |= MOD_ALT
		case "shift":
			mods |= MOD_SHIFT
		case "win", "super":
			mods |= MOD_WIN
		default:
			v, found := nameToVK[lower(t)]
			if !found {
				return 0, 0, false
			}
			vk = v
		}
	}
	if vk == 0 {
		return 0, 0, false
	}
	return mods, vk, true
}

func splitPlus(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == '+' {
			out = append(out, cur)
			cur = ""
		} else {
			cur += string(r)
		}
	}
	out = append(out, cur)
	return out
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

var nameToVK = map[string]uint32{
	"backspace": VK_BACK, "tab": VK_TAB, "enter": VK_RETURN, "return": VK_RETURN,
	"esc": VK_ESCAPE, "escape": VK_ESCAPE, "space": VK_SPACE, "pgup": VK_PRIOR,
	"pageup": VK_PRIOR, "pgdn": VK_NEXT, "pagedown": VK_NEXT, "end": VK_END,
	"home": VK_HOME, "left": VK_LEFT, "up": VK_UP, "right": VK_RIGHT, "down": VK_DOWN,
	"ins": VK_INSERT, "insert": VK_INSERT, "del": VK_DELETE, "delete": VK_DELETE,
	"printscreen": VK_SNAPSHOT, "prtsc": VK_SNAPSHOT,
	"f1": VK_F1, "f2": VK_F1 + 1, "f3": VK_F1 + 2, "f4": VK_F1 + 3, "f5": VK_F1 + 4,
	"f6": VK_F1 + 5, "f7": VK_F1 + 6, "f8": VK_F1 + 7, "f9": VK_F1 + 8,
	"f10": VK_F1 + 9, "f11": VK_F1 + 10, "f12": VK_F1 + 11,
}

func init() {
	for c := 'A'; c <= 'Z'; c++ {
		nameToVK[stringsLower(string(c))] = uint32(c)
	}
	for c := '0'; c <= '9'; c++ {
		nameToVK[string(c)] = uint32(c)
	}
}

func stringsLower(s string) string { return lower(s) }
