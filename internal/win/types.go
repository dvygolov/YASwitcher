//go:build windows

package win

import (
	"syscall"
	"unsafe"
)

// ---------------- basic types ----------------

type HWND uintptr
type HDC uintptr
type HICON uintptr
type HMENU uintptr
type HBRUSH uintptr
type HGDIOBJ uintptr
type HFONT uintptr
type HBITMAP uintptr
type GLFW uintptr

type POINT struct{ X, Y int32 }

type RECT struct{ Left, Top, Right, Bottom int32 }

func (r RECT) W() int32 { return r.Right - r.Left }
func (r RECT) H() int32 { return r.Bottom - r.Top }

type SIZE struct{ CX, CY int32 }

type MSG struct {
	HWnd    HWND
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
	Private uint32
}

type MONITORINFO struct {
	CbSize    uint32
	RcMonitor RECT
	RcWork    RECT
	DwFlags   uint32
}

type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         HICON
	HCursor       uintptr
	HbrBackground HBRUSH
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       HICON
}

type PAINTSTRUCT struct {
	Hdc         HDC
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

// UTF16Ptr converts a Go string to a NUL terminated UTF-16 pointer.
func UTF16Ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return nil
	}
	return p
}

// UTF16ToString converts a NUL terminated UTF-16 buffer to a Go string.
func UTF16ToString(b []uint16) string {
	return syscall.UTF16ToString(b)
}

// CopyUTF16 copies s into a fixed size UTF-16 array, NUL terminated.
func CopyUTF16(dst []uint16, s string) {
	src, err := syscall.UTF16FromString(s)
	if err != nil {
		src = []uint16{0}
	}
	if len(src) >= len(dst) {
		if len(dst) == 0 {
			return
		}
		src = src[:len(dst)-1]
	}
	copy(dst, src)
	if len(dst) > 0 {
		dst[len(dst)-1] = 0
	}
}

// GoString copies a NUL terminated UTF-16 pointer to a Go string, bounded.
func GoString(p *uint16, max int) string {
	if p == nil {
		return ""
	}
	n := 0
	for n < max {
		if *(*uint16)(unsafe.Add(unsafe.Pointer(p), uintptr(n)*2)) == 0 {
			break
		}
		n++
	}
	return syscall.UTF16ToString(unsafe.Slice(p, n))
}

// ---------------- constants ----------------

const (
	WM_NULL          = 0x0000
	WM_DESTROY       = 0x0002
	WM_CLOSE         = 0x0010
	WM_PAINT         = 0x000F
	WM_QUIT          = 0x0012
	WM_KEYDOWN       = 0x0100
	WM_KEYUP         = 0x0101
	WM_CHAR          = 0x0102
	WM_SYSKEYDOWN    = 0x0104
	WM_SYSKEYUP      = 0x0105
	WM_MOUSEMOVE     = 0x0200
	WM_LBUTTONDOWN   = 0x0201
	WM_LBUTTONUP     = 0x0202
	WM_RBUTTONUP     = 0x0205
	WM_MOUSEWHEEL    = 0x020A
	WM_ACTIVATE      = 0x0006
	WM_ACTIVATEAPP   = 0x001C
	WM_SETFOCUS      = 0x0007
	WM_KILLFOCUS     = 0x0008
	WM_ERASEBKGND    = 0x0014
	WM_SIZE          = 0x0005
	WM_MOVE          = 0x0003
	WM_NCCREATE      = 0x0081
	WM_NCCALCSIZE    = 0x0083
	WM_TIMER         = 0x0113
	WM_HOTKEY        = 0x0312
	WM_GETDLGCODE    = 0x0087
	WM_SETCURSOR     = 0x0020
	WM_SYSCOMMAND    = 0x0112
	WM_COMMAND       = 0x0111
	WM_CONTEXTMENU   = 0x007B
	WM_LBUTTONDBLCLK = 0x0203
	WM_APP           = 0x8000
	WM_TRAYICON      = WM_APP + 1
	WM_NCHITTEST     = 0x0084
)

const (
	WS_OVERLAPPED  = 0x00000000
	WS_POPUP       = 0x80000000
	WS_CHILD       = 0x40000000
	WS_VISIBLE     = 0x10000000
	WS_CAPTION     = 0x00C00000
	WS_THICKFRAME  = 0x00040000
	WS_MINIMIZEBOX = 0x00020000
	WS_MAXIMIZEBOX = 0x00010000
	WS_SYSMENU     = 0x00080000
	WS_DISABLED    = 0x08000000
	WS_TABSTOP     = 0x00010000

	WS_EX_TOPMOST       = 0x00000008
	WS_EX_TRANSPARENT   = 0x00000020
	WS_EX_TOOLWINDOW    = 0x00000080
	WS_EX_APPWINDOW     = 0x00040000
	WS_EX_NOACTIVATE    = 0x08000000
	WS_EX_LAYERED       = 0x00080000
	WS_EX_TOPMOST2      = 0x00000008
	WS_EX_WINDOWEDGE    = 0x00000100
	WS_EX_ACCEPTFILES   = 0x00000010
	WS_EX_COMPOSITED    = 0x02000000
	WS_EX_DLGMODALFRAME = 0x00000001
)

const (
	SW_HIDE           = 0
	SW_SHOWNORMAL     = 1
	SW_SHOWNOACTIVATE = 4
	SW_SHOW           = 5
	SW_MINIMIZE       = 6
	SW_RESTORE        = 9
)

const (
	HWND_TOPMOST   = ^HWND(0)
	HWND_NOTOPMOST = ^HWND(1)
	HWND_TOP       = 0

	SWP_NOSIZE     = 0x0001
	SWP_NOMOVE     = 0x0002
	SWP_NOZORDER   = 0x0004
	SWP_NOACTIVATE = 0x0010
	SWP_SHOWWINDOW = 0x0040
)

const (
	GW_OWNER     = 4
	GA_ROOT      = 2
	GA_ROOTOWNER = 3
)

const (
	MOD_ALT      = 0x0001
	MOD_CONTROL  = 0x0002
	MOD_SHIFT    = 0x0004
	MOD_WIN      = 0x0008
	MOD_NOREPEAT = 0x4000
)

const (
	VK_LBUTTON    = 0x01
	VK_RBUTTON    = 0x02
	VK_CANCEL     = 0x03
	VK_MBUTTON    = 0x04
	VK_BACK       = 0x08
	VK_TAB        = 0x09
	VK_RETURN     = 0x0D
	VK_SHIFT      = 0x10
	VK_CONTROL    = 0x11
	VK_MENU       = 0x12
	VK_PAUSE      = 0x13
	VK_CAPITAL    = 0x14
	VK_ESCAPE     = 0x1B
	VK_SPACE      = 0x20
	VK_PRIOR      = 0x21
	VK_NEXT       = 0x22
	VK_END        = 0x23
	VK_HOME       = 0x24
	VK_LEFT       = 0x25
	VK_UP         = 0x26
	VK_RIGHT      = 0x27
	VK_DOWN       = 0x28
	VK_SNAPSHOT   = 0x2C
	VK_INSERT     = 0x2D
	VK_DELETE     = 0x2E
	VK_LWIN       = 0x5B
	VK_RWIN       = 0x5C
	VK_NUMPAD0    = 0x60
	VK_NUMPAD9    = 0x69
	VK_F1         = 0x70
	VK_F24        = 0x87
	VK_OEM_1      = 0xBA
	VK_OEM_PLUS   = 0xBB
	VK_OEM_COMMA  = 0xBC
	VK_OEM_MINUS  = 0xBD
	VK_OEM_PERIOD = 0xBE
	VK_OEM_2      = 0xBF
	VK_OEM_3      = 0xC0
	VK_OEM_4      = 0xDB
	VK_OEM_5      = 0xDC
	VK_OEM_6      = 0xDD
	VK_OEM_7      = 0xDE
	VK_OEM_8      = 0xDF
	VK_OEM_102    = 0xE2
	VK_LSHIFT     = 0xA0
	VK_RSHIFT     = 0xA1
	VK_LCONTROL   = 0xA2
	VK_RCONTROL   = 0xA3
	VK_LMENU      = 0xA4
	VK_RMENU      = 0xA5
)

const (
	SM_CXSCREEN        = 0
	SM_CYSCREEN        = 1
	SM_XVIRTUALSCREEN  = 76
	SM_YVIRTUALSCREEN  = 77
	SM_CXVIRTUALSCREEN = 78
	SM_CYVIRTUALSCREEN = 79
	SM_CMONITORS       = 80
)

const (
	MONITOR_DEFAULTTONULL    = 0
	MONITOR_DEFAULTTOPRIMARY = 1
	MONITOR_DEFAULTTONEAREST = 2
)

const (
	SWP_FRAMECHANGED = 0x0020
)

const (
	DWMWA_CLOAKED         = 14
	WS_EX_NOACTIVATE_FLAG = 0x08000000
)

const (
	DT_LEFT         = 0x00000000
	DT_CENTER       = 0x00000001
	DT_RIGHT        = 0x00000002
	DT_VCENTER      = 0x00000004
	DT_SINGLELINE   = 0x00000020
	DT_NOPREFIX     = 0x00000800
	DT_END_ELLIPSIS = 0x00008000
	DT_WORDBREAK    = 0x00000010
)

const (
	SRCCOPY     = 0x00CC0020
	TRANSPARENT = 1
	OPAQUE      = 2
	BLACKNESS   = 0x00000042
	WHITENESS   = 0x00FF0062
)

const (
	PS_SOLID               = 0
	FW_NORMAL              = 400
	FW_SEMIBOLD            = 600
	FW_BOLD                = 700
	DEFAULT_CHARSET        = 1
	CLEARTYPE_QUALITY      = 5
	ANTIALIASED_QUALITY    = 4
	NONANTIALIASED_QUALITY = 3
)

const (
	LOGPIXELSX = 88
)

const (
	MB_OK              = 0x00000000
	MB_OKCANCEL        = 0x00000001
	MB_YESNOCANCEL     = 0x00000003
	MB_ICONERROR       = 0x00000010
	MB_ICONQUESTION    = 0x00000020
	MB_ICONWARNING     = 0x00000030
	MB_ICONINFORMATION = 0x00000040
	MB_SETFOREGROUND   = 0x00010000
	MB_TOPMOST         = 0x00040000
)

const (
	MF_STRING       = 0x00000000
	MF_SEPARATOR    = 0x00000800
	MF_CHECKED      = 0x00000008
	MF_GRAYED       = 0x00000001
	MF_DISABLED     = 0x00000002
	MF_DEFAULT      = 0x00001000
	TPM_RIGHTBUTTON = 0x0002
	TPM_RETURNCMD   = 0x0100
	TPM_LEFTALIGN   = 0x0000
	TPM_BOTTOMALIGN = 0x0020
	TPM_NONOTIFY    = 0x0080
)

const (
	NIM_ADD        = 0x0
	NIM_MODIFY     = 0x1
	NIM_DELETE     = 0x2
	NIM_SETVERSION = 0x4
	NIF_MESSAGE    = 0x1
	NIF_ICON       = 0x2
	NIF_TIP        = 0x4
	NIF_INFO       = 0x10
	NIIF_NONE      = 0
	NIIF_INFO      = 1
	NIIF_WARNING   = 2
	NIIF_ERROR     = 3
)

const (
	HWND_BROADCAST = 0xFFFF
	HWND_MESSAGE   = ^HWND(2)
)

const (
	IMAGE_ICON       = 1
	LR_LOADFROMFILE  = 0x10
	LR_DEFAULTSIZE   = 0x40
	LR_SHARED        = 0x8000
	DI_NORMAL        = 0x0003
	GCLP_HICON       = -14
	GCLP_HICONSM     = -34
	ICON_SMALL       = 0
	ICON_BIG         = 1
	ICON_SMALL2      = 2
	WM_GETICON       = 0x007F
	SMTO_ABORTIFHUNG = 0x0002
	SMTO_BLOCK       = 0x0001
)

const (
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	PROCESS_QUERY_INFORMATION         = 0x0400
	PROCESS_VM_READ                   = 0x0010
)

const (
	IDC_ARROW = 32512
	IDC_IBEAM = 32513
	IDC_HAND  = 32649
)

const (
	ERROR_ALREADY_EXISTS = 183
)

// Helper: RECT from LParam packed into a POINT during WM_MOUSEMOVE.
func PointFromLParam(lp uintptr) POINT {
	return POINT{X: int32(int16(lp & 0xFFFF)), Y: int32(int16((lp >> 16) & 0xFFFF))}
}

// MakeIntResource returns the low word resource id pointer used by
// LoadCursor/LoadIcon when the icon is a built-in system resource.
func MakeIntResource(id uintptr) *uint16 { return (*uint16)(unsafe.Pointer(id)) }
