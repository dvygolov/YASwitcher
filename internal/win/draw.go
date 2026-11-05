//go:build windows

package win

import (
	"syscall"
	"unsafe"
)

// Drawing helpers. Colour arguments are 0xRRGGBB; converted to GDI COLORREF.

func colorRef(rgb uint32) uintptr {
	r := (rgb >> 16) & 0xFF
	g := (rgb >> 8) & 0xFF
	b := rgb & 0xFF
	return uintptr(r | (g << 8) | (b << 16))
}

// Font is a GDI font selected into a device context for drawing.
type Font struct{ h HFONT }

// NewFont creates a Segoe UI font with the given pixel height, weight
// (FW_NORMAL / FW_SEMIBOLD / FW_BOLD) and italic flag. nil on failure.
func NewFont(height, weight int32, italic bool) *Font {
	it := uintptr(0)
	if italic {
		it = 1
	}
	face := UTF16Ptr("Segoe UI")
	r, _, _ := pCreateFontW.Call(
		^uintptr(uint32(height)-1), // negative height = character height
		0, 0, 0,
		uintptr(weight),
		it, 0, 0,
		DEFAULT_CHARSET,
		0, 0, CLEARTYPE_QUALITY, 0,
		uintptr(unsafe.Pointer(face)),
	)
	if r == 0 {
		return nil
	}
	return &Font{h: HFONT(r)}
}

func (f *Font) Valid() bool { return f != nil && f.h != 0 }

func (f *Font) Delete() {
	if f != nil && f.h != 0 {
		pDeleteObject.Call(uintptr(f.h))
		f.h = 0
	}
}

func SelectObject(dc HDC, obj uintptr) uintptr {
	r, _, _ := pSelectObject.Call(uintptr(dc), obj)
	return r
}

// SelectFont selects f into dc, returning the previous font handle.
func SelectFont(dc HDC, f *Font) HFONT {
	if f == nil || f.h == 0 {
		return 0
	}
	r, _, _ := pSelectObject.Call(uintptr(dc), uintptr(f.h))
	return HFONT(r)
}

// FillSolid fills r with an opaque solid colour.
func FillSolid(dc HDC, r RECT, rgb uint32) {
	br, _, _ := pCreateSolidBrush.Call(colorRef(rgb))
	if br == 0 {
		return
	}
	pFillRect.Call(uintptr(dc), uintptr(unsafe.Pointer(&r)), br)
	pDeleteObject.Call(br)
}

// FillSolidRound fills a rounded rectangle with a solid colour.
func FillSolidRound(dc HDC, r RECT, radius int32, rgb uint32) {
	br, _, _ := pCreateSolidBrush.Call(colorRef(rgb))
	if br == 0 {
		return
	}
	old, _, _ := pSelectObject.Call(uintptr(dc), br)
	nullPen, _, _ := pGetStockObject.Call(8) // NULL_PEN
	oldPen, _, _ := pSelectObject.Call(uintptr(dc), nullPen)
	pRoundRect.Call(uintptr(dc), uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(radius*2), uintptr(radius*2))
	pSelectObject.Call(uintptr(dc), oldPen)
	pSelectObject.Call(uintptr(dc), old)
	pDeleteObject.Call(br)
}

// StrokeRound draws the outline of a rounded rectangle.
func StrokeRound(dc HDC, r RECT, radius, width int32, rgb uint32) {
	if width < 1 {
		width = 1
	}
	pen, _, _ := pCreatePen.Call(PS_SOLID, uintptr(width), colorRef(rgb))
	if pen == 0 {
		return
	}
	old, _, _ := pSelectObject.Call(uintptr(dc), pen)
	nullBrush, _, _ := pGetStockObject.Call(5) // NULL_BRUSH (HOLLOW_BRUSH)
	oldBr, _, _ := pSelectObject.Call(uintptr(dc), nullBrush)
	pRoundRect.Call(uintptr(dc), uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(radius*2), uintptr(radius*2))
	pSelectObject.Call(uintptr(dc), oldBr)
	pSelectObject.Call(uintptr(dc), old)
	pDeleteObject.Call(pen)
}

// Line draws a straight line.
func Line(dc HDC, x1, y1, x2, y2, width int32, rgb uint32) {
	if width < 1 {
		width = 1
	}
	pen, _, _ := pCreatePen.Call(PS_SOLID, uintptr(width), colorRef(rgb))
	if pen == 0 {
		return
	}
	old, _, _ := pSelectObject.Call(uintptr(dc), pen)
	pMoveToEx.Call(uintptr(dc), uintptr(x1), uintptr(y1), 0)
	pLineTo.Call(uintptr(dc), uintptr(x2), uintptr(y2))
	pSelectObject.Call(uintptr(dc), old)
	pDeleteObject.Call(pen)
}

// FillEllipse paints an ellipse.
func FillEllipse(dc HDC, r RECT, rgb uint32) {
	br, _, _ := pCreateSolidBrush.Call(colorCode(rgb))
	if br == 0 {
		return
	}
	old, _, _ := pSelectObject.Call(uintptr(dc), br)
	nullPen, _, _ := pGetStockObject.Call(8) // NULL_PEN
	oldPen, _, _ := pSelectObject.Call(uintptr(dc), nullPen)
	pEllipse.Call(uintptr(dc), uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom))
	pSelectObject.Call(uintptr(dc), oldPen)
	pSelectObject.Call(uintptr(dc), old)
	pDeleteObject.Call(br)
}

func colorCode(rgb uint32) uintptr { return colorRef(rgb) }

// DrawText draws text into r using align (DT_* flags), colour and font. The
// rect is adjusted by DrawText, so callers pass a copy when they need it back.
func DrawText(dc HDC, text string, r *RECT, align uint32, rgb uint32, f *Font) {
	old := SelectFont(dc, f)
	pSetBkMode.Call(uintptr(dc), TRANSPARENT)
	pSetTextColor.Call(uintptr(dc), colorRef(rgb))
	pDrawTextW.Call(uintptr(dc),
		uintptr(unsafe.Pointer(UTF16Ptr(text))),
		^uintptr(0),
		uintptr(unsafe.Pointer(r)),
		uintptr(align),
	)
	if old != 0 {
		pSelectObject.Call(uintptr(dc), uintptr(old))
	}
}

// TextWidth measures the pixel width of text with the given font.
func TextWidth(dc HDC, text string, f *Font) int32 {
	old := SelectFont(dc, f)
	var sz SIZE
	pGetTextExtentPoint.Call(uintptr(dc),
		uintptr(unsafe.Pointer(UTF16Ptr(text))),
		uintptr(len(utf16Runes(text))),
		uintptr(unsafe.Pointer(&sz)),
	)
	if old != 0 {
		pSelectObject.Call(uintptr(dc), uintptr(old))
	}
	return sz.CX
}

func utf16Runes(s string) []uint16 {
	if s == "" {
		return []uint16{0}
	}
	u, err := syscall.UTF16FromString(s)
	if err != nil {
		return []uint16{0}
	}
	return u
}

// DrawIcon draws an HICON into the rectangle r on dc. Returns false when the
// icon handle is zero. Icons are drawn at their native size and scaled to r.
func DrawIcon(dc HDC, r RECT, icon HICON) bool {
	if icon == 0 {
		return false
	}
	w := uintptr(r.W())
	h := uintptr(r.H())
	rv, _, _ := pDrawIconEx.Call(uintptr(dc), uintptr(r.Left), uintptr(r.Top), uintptr(icon), w, h, 0, 0, DI_NORMAL)
	return rv != 0
}

// Buffer is a memory DC with a bitmap, used for flicker-free double buffering.
type Buffer struct {
	DC      HDC
	bitmap  HBITMAP
	oldBits uintptr
	W, H    int32
}

// NewBuffer creates a screen-compatible buffer of the given size. The caller
// must call Release when finished.
func NewBuffer(w, h int32) *Buffer {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	screen, _, _ := pGetDC.Call(0)
	dc, _, _ := pCreateCompatibleDC.Call(screen)
	bm, _, _ := pCreateCompatibleBmp.Call(screen, uintptr(w), uintptr(h))
	old, _, _ := pSelectObject.Call(dc, bm)
	pReleaseDC.Call(0, screen)
	return &Buffer{DC: HDC(dc), bitmap: HBITMAP(bm), oldBits: old, W: w, H: h}
}

// Release frees the buffer's GDI objects.
func (b *Buffer) Release() {
	if b == nil {
		return
	}
	pSelectObject.Call(uintptr(b.DC), b.oldBits)
	pDeleteObject.Call(uintptr(b.bitmap))
	pDeleteDC.Call(uintptr(b.DC))
	b.bitmap = 0
	b.DC = 0
}

// BitBlt copies the buffer onto a destination DC at (x, y).
func (b *Buffer) Blit(dst HDC, x, y int32) {
	pBitBlt.Call(uintptr(dst), uintptr(x), uintptr(y), uintptr(b.W), uintptr(b.H), uintptr(b.DC), 0, 0, SRCCOPY)
}

// SetTextColor sets the text colour on dc and returns the previous colour.
func SetTextColor(dc HDC, rgb uint32) uintptr {
	r, _, _ := pSetTextColor.Call(uintptr(dc), colorRef(rgb))
	return r
}

// SetBkMode sets the background mode on dc (TRANSPARENT / OPAQUE).
func SetBkMode(dc HDC, mode int32) {
	pSetBkMode.Call(uintptr(dc), uintptr(mode))
}

// TextOut draws a UTF-16 string at (x, y) on dc with the currently selected font.
func TextOut(dc HDC, x, y int32, u []uint16, n int) bool {
	if len(u) == 0 {
		return false
	}
	r, _, _ := pTextOutW.Call(uintptr(dc), uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&u[0])), uintptr(n))
	return r != 0
}
