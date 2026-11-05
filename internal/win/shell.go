//go:build windows

package win

import (
	"syscall"
	"unsafe"
)

// ---------------- tray icon ----------------

const (
	NIF_SHOWTIP = 0x80
)

type NOTIFYICONDATAW struct {
	CbSize           uint32
	HWnd             HWND
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            HICON
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         [16]byte
	HBalloonIcon     HICON
}

type MenuItem struct {
	ID    int
	Label string
	Sep   bool
	Check bool
	Gray  bool
}

// BuildMenu creates a popup menu from items. The menu must be Destroyed by the
// caller after TrackMenu.
func BuildMenu(items []MenuItem) HMENU {
	r, _, _ := pCreatePopupMenu.Call()
	hm := HMENU(r)
	for _, it := range items {
		if it.Sep {
			pAppendMenuW.Call(uintptr(hm), MF_SEPARATOR, 0, 0)
			continue
		}
		flags := uintptr(MF_STRING)
		if it.Check {
			flags |= MF_CHECKED
		}
		if it.Gray {
			flags |= MF_GRAYED
		}
		pAppendMenuW.Call(uintptr(hm), flags, uintptr(it.ID), uintptr(unsafe.Pointer(UTF16Ptr(it.Label))))
	}
	return hm
}

// TrackMenu shows a popup menu at pt and returns the chosen item id (0 if none).
func TrackMenu(m HMENU, pt POINT) int {
	r, _, _ := pTrackPopupMenu.Call(uintptr(m), TPM_RETURNCMD|TPM_RIGHTBUTTON|TPM_NONOTIFY,
		uintptr(pt.X), uintptr(pt.Y), 0, 0, 0)
	return int(r)
}

func DestroyMenu(m HMENU) { pDestroyMenu.Call(uintptr(m)) }

// ExePath returns the full path of the running executable.
func ExePath() string {
	buf := make([]uint16, 1024)
	r, _, _ := pGetModuleFileNameW.Call(0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:r])
}

// ShellOpen opens a path or URL with the default handler.
func ShellOpen(target string) {
	pShellExecuteW.Call(0, uintptr(unsafe.Pointer(UTF16Ptr("open"))),
		uintptr(unsafe.Pointer(UTF16Ptr(target))), 0, 0, SW_SHOWNORMAL)
}

var (
	advapiRegOpenKeyExW   = dllAdvapi32.NewProc("RegOpenKeyExW")
	advapiRegSetValueExW  = dllAdvapi32.NewProc("RegSetValueExW")
	advapiRegDeleteValueW = dllAdvapi32.NewProc("RegDeleteValueW")
)

const (
	HKEY_CURRENT_USER = 0x80000001
	KEY_SET_VALUE     = 0x0002
	KEY_QUERY_VALUE   = 0x0001
	REG_SZ            = 1
)

const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

func openRunKey() uintptr {
	var h uintptr
	r, _, _ := advapiRegOpenKeyExW.Call(HKEY_CURRENT_USER,
		uintptr(unsafe.Pointer(UTF16Ptr(runKeyPath))), 0, KEY_SET_VALUE, uintptr(unsafe.Pointer(&h)))
	if r != 0 {
		return 0
	}
	return h
}

// SetAutostart registers (or updates) a value in HKCU\...\Run.
func SetAutostart(name, exePath string) bool {
	h := openRunKey()
	if h == 0 {
		return false
	}
	defer pCloseHandle.Call(h)
	s := UTF16Ptr(exePath)
	r, _, _ := advapiRegSetValueExW.Call(h,
		uintptr(unsafe.Pointer(UTF16Ptr(name))), 0, REG_SZ,
		uintptr(unsafe.Pointer(s)), uintptr((len(utf16Runes(exePath))+1)*2))
	return r == 0
}

// RemoveAutostart deletes the Run value.
func RemoveAutostart(name string) {
	h := openRunKey()
	if h == 0 {
		return
	}
	defer pCloseHandle.Call(h)
	advapiRegDeleteValueW.Call(h, uintptr(unsafe.Pointer(UTF16Ptr(name))))
}

// CloseWindow posts WM_CLOSE to a window gracefully.
func CloseWindow(h HWND) {
	PostMessage(h, WM_CLOSE, 0, 0)
}

// Activate shows and focuses a window.
func Activate(h HWND) {
	if h == 0 {
		return
	}
	h.SetForeground()
}

// ---------------- tray icon ----------------

// Tray is a taskbar notification icon bound to a window and a callback message.
type Tray struct {
	data NOTIFYICONDATAW
	ok   bool
}

// NewTray prepares a tray icon. Call Add to show it.
func NewTray(hwnd HWND, icon HICON, tip string, cbMsg uint32) *Tray {
	t := &Tray{}
	t.data.CbSize = uint32(unsafe.Sizeof(t.data))
	t.data.HWnd = hwnd
	t.data.UID = 1
	t.data.UFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP | NIF_SHOWTIP
	t.data.UCallbackMessage = cbMsg
	t.data.HIcon = icon
	CopyUTF16(t.data.SzTip[:], tip)
	return t
}

// Add registers the icon. Returns ok and the last error number.
func (t *Tray) Add() (bool, uint32) {
	r, _, _ := pShellNotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&t.data)))
	t.ok = r != 0
	if t.ok {
		// NIM_SETVERSION so balloon clicks arrive as WM_CONTEXTMENU
		t.data.UVersion = 4
		pShellNotifyIconW.Call(NIM_SETVERSION, uintptr(unsafe.Pointer(&t.data)))
	}
	return t.ok, uint32(r)
}

// SetTip updates the hover tooltip.
func (t *Tray) SetTip(tip string) {
	CopyUTF16(t.data.SzTip[:], tip)
	pShellNotifyIconW.Call(NIM_MODIFY, uintptr(unsafe.Pointer(&t.data)))
}

// Remove deletes the icon from the notification area.
func (t *Tray) Remove() {
	pShellNotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&t.data)))
	t.ok = false
}
