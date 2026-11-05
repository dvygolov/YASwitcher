//go:build windows

package win

import (
	"syscall"
	"unsafe"
)

// Monitor describes a display in virtual-screen coordinates.
type Monitor struct {
	Handle  uintptr
	Rect    RECT
	Work    RECT
	Primary bool
}

func monitorInfo(h uintptr) Monitor {
	mi := MONITORINFO{CbSize: uint32(unsafe.Sizeof(MONITORINFO{}))}
	pGetMonitorInfoW.Call(h, uintptr(unsafe.Pointer(&mi)))
	return Monitor{Handle: h, Rect: mi.RcMonitor, Work: mi.RcWork, Primary: mi.DwFlags&1 != 0}
}

// Monitors enumerates all display monitors.
func Monitors() []Monitor {
	out := make([]Monitor, 0, 4)
	cb := syscall.NewCallback(func(hMon uintptr, _ uintptr, _ uintptr, _ uintptr) uintptr {
		out = append(out, monitorInfo(hMon))
		return 1
	})
	pEnumDisplayMonitors.Call(0, 0, cb, 0)
	return out
}

// MonitorAt returns the monitor containing a screen point (nearest on failure).
func MonitorAt(p POINT) Monitor {
	h := monitorAtPoint(p)
	if h == 0 {
		return Monitor{}
	}
	return monitorInfo(h)
}

func monitorAt(h HWND) uintptr {
	r, _, _ := pMonitorFromWindow.Call(uintptr(h), MONITOR_DEFAULTTONEAREST)
	return r
}

// MonitorFromPoint returns the monitor containing pt.
func MonitorFromPoint(p POINT) Monitor {
	h := monitorAtPoint(p)
	if h == 0 {
		return Monitor{}
	}
	return monitorInfo(h)
}

// MonitorFromWindow returns the monitor a window is on.
func MonitorFromWindow(h HWND) Monitor {
	mh := monitorAt(h)
	if mh == 0 {
		return Monitor{}
	}
	return monitorInfo(mh)
}

// VirtualScreen returns the bounding box covering all monitors.
func VirtualScreen() RECT {
	x := GetSystemMetrics(SM_XVIRTUALSCREEN)
	y := GetSystemMetrics(SM_YVIRTUALSCREEN)
	w := GetSystemMetrics(SM_CXVIRTUALSCREEN)
	h := GetSystemMetrics(SM_CYVIRTUALSCREEN)
	return RECT{Left: x, Top: y, Right: x + w, Bottom: y + h}
}

// DpiForPoint returns the effective DPI (96 = 100%) of the monitor at p.
func DpiForPoint(p POINT) uint32 {
	mh := monitorAtPoint(p)
	if mh != 0 && pGetDpiForMonitor.Find() == nil {
		var x, y uint32
		if r, _, _ := pGetDpiForMonitor.Call(mh, 0, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y))); r == 0 && x >= 96 && x <= 480 {
			return x
		}
	}
	if pGetDpiForSystem.Find() == nil {
		if r, _, _ := pGetDpiForSystem.Call(); r >= 96 && r <= 480 {
			return uint32(r)
		}
	}
	return 96
}

// DpiForWindow returns the effective DPI of the monitor a window is on.
func DpiForWindow(h HWND) uint32 {
	if h != 0 && pGetDpiForWindow.Find() == nil {
		if r, _, _ := pGetDpiForWindow.Call(uintptr(h)); r >= 96 && r <= 480 {
			return uint32(r)
		}
	}
	return DpiForPoint(GetCursorPos())
}

// Scale converts a 96-DPI design value to physical pixels for the given DPI.
func Scale(v int32, dpi uint32) int32 {
	if dpi == 0 {
		dpi = 96
	}
	return int32((int64(v)*int64(dpi) + 48) / 96)
}

// monitorAtPoint returns the handle of the monitor containing pt.
func monitorAtPoint(p POINT) uintptr {
	// POINT is packed into a single 64-bit value (low word x, high word y).
	packed := uintptr(uint32(p.X)) | uintptr(uint32(p.Y))<<32
	r, _, _ := pMonitorFromPoint.Call(packed, MONITOR_DEFAULTTONEAREST)
	return r
}
