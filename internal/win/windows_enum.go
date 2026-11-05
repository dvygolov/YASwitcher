//go:build windows

package win

import (
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// WindowInfo is one entry of the Alt-Tab style window list.
type WindowInfo struct {
	HWND     HWND
	Title    string
	Process  string // exe name without .exe: "explorer", "chrome", "code"
	Path     string // full executable path
	PID      uint32
	Rect     RECT
	Icon     HICON
	iconDone bool
}

type enumCtx struct{ out []HWND }

var enumWindowsCB = syscall.NewCallback(func(hwnd HWND, lparam uintptr) uintptr {
	ctx := (*enumCtx)(unsafe.Pointer(lparam))
	ctx.out = append(ctx.out, hwnd)
	return 1
})

// TopLevelWindows enumerates top-level windows in Z order (top first).
func TopLevelWindows() []HWND {
	ctx := &enumCtx{out: make([]HWND, 0, 512)}
	pEnumWindows.Call(enumWindowsCB, uintptr(unsafe.Pointer(ctx)))
	return ctx.out
}

// IsAltTabWindow reports whether a window belongs in the switcher list.
func IsAltTabWindow(h HWND) bool { return altTabRejectReason(h) == "" }

// altTabRejectReason returns why a window is not an Alt-Tab window, or "" when
// it should be included. Mirrors Switcheroo's AppWindow.IsAltTabWindow.
func altTabRejectReason(h HWND) string {
	if h == 0 || !h.IsWindow() {
		return "no-window"
	}
	ex := h.ExStyle()
	style := h.Style()

	if ex&WS_EX_APPWINDOW != 0 {
		if h.IsCloaked() {
			return "appwindow-cloaked"
		}
		if h.Title() == "" {
			return "appwindow-notitle"
		}
		return ""
	}
	if !h.IsVisible() {
		return "invisible"
	}
	if h.Title() == "" {
		return "empty-title"
	}
	if ex&WS_EX_TOOLWINDOW != 0 || style&WS_EX_TOOLWINDOW != 0 {
		return "toolwindow"
	}
	if ex&WS_EX_NOACTIVATE != 0 {
		return "noactivate"
	}
	if owner := h.Owner(); owner != 0 && owner.IsVisible() {
		return "owned"
	}
	if h.Prop("ITaskList_Deleted") != 0 {
		return "tasklist-deleted"
	}
	cls := h.ClassName()
	if cls == "Windows.UI.Core.CoreWindow" {
		return "uwp-corewindow"
	}
	if cls == "ApplicationFrameWindow" {
		if cloakType := h.Prop("ApplicationViewCloakType"); cloakType == 1 {
			return "uwp-cloaked-appview"
		}
	}
	if h.IsCloaked() {
		return "cloaked"
	}
	return ""
}

// ListWindows returns the Alt-Tab style list, excluding the given process id.
func ListWindows(selfPID uint32) []WindowInfo {
	all := TopLevelWindows()
	out := make([]WindowInfo, 0, len(all))
	for _, h := range all {
		if altTabRejectReason(h) != "" {
			continue
		}
		pid := h.PID()
		if pid == selfPID {
			continue
		}
		info := WindowInfo{
			HWND:  h,
			Title: strings.TrimSpace(h.Title()),
			PID:   pid,
			Rect:  h.Rect(),
		}
		info.Path = ProcessPath(pid)
		info.Process = processName(info.Path)
		out = append(out, info)
	}
	return out
}

// ProcessImagePath returns the full path of the exe behind a window.
func ProcessPath(pid uint32) string {
	h, _, _ := pOpenProcess.Call(PROCESS_QUERY_LIMITED_INFORMATION, 0, uintptr(pid))
	if h == 0 {
		h, _, _ = pOpenProcess.Call(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ, 0, uintptr(pid))
	}
	if h == 0 {
		return ""
	}
	defer pCloseHandle.Call(h)
	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	r, _, _ := pQueryFullProcessImage.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r != 0 {
		return syscall.UTF16ToString(buf[:size])
	}
	n, _, _ := pGetModuleFileNameExW.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

// processName turns "C:\...\Code.exe" into "code".
func processName(path string) string {
	if path == "" {
		return ""
	}
	base := filepath.Base(path)
	base = strings.TrimSuffix(strings.ToLower(base), ".exe")
	return base
}

// SelfPID returns the process id of the running program.
func SelfPID() uint32 {
	r, _, _ := pGetCurrentProcessId.Call()
	return uint32(r)
}
