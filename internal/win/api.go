//go:build windows

// Package win is the minimal, dependency-free set of Win32 bindings Switcher
// needs. Everything is loaded lazily from system DLLs through
// syscall.NewLazyDLL: no CGO, no third party packages, tiny binary.
package win

import "syscall"

var (
	dllUser32   = syscall.NewLazyDLL("user32.dll")
	dllGdi32    = syscall.NewLazyDLL("gdi32.dll")
	dllKernel32 = syscall.NewLazyDLL("kernel32.dll")
	dllShell32  = syscall.NewLazyDLL("shell32.dll")
	dllDwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	dllAdvapi32 = syscall.NewLazyDLL("advapi32.dll")
	dllPsapi    = syscall.NewLazyDLL("psapi.dll")
	dllShcore   = syscall.NewLazyDLL("shcore.dll")
	dllMsimg32  = syscall.NewLazyDLL("msimg32.dll")
)

// kernel32
var (
	pGetModuleHandleW      = dllKernel32.NewProc("GetModuleHandleW")
	pCreateMutexW          = dllKernel32.NewProc("CreateMutexW")
	pSleep                 = dllKernel32.NewProc("Sleep")
	pGetTickCount64        = dllKernel32.NewProc("GetTickCount64")
	pQueryFullProcessImage = dllKernel32.NewProc("QueryFullProcessImageNameW")
	pOpenProcess           = dllKernel32.NewProc("OpenProcess")
	pCloseHandle           = dllKernel32.NewProc("CloseHandle")
	pGlobalAlloc           = dllKernel32.NewProc("GlobalAlloc")
	pGlobalFree            = dllKernel32.NewProc("GlobalFree")
	pGlobalLock            = dllKernel32.NewProc("GlobalLock")
	pGlobalUnlock          = dllKernel32.NewProc("GlobalUnlock")
	pGetCurrentProcessId   = dllKernel32.NewProc("GetCurrentProcessId")
	pGetCurrentThreadId    = dllKernel32.NewProc("GetCurrentThreadId")
	pGetModuleFileNameW    = dllKernel32.NewProc("GetModuleFileNameW")
)

// user32: window management
var (
	pRegisterClassExW         = dllUser32.NewProc("RegisterClassExW")
	pCreateWindowExW          = dllUser32.NewProc("CreateWindowExW")
	pDestroyWindow            = dllUser32.NewProc("DestroyWindow")
	pDefWindowProcW           = dllUser32.NewProc("DefWindowProcW")
	pShowWindow               = dllUser32.NewProc("ShowWindow")
	pUpdateWindow             = dllUser32.NewProc("UpdateWindow")
	pSetWindowPos             = dllUser32.NewProc("SetWindowPos")
	pGetMessageW              = dllUser32.NewProc("GetMessageW")
	pPeekMessageW             = dllUser32.NewProc("PeekMessageW")
	pTranslateMessage         = dllUser32.NewProc("TranslateMessage")
	pDispatchMessageW         = dllUser32.NewProc("DispatchMessageW")
	pPostQuitMessage          = dllUser32.NewProc("PostQuitMessage")
	pPostMessageW             = dllUser32.NewProc("PostMessageW")
	pSendMessageW             = dllUser32.NewProc("SendMessageW")
	pSendMessageTimeoutW      = dllUser32.NewProc("SendMessageTimeoutW")
	pSetWindowLongPtrW        = dllUser32.NewProc("SetWindowLongPtrW")
	pGetWindowLongPtrW        = dllUser32.NewProc("GetWindowLongPtrW")
	pGetWindowRect            = dllUser32.NewProc("GetWindowRect")
	pGetClientRect            = dllUser32.NewProc("GetClientRect")
	pGetClassNameW            = dllUser32.NewProc("GetClassNameW")
	pGetWindowTextW           = dllUser32.NewProc("GetWindowTextW")
	pGetWindowTextLengthW     = dllUser32.NewProc("GetWindowTextLengthW")
	pEnumWindows              = dllUser32.NewProc("EnumWindows")
	pIsWindowVisible          = dllUser32.NewProc("IsWindowVisible")
	pIsWindow                 = dllUser32.NewProc("IsWindow")
	pIsWindowEnabled          = dllUser32.NewProc("IsWindowEnabled")
	pIsIconic                 = dllUser32.NewProc("IsIconic")
	pGetWindow                = dllUser32.NewProc("GetWindow")
	pGetAncestor              = dllUser32.NewProc("GetAncestor")
	pGetLastActivePopup       = dllUser32.NewProc("GetLastActivePopup")
	pGetPropW                 = dllUser32.NewProc("GetPropW")
	pGetForegroundWindow      = dllUser32.NewProc("GetForegroundWindow")
	pSetForegroundWindow      = dllUser32.NewProc("SetForegroundWindow")
	pBringWindowToTop         = dllUser32.NewProc("BringWindowToTop")
	pShowWindowAsync          = dllUser32.NewProc("ShowWindowAsync")
	pAttachThreadInput        = dllUser32.NewProc("AttachThreadInput")
	pGetWindowThreadProcessId = dllUser32.NewProc("GetWindowThreadProcessId")
	pGetClassLongPtrW         = dllUser32.NewProc("GetClassLongPtrW")
)

// user32: painting & input
var (
	pRegisterHotKey        = dllUser32.NewProc("RegisterHotKey")
	pUnregisterHotKey      = dllUser32.NewProc("UnregisterHotKey")
	pGetDC                 = dllUser32.NewProc("GetDC")
	pReleaseDC             = dllUser32.NewProc("ReleaseDC")
	pInvalidateRect        = dllUser32.NewProc("InvalidateRect")
	pBeginPaint            = dllUser32.NewProc("BeginPaint")
	pEndPaint              = dllUser32.NewProc("EndPaint")
	pFillRect              = dllUser32.NewProc("FillRect")
	pDrawTextW             = dllUser32.NewProc("DrawTextW")
	pSetTimer              = dllUser32.NewProc("SetTimer")
	pKillTimer             = dllUser32.NewProc("KillTimer")
	pMessageBoxW           = dllUser32.NewProc("MessageBoxW")
	pCreatePopupMenu       = dllUser32.NewProc("CreatePopupMenu")
	pAppendMenuW           = dllUser32.NewProc("AppendMenuW")
	pDestroyMenu           = dllUser32.NewProc("DestroyMenu")
	pTrackPopupMenu        = dllUser32.NewProc("TrackPopupMenu")
	pCreateIconIndirect    = dllUser32.NewProc("CreateIconIndirect")
	pDestroyIcon           = dllUser32.NewProc("DestroyIcon")
	pGetIconInfo           = dllUser32.NewProc("GetIconInfo")
	pDrawIconEx            = dllUser32.NewProc("DrawIconEx")
	pCopyIcon              = dllUser32.NewProc("CopyIcon")
	pSetProcessDPIAware    = dllUser32.NewProc("SetProcessDPIAware")
	pSetProcessDpiCtx      = dllUser32.NewProc("SetProcessDpiAwarenessContext")
	pGetDpiForWindow       = dllUser32.NewProc("GetDpiForWindow")
	pGetDpiForSystem       = dllUser32.NewProc("GetDpiForSystem")
	pClientToScreen        = dllUser32.NewProc("ClientToScreen")
	pScreenToClient        = dllUser32.NewProc("ScreenToClient")
	pGetSystemMetrics      = dllUser32.NewProc("GetSystemMetrics")
	pGetCursorPos          = dllUser32.NewProc("GetCursorPos")
	pLoadCursorW           = dllUser32.NewProc("LoadCursorW")
	pSetCursor             = dllUser32.NewProc("SetCursor")
	pSetWindowRgn          = dllUser32.NewProc("SetWindowRgn")
	pSetLayeredWindowAttrs = dllUser32.NewProc("SetLayeredWindowAttributes")
	pSetWindowsHookExW     = dllUser32.NewProc("SetWindowsHookExW")
	pCallNextHookEx        = dllUser32.NewProc("CallNextHookEx")
	pUnhookWindowsHookEx   = dllUser32.NewProc("UnhookWindowsHookEx")
	pGetKeyState           = dllUser32.NewProc("GetKeyState")
)

// user32: monitors
var (
	pMonitorFromPoint    = dllUser32.NewProc("MonitorFromPoint")
	pMonitorFromWindow   = dllUser32.NewProc("MonitorFromWindow")
	pGetMonitorInfoW     = dllUser32.NewProc("GetMonitorInfoW")
	pEnumDisplayMonitors = dllUser32.NewProc("EnumDisplayMonitors")
)

// gdi32
var (
	pCreateCompatibleDC  = dllGdi32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBmp = dllGdi32.NewProc("CreateCompatibleBitmap")
	pCreateDIBSection    = dllGdi32.NewProc("CreateDIBSection")
	pDeleteDC            = dllGdi32.NewProc("DeleteDC")
	pDeleteObject        = dllGdi32.NewProc("DeleteObject")
	pSelectObject        = dllGdi32.NewProc("SelectObject")
	pCreateSolidBrush    = dllGdi32.NewProc("CreateSolidBrush")
	pCreatePen           = dllGdi32.NewProc("CreatePen")
	pSetBkMode           = dllGdi32.NewProc("SetBkMode")
	pSetBkColor          = dllGdi32.NewProc("SetBkColor")
	pSetTextColor        = dllGdi32.NewProc("SetTextColor")
	pGetTextExtentPoint  = dllGdi32.NewProc("GetTextExtentPoint32W")
	pGetDeviceCaps       = dllGdi32.NewProc("GetDeviceCaps")
	pCreateFontW         = dllGdi32.NewProc("CreateFontW")
	pGetStockObject      = dllGdi32.NewProc("GetStockObject")
	pBitBlt              = dllGdi32.NewProc("BitBlt")
	pStretchBlt          = dllGdi32.NewProc("StretchBlt")
	pRectangle           = dllGdi32.NewProc("Rectangle")
	pRoundRect           = dllGdi32.NewProc("RoundRect")
	pMoveToEx            = dllGdi32.NewProc("MoveToEx")
	pLineTo              = dllGdi32.NewProc("LineTo")
	pEllipse             = dllGdi32.NewProc("Ellipse")
	pTextOutW            = dllGdi32.NewProc("TextOutW")
	pCreateRoundRectRgn  = dllGdi32.NewProc("CreateRoundRectRgn")
)

// msimg32
var (
	pAlphaBlend = dllMsimg32.NewProc("AlphaBlend")
)

// dwmapi
var (
	pDwmSetWindowAttribute = dllDwmapi.NewProc("DwmSetWindowAttribute")
	pDwmGetWindowAttribute = dllDwmapi.NewProc("DwmGetWindowAttribute")
)

// psapi
var (
	pGetModuleFileNameExW = dllPsapi.NewProc("GetModuleFileNameExW")
)

// shcore
var (
	pGetDpiForMonitor = dllShcore.NewProc("GetDpiForMonitor")
)

// shell32
var (
	pShellNotifyIconW      = dllShell32.NewProc("Shell_NotifyIconW")
	pShellExecuteW         = dllShell32.NewProc("ShellExecuteW")
	pExtractIconExW        = dllShell32.NewProc("ExtractIconExW")
	pSHGetFileInfoW        = dllShell32.NewProc("SHGetFileInfoW")
	pExtractAssociatedIcon = dllShell32.NewProc("ExtractAssociatedIconW")
)
