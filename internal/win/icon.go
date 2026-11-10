//go:build windows

package win

import (
	"unsafe"
)

// Window icon lookup: prefer the live window icon (WM_GETICON / class icon),
// fall back to the icon embedded in the executable.

const (
	ICON_SMALL_ID  = 0
	ICON_BIG_ID    = 1
	ICON_SMALL2_ID = 2
)

// WindowIcon returns a small icon for the window, or 0. The caller owns nothing:
// the icon handle is owned by the window or was created here and cached by the
// caller via EnsureIcon.
func WindowIcon(h HWND) HICON {
	// WM_GETICON with a timeout so a hung app cannot stall the switcher.
	if v, ok := SendMessageTimeout(h, WM_GETICON, ICON_SMALL2_ID, 0, SMTO_ABORTIFHUNG, 80); ok && v != 0 {
		if c, _, _ := pCopyIcon.Call(v); c != 0 {
			return HICON(c)
		}
	}
	if v, ok := SendMessageTimeout(h, WM_GETICON, ICON_BIG_ID, 0, SMTO_ABORTIFHUNG, 80); ok && v != 0 {
		if c, _, _ := pCopyIcon.Call(v); c != 0 {
			return HICON(c)
		}
	}
	if v, ok := SendMessageTimeout(h, WM_GETICON, ICON_SMALL_ID, 0, SMTO_ABORTIFHUNG, 80); ok && v != 0 {
		if c, _, _ := pCopyIcon.Call(v); c != 0 {
			return HICON(c)
		}
	}
	if v, _, _ := pGetClassLongPtrW.Call(uintptr(h), ^uintptr(33)); v != 0 { // GCLP_HICONSM = -34
		if c, _, _ := pCopyIcon.Call(v); c != 0 {
			return HICON(c)
		}
	}
	if v, _, _ := pGetClassLongPtrW.Call(uintptr(h), ^uintptr(13)); v != 0 { // GCLP_HICON = -14
		if c, _, _ := pCopyIcon.Call(v); c != 0 {
			return HICON(c)
		}
	}
	return 0
}

// ExeIcon extracts a small icon from an executable file.
func ExeIcon(path string) HICON {
	if path == "" {
		return 0
	}
	var large, small HICON
	r, _, _ := pExtractIconExW.Call(
		uintptr(unsafe.Pointer(UTF16Ptr(path))),
		0,
		uintptr(unsafe.Pointer(&large)),
		uintptr(unsafe.Pointer(&small)),
		1,
	)
	if r == 0 {
		return 0
	}
	if large != 0 {
		pDestroyIcon.Call(uintptr(large))
	}
	return small
}

// DestroyIcon frees an icon created by CopyIcon / ExtractIcon.
func (h HICON) Destroy() {
	if h != 0 {
		pDestroyIcon.Call(uintptr(h))
	}
}

// ShellIcon returns the small shell icon for a file path (generic fallback).
func ShellIcon(path string) HICON {
	type shfileinfo struct {
		HIcon         HICON
		IIcon         int32
		DwAttributes  uint32
		SzDisplayName [260]uint16
		SzTypeName    [80]uint16
	}
	var sfi shfileinfo
	r, _, _ := pSHGetFileInfoW.Call(
		uintptr(unsafe.Pointer(UTF16Ptr(path))),
		0,
		uintptr(unsafe.Pointer(&sfi)),
		unsafe.Sizeof(sfi),
		0x000000101, // SHGFI_ICON | SHGFI_SMALLICON
	)
	if r != 0 && sfi.HIcon != 0 {
		return sfi.HIcon
	}
	return 0
}

// LoadAppIcon loads the icon resource #1 from our own executable module, which
// is where go-winres places it. Returns 0 when the build has no resources.
func LoadAppIcon() HICON {
	h, _, _ := pLoadImageW.Call(uintptr(GetModuleHandle()), 1, IMAGE_ICON,
		0, 0, LR_DEFAULTSIZE)
	return HICON(h)
}
