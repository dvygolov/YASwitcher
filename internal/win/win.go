//go:build windows

package win

import (
	"syscall"
	"unsafe"
)

// WndProcFunc is a window procedure callback.
type WndProcFunc func(hwnd HWND, msg uint32, wp, lp uintptr) uintptr

type CREATESTRUCTW struct {
	LpCreateParams uintptr
	HInstance      uintptr
	HMENU          HMENU
	HWndParent     HWND
	Cy             int32
	Cx             int32
	Y              int32
	X              int32
	Style          int32
	LpszName       *uint16
	LpszClass      *uint16
	DwExStyle      uint32
}

const (
	GWLP_USERDATA = -21
	GWLP_WNDPROC  = -4
)

var (
	wndProcs   []WndProcFunc
	wndProcCB  = syscall.NewCallback(wndProcDispatcher)
	registered = map[string]bool{}
)

func wndProcDispatcher(hwnd HWND, msg uint32, wparam, lparam uintptr) uintptr {
	var id uintptr
	if msg == WM_NCCREATE {
		cs := (*CREATESTRUCTW)(unsafe.Pointer(lparam))
		id = cs.LpCreateParams
		SetWindowLongPtr(hwnd, GWLP_USERDATA, id)
	} else {
		id = GetWindowLongPtr(hwnd, GWLP_USERDATA)
	}
	if id > 0 && int(id) <= len(wndProcs) {
		return wndProcs[id-1](hwnd, msg, wparam, lparam)
	}
	return DefWindowProc(hwnd, msg, wparam, lparam)
}

// RegisterClass registers a window class once. All windows of that class share
// the dispatcher, which routes messages through GWLP_USERDATA.
func RegisterClass(className string, background uintptr) {
	if registered[className] {
		return
	}
	wc := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		Style:         0,
		LpfnWndProc:   wndProcCB,
		HInstance:     uintptr(GetModuleHandle()),
		HCursor:       LoadCursor(0, IDC_ARROW),
		HbrBackground: HBRUSH(background),
		LpszClassName: UTF16Ptr(className),
	}
	r, _, _ := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if r != 0 {
		registered[className] = true
	}
}

// NewHandler registers a message handler for one window instance and returns the
// id to pass as lpCreateParams to CreateWindowEx.
func NewHandler(fn WndProcFunc) uintptr {
	wndProcs = append(wndProcs, fn)
	return uintptr(len(wndProcs))
}

func GetModuleHandle() syscall.Handle {
	r, _, _ := pGetModuleHandleW.Call(0)
	return syscall.Handle(r)
}

func CreateWindowEx(exStyle uint32, className, title string, style uint32, x, y, w, h int32, parent HWND, hm HMENU, param uintptr) HWND {
	r, _, _ := pCreateWindowExW.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(UTF16Ptr(className))),
		uintptr(unsafe.Pointer(UTF16Ptr(title))),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent), uintptr(hm), uintptr(GetModuleHandle()),
		param,
	)
	return HWND(r)
}

func DefWindowProc(h HWND, msg uint32, wp, lp uintptr) uintptr {
	r, _, _ := pDefWindowProcW.Call(uintptr(h), uintptr(msg), wp, lp)
	return r
}

func (h HWND) Destroy() { pDestroyWindow.Call(uintptr(h)) }

func (h HWND) Show(cmd int) { pShowWindow.Call(uintptr(h), uintptr(cmd)) }

func (h HWND) IsWindow() bool {
	r, _, _ := pIsWindow.Call(uintptr(h))
	return r != 0
}

func (h HWND) Update() { pUpdateWindow.Call(uintptr(h)) }

func (h HWND) Invalidate(erase bool) {
	e := uintptr(0)
	if erase {
		e = 1
	}
	pInvalidateRect.Call(uintptr(h), 0, e)
}

func (h HWND) InvalidateRect(r *RECT, erase bool) {
	e := uintptr(0)
	if erase {
		e = 1
	}
	pInvalidateRect.Call(uintptr(h), uintptr(unsafe.Pointer(r)), e)
}

func (h HWND) SetPos(z HWND, x, y, w, hgt int32, flags uint32) {
	pSetWindowPos.Call(uintptr(h), uintptr(z), uintptr(x), uintptr(y), uintptr(w), uintptr(hgt), uintptr(flags))
}

func (h HWND) Rect() RECT {
	var r RECT
	pGetWindowRect.Call(uintptr(h), uintptr(unsafe.Pointer(&r)))
	return r
}

func (h HWND) ClientRect() RECT {
	var r RECT
	pGetClientRect.Call(uintptr(h), uintptr(unsafe.Pointer(&r)))
	return r
}

func (h HWND) ClassName() string {
	buf := make([]uint16, 256)
	n, _, _ := pGetClassNameW.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

func (h HWND) Title() string {
	n, _, _ := pGetWindowTextLengthW.Call(uintptr(h))
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	m, _, _ := pGetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if m == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:m])
}

func (h HWND) Style() uint32 {
	r, _, _ := pGetWindowLongPtrW.Call(uintptr(h), ^uintptr(15)) // GWL_STYLE = -16
	return uint32(r)
}

func (h HWND) ExStyle() uint32 {
	r, _, _ := pGetWindowLongPtrW.Call(uintptr(h), ^uintptr(19)) // GWL_EXSTYLE = -20
	return uint32(r)
}

func (h HWND) IsVisible() bool {
	r, _, _ := pIsWindowVisible.Call(uintptr(h))
	return r != 0
}

func (h HWND) IsCloaked() bool {
	var cloaked uint32
	ret, _, _ := pDwmGetWindowAttribute.Call(uintptr(h), DWMWA_CLOAKED, uintptr(unsafe.Pointer(&cloaked)), unsafe.Sizeof(cloaked))
	if ret != 0 {
		return false
	}
	return cloaked != 0
}

func (h HWND) PID() uint32 {
	var pid uint32
	pGetWindowThreadProcessId.Call(uintptr(h), uintptr(unsafe.Pointer(&pid)))
	return pid
}

func (h HWND) Owner() HWND {
	r, _, _ := pGetWindow.Call(uintptr(h), GW_OWNER)
	return HWND(r)
}

func (h HWND) RootOwner() HWND {
	r, _, _ := pGetAncestor.Call(uintptr(h), GA_ROOTOWNER)
	return HWND(r)
}

func (h HWND) LastActivePopup() HWND {
	r, _, _ := pGetLastActivePopup.Call(uintptr(h))
	return HWND(r)
}

func (h HWND) Prop(name string) uintptr {
	r, _, _ := pGetPropW.Call(uintptr(h), uintptr(unsafe.Pointer(UTF16Ptr(name))))
	return r
}

func GetForegroundWindow() HWND {
	r, _, _ := pGetForegroundWindow.Call()
	return HWND(r)
}

func (h HWND) SetForeground() {
	// Only the foreground process may call SetForegroundWindow freely. Attach to
	// the current foreground thread to borrow its right for this call.
	fg := GetForegroundWindow()
	fgThread, _, _ := pGetWindowThreadProcessId.Call(uintptr(fg), 0)
	selfThread, _, _ := pGetCurrentThreadId.Call()
	attached := false
	if fgThread != 0 && fgThread != selfThread {
		r, _, _ := pAttachThreadInput.Call(selfThread, fgThread, 1)
		attached = r != 0
		if attached {
			defer pAttachThreadInput.Call(selfThread, fgThread, 0)
		}
	}
	if h.IsIconic() {
		pShowWindowAsync.Call(uintptr(h), SW_RESTORE)
	}
	pBringWindowToTop.Call(uintptr(h))
	pSetForegroundWindow.Call(uintptr(h))
}

func (h HWND) IsIconic() bool {
	r, _, _ := pIsIconic.Call(uintptr(h))
	return r != 0
}

func SetWindowLongPtr(h HWND, index int32, val uintptr) uintptr {
	r, _, _ := pSetWindowLongPtrW.Call(uintptr(h), uintptr(index), val)
	return r
}

func GetWindowLongPtr(h HWND, index int32) uintptr {
	r, _, _ := pGetWindowLongPtrW.Call(uintptr(h), uintptr(index))
	return r
}

func MsgBox(text, caption string, flags uint32) int32 {
	r, _, _ := pMessageBoxW.Call(0, uintptr(unsafe.Pointer(UTF16Ptr(text))), uintptr(unsafe.Pointer(UTF16Ptr(caption))), uintptr(flags))
	return int32(r)
}

func GetMessage(msg *MSG, h HWND, min, max uint32) int32 {
	r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(msg)), uintptr(h), uintptr(min), uintptr(max))
	return int32(r)
}

func PeekMessage(msg *MSG, h HWND, min, max uint32, remove uint32) bool {
	r, _, _ := pPeekMessageW.Call(uintptr(unsafe.Pointer(msg)), uintptr(h), uintptr(min), uintptr(max), uintptr(remove))
	return r != 0
}

func TranslateMessage(msg *MSG) { pTranslateMessage.Call(uintptr(unsafe.Pointer(msg))) }
func DispatchMessage(msg *MSG)  { pDispatchMessageW.Call(uintptr(unsafe.Pointer(msg))) }
func PostQuitMessage(code int32) {
	pPostQuitMessage.Call(uintptr(code))
}
func PostMessage(h HWND, msg uint32, wp, lp uintptr) {
	pPostMessageW.Call(uintptr(h), uintptr(msg), wp, lp)
}
func SendMessage(h HWND, msg uint32, wp, lp uintptr) uintptr {
	r, _, _ := pSendMessageW.Call(uintptr(h), uintptr(msg), wp, lp)
	return r
}

// SendMessageTimeout sends a message with a timeout, returning the result value.
func SendMessageTimeout(h HWND, msg uint32, wp, lp uintptr, flags, timeout uint32) (uintptr, bool) {
	var out uintptr
	r, _, _ := pSendMessageTimeoutW.Call(uintptr(h), uintptr(msg), wp, lp, uintptr(flags), uintptr(timeout), uintptr(unsafe.Pointer(&out)))
	return out, r != 0
}

func GetDC(h HWND) HDC {
	r, _, _ := pGetDC.Call(uintptr(h))
	return HDC(r)
}

func ReleaseDC(h HWND, dc HDC) { pReleaseDC.Call(uintptr(h), uintptr(dc)) }

func BeginPaint(h HWND, ps *PAINTSTRUCT) HDC {
	r, _, _ := pBeginPaint.Call(uintptr(h), uintptr(unsafe.Pointer(ps)))
	return HDC(r)
}

func EndPaint(h HWND, ps *PAINTSTRUCT) { pEndPaint.Call(uintptr(h), uintptr(unsafe.Pointer(ps))) }

func SetTimer(h HWND, id uintptr, ms uint32, cb uintptr) uintptr {
	r, _, _ := pSetTimer.Call(uintptr(h), id, uintptr(ms), cb)
	return r
}

func KillTimer(h HWND, id uintptr) { pKillTimer.Call(uintptr(h), id) }

func SetProcessDPIAware() {
	if pSetProcessDpiCtx.Find() == nil {
		// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4
		if r, _, _ := pSetProcessDpiCtx.Call(^uintptr(3)); r != 0 {
			return
		}
	}
	pSetProcessDPIAware.Call()
}

func GetSystemMetrics(index int32) int32 {
	r, _, _ := pGetSystemMetrics.Call(uintptr(index))
	return int32(r)
}

func GetCursorPos() POINT {
	var p POINT
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return p
}

func LoadCursor(h uintptr, id uintptr) uintptr {
	r, _, _ := pLoadCursorW.Call(h, id)
	return r
}

func SetCursor(c uintptr) uintptr {
	r, _, _ := pSetCursor.Call(c)
	return r
}

func Sleep(ms uint32) { pSleep.Call(uintptr(ms)) }

func SetForegroundWindow(h HWND) { h.SetForeground() }

func ShowWindow(h HWND, cmd int32) { h.Show(int(cmd)) }

func IsWindowVisible(h HWND) bool { return h.IsVisible() }

func IsWindow(h HWND) bool { return h.IsWindow() }

func SetWindowRgn(h HWND, rgn uintptr, redraw bool) {
	rd := uintptr(0)
	if redraw {
		rd = 1
	}
	pSetWindowRgn.Call(uintptr(h), rgn, rd)
}

func CreateRoundRectRgn(x1, y1, x2, y2, w, h int32) uintptr {
	r, _, _ := pCreateRoundRectRgn.Call(uintptr(x1), uintptr(y1), uintptr(x2), uintptr(y2), uintptr(w), uintptr(h))
	return r
}

func ClientToScreen(h HWND, p *POINT) {
	pClientToScreen.Call(uintptr(h), uintptr(unsafe.Pointer(p)))
}

func ScreenToClient(h HWND, p *POINT) {
	pScreenToClient.Call(uintptr(h), uintptr(unsafe.Pointer(p)))
}

// mutexHandle keeps the single instance mutex alive for the process lifetime.
var mutexHandle uintptr

// SingleInstance returns true if this is the only running copy. The mutex is
// held for the lifetime of the process.
func SingleInstance(name string) bool {
	h, _, err := pCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(UTF16Ptr(name))))
	if h == 0 {
		return false
	}
	mutexHandle = h
	// ERROR_ALREADY_EXISTS is surfaced as the last error of CreateMutexW.
	return err != syscall.Errno(ERROR_ALREADY_EXISTS)
}

func GetTickCount() uint64 {
	r, _, _ := pGetTickCount64.Call()
	return uint64(r)
}
