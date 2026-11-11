package main

import (
	"strings"
	"syscall"

	"github.com/dvygolov/yaswitcher/internal/win"
)

// Layout constants in 96-DPI design units. Everything is scaled by the DPI of
// the monitor the overlay opens on.
const (
	panelW      = 520
	headerH     = 56
	rowH        = 52
	padding     = 12
	iconSize    = 32
	maxPanelH   = 560
	searchFontH = 20
	rowFontH    = 17
	smallFontH  = 13
	radius      = 10
)

// Theme holds the colours for the overlay. Two presets: dark and light, close
// to Switcheroo's dark/light skins but flat.
type Theme struct {
	Name        string
	Bg          uint32
	BgAlt       uint32 // search bar / footer strip
	Border      uint32
	Text        uint32
	SubText     uint32
	Accent      uint32
	SelBg       uint32
	SelText     uint32
	SelSub      uint32
	Highlight   uint32 // match highlight in text
	MonitorText uint32
}

func themeFor(dark bool) Theme {
	if dark {
		return Theme{
			Name:        "dark",
			Bg:          0x1E1E22,
			BgAlt:       0x2A2A30,
			Border:      0x3A3A44,
			Text:        0xE8E8EC,
			SubText:     0x9A9AA6,
			Accent:      0x4C9AFF,
			SelBg:       0x2F4B7C,
			SelText:     0xFFFFFF,
			SelSub:      0xC7D6F2,
			Highlight:   0x6FB4FF,
			MonitorText: 0x8FD18F,
		}
	}
	return Theme{
		Name:        "light",
		Bg:          0xFAFAFC,
		BgAlt:       0xEFEFF3,
		Border:      0xCFCFD8,
		Text:        0x1B1B20,
		SubText:     0x6B6B76,
		Accent:      0x1F6FEB,
		SelBg:       0x1F6FEB,
		SelText:     0xFFFFFF,
		SelSub:      0xD6E4FB,
		Highlight:   0x0B4FB0,
		MonitorText: 0x1F7A1F,
	}
}

// Row is one drawable window entry.
type Row struct {
	Win   win.WindowInfo
	Score MatchResult
}

// Overlay is the switcher panel window. It is created once and shown/hidden on
// demand, so the cost of opening it is just a paint.
type Overlay struct {
	hwnd win.HWND
	dc   win.HDC // cached DC for the double buffer
	app  *App

	theme   Theme
	dpi     uint32
	rows    []Row
	sel     int
	query   string
	scroll  int
	visible bool

	fontSearch *win.Font
	fontRow    *win.Font
	fontSub    *win.Font
	fontSmall  *win.Font

	// geometry computed on each open
	scale        int32
	panel        win.RECT
	header       win.RECT
	searchBox    win.RECT
	listRect     win.RECT
	footerRect   win.RECT
	rowHeight    int32
	visibleRows  int
	monitorLabel string

	// icons cache keyed by process+path
	icons map[string]win.HICON
}

// NewOverlay creates the hidden panel window. The window class is registered in
// app.Start; here we only bind a per-instance handler.
func NewOverlay(app *App) *Overlay {
	o := &Overlay{
		app:   app,
		theme: themeFor(app.cfg.DarkTheme),
		icons: make(map[string]win.HICON),
	}
	param := win.NewHandler(o.wndProc)
	o.hwnd = win.CreateWindowEx(
		win.WS_EX_TOPMOST|win.WS_EX_TOOLWINDOW,
		className, appTitle,
		win.WS_POPUP,
		0, 0, 1, 1, 0, 0, param,
	)
	return o
}

// Reload refreshes the window list from the current desktop state.
func (o *Overlay) Reload() {
	self := win.SelfPID()
	list := win.ListWindows(self)
	o.rows = o.rows[:0]
	o.query = ""
	o.sel = 0
	o.scroll = 0
	for _, w := range list {
		o.rows = append(o.rows, Row{Win: w})
	}
	o.filter()
}

// filter re-scores rows for the current query and keeps matching ones sorted.
func (o *Overlay) filter() {
	if o.query == "" {
		// original Z order
		o.rows = o.rows[:0]
		for _, w := range win.ListWindows(win.SelfPID()) {
			o.rows = append(o.rows, Row{Win: w})
		}
		o.sel = 0
		o.scroll = 0
		return
	}
	scored := make([]Row, 0, len(o.rows))
	for _, r := range o.rows {
		m := scoreTitle(o.query, r.Win.Title, r.Win.Process)
		if !m.Matched {
			continue
		}
		r.Score = m
		scored = append(scored, r)
	}
	// simple insertion sort by descending score; lists are small
	for i := 1; i < len(scored); i++ {
		for j := i; j > 0 && scored[j].Score.Score > scored[j-1].Score.Score; j-- {
			scored[j], scored[j-1] = scored[j-1], scored[j]
		}
	}
	if len(scored) > o.app.cfg.MaxResults {
		scored = scored[:o.app.cfg.MaxResults]
	}
	o.rows = scored
	o.sel = 0
	o.scroll = 0
}

// Show opens the overlay on the monitor where the cursor is.
func (o *Overlay) Show() {
	if o.hwnd == 0 {
		return
	}
	pt := win.GetCursorPos()
	mon := win.MonitorAt(pt)
	o.dpi = win.DpiForPoint(pt)
	o.scale = win.Scale(1, o.dpi)
	if o.scale < 1 {
		o.scale = 1
	}
	o.theme = themeFor(o.app.cfg.DarkTheme)
	o.visible = true
	o.Reload()
	o.ensureFonts()
	o.layout(mon)
	// position the window first, then everything else is client-relative
	o.hwnd.SetPos(win.HWND_TOPMOST, o.panel.Left, o.panel.Top, o.panel.W(), o.panel.H(), win.SWP_SHOWWINDOW)
	o.hwnd.Show(win.SW_SHOW)
	// Force foreground on the next message-loop iteration so we never block the
	// (thread-affine) low-level keyboard hook inside Show().
	win.PostMessage(o.hwnd, win.WM_APP+40, 0, 0)
	o.hwnd.Invalidate(true)
}

// Hide closes the overlay without activating another window.
func (o *Overlay) Hide() {
	if o.hwnd != 0 {
		o.hwnd.Show(win.SW_HIDE)
	}
	o.visible = false
}

func (o *Overlay) IsVisible() bool { return o.visible }

func (o *Overlay) ensureFonts() {
	o.fontSearch = win.NewFont(win.Scale(searchFontH, o.dpi), 400, false)
	o.fontRow = win.NewFont(win.Scale(rowFontH, o.dpi), 400, false)
	o.fontSub = win.NewFont(win.Scale(smallFontH, o.dpi), 400, false)
	o.fontSmall = win.NewFont(win.Scale(smallFontH, o.dpi), 400, false)
}

// layout computes the panel rectangle in screen coordinates and the sub-rects
// in client coordinates (the window origin is the panel's top-left).
func (o *Overlay) layout(mon win.Monitor) {
	w := win.Scale(panelW, o.dpi)
	rows := len(o.rows)
	if rows > o.app.cfg.MaxResults {
		rows = o.app.cfg.MaxResults
	}
	if rows == 0 {
		rows = 1
	}
	rowH := win.Scale(rowH, o.dpi)
	h := win.Scale(headerH, o.dpi) + rowH*int32(min(rows, 14)) + win.Scale(padding*2, o.dpi)
	maxH := win.Scale(maxPanelH, o.dpi)
	if h > maxH {
		h = maxH
	}
	work := mon.Work
	if work.W() == 0 {
		work = win.VirtualScreen()
	}
	x := work.Left + (work.W()-w)/2
	y := work.Top + (work.H()-h)/2 - win.Scale(60, o.dpi)
	if y < work.Top+win.Scale(40, o.dpi) {
		y = work.Top + win.Scale(40, o.dpi)
	}
	// screen rectangle of the whole panel
	o.panel = win.RECT{Left: x, Top: y, Right: x + w, Bottom: y + h}

	pad := win.Scale(padding, o.dpi)
	// client rects: 0,0 is the window's top-left corner
	o.header = win.RECT{Left: pad, Top: pad, Right: w - pad, Bottom: pad + win.Scale(headerH, o.dpi)}
	o.searchBox = win.RECT{Left: o.header.Left, Top: o.header.Top + win.Scale(4, o.dpi),
		Right: o.header.Right, Bottom: o.header.Top + win.Scale(headerH-12, o.dpi)}
	o.rowHeight = rowH
	o.listRect = win.RECT{Left: o.header.Left, Top: o.header.Bottom, Right: o.header.Right, Bottom: h - pad - win.Scale(26, o.dpi)}
	o.visibleRows = int(o.listRect.H() / rowH)
	if o.visibleRows < 1 {
		o.visibleRows = 1
	}
	o.monitorLabel = monitorLabel(mon)
}

// monitorLabel describes the monitor the overlay opens on.
func monitorLabel(m win.Monitor) string {
	all := win.Monitors()
	idx := 1
	for i, mm := range all {
		if mm.Handle == m.Handle {
			idx = i + 1
			break
		}
	}
	w := m.Work.W()
	h := m.Work.H()
	return "Monitor " + itoa(int64(idx)) + " · " + itoa(int64(w)) + "×" + itoa(int64(h))
}

// ---------------------------------------------------------------------------
// painting
// ---------------------------------------------------------------------------

func (o *Overlay) paint(h win.HWND) {
	var ps win.PAINTSTRUCT
	hdc := win.BeginPaint(h, &ps)
	defer win.EndPaint(h, &ps)

	var rc win.RECT
	// use the full window rect for the double buffer
	rc = h.ClientRect()
	if rc.W() <= 0 || rc.H() <= 0 {
		return
	}
	buf := win.NewBuffer(rc.W(), rc.H())
	if buf == nil || buf.DC == 0 {
		return
	}
	defer buf.Release()
	o.draw(buf.DC, rc)
	buf.Blit(hdc, 0, 0)
}

// draw renders the whole panel into dc (window client coordinates).
func (o *Overlay) draw(dc win.HDC, client win.RECT) {
	t := o.theme

	// background with rounded corners
	win.FillSolid(dc, client, t.Bg)

	pad := win.Scale(padding, o.dpi)

	// header / search bar
	win.FillSolidRound(dc, o.searchBox, win.Scale(8, o.dpi), t.BgAlt)
	query := o.query
	if query == "" {
		query = "Type to search windows…"
	}
	win.DrawText(dc, query,
		&win.RECT{Left: o.searchBox.Left + win.Scale(14, o.dpi), Top: o.searchBox.Top,
			Right: o.searchBox.Right - win.Scale(120, o.dpi), Bottom: o.searchBox.Bottom},
		win.DT_LEFT|win.DT_VCENTER|win.DT_SINGLELINE|win.DT_END_ELLIPSIS|win.DT_NOPREFIX,
		t.SubText, o.fontSearch)

	if o.app.cfg.ShowMonitor {
		mr := win.RECT{Left: o.searchBox.Right - win.Scale(210, o.dpi), Top: o.searchBox.Top,
			Right: o.searchBox.Right - win.Scale(12, o.dpi), Bottom: o.searchBox.Bottom}
		win.DrawText(dc, o.monitorLabel, &mr,
			win.DT_RIGHT|win.DT_VCENTER|win.DT_SINGLELINE|win.DT_END_ELLIPSIS|win.DT_NOPREFIX,
			t.MonitorText, o.fontSmall)
	}

	// rows
	start := o.scroll
	end := start + o.visibleRows
	if end > len(o.rows) {
		end = len(o.rows)
	}
	y := o.listRect.Top
	for i := start; i < end; i++ {
		r := o.rows[i]
		rowRect := win.RECT{Left: o.listRect.Left, Top: y, Right: o.listRect.Right, Bottom: y + o.rowHeight}
		sel := i == o.sel
		if sel {
			win.FillSolidRound(dc, rowRect, win.Scale(8, o.dpi), t.SelBg)
		}
		o.drawRow(dc, rowRect, r, sel)
		y += o.rowHeight
	}
	if len(o.rows) == 0 {
		empty := win.RECT{Left: o.listRect.Left, Top: o.listRect.Top, Right: o.listRect.Right, Bottom: o.listRect.Top + o.rowHeight}
		win.DrawText(dc, "No matching windows", &empty, win.DT_LEFT|win.DT_VCENTER|win.DT_SINGLELINE|win.DT_NOPREFIX, t.SubText, o.fontRow)
	}

	// scrollbar: a thin track with a thumb, only when the list overflows.
	if len(o.rows) > o.visibleRows {
		trackX := o.listRect.Right - win.Scale(3, o.dpi)
		track := win.RECT{Left: trackX, Top: o.listRect.Top, Right: trackX + win.Scale(3, o.dpi), Bottom: o.listRect.Bottom}
		win.FillSolidRound(dc, track, 2, t.BgAlt)
		thumbH := int32(float32(o.visibleRows) / float32(len(o.rows)) * float32(track.H()))
		if thumbH < win.Scale(24, o.dpi) {
			thumbH = win.Scale(24, o.dpi)
		}
		maxScroll := len(o.rows) - o.visibleRows
		frac := 0.0
		if maxScroll > 0 {
			frac = float64(o.scroll) / float64(maxScroll)
		}
		thumbTop := track.Top + int32(frac*float64(track.H()-thumbH))
		thumb := win.RECT{Left: trackX, Top: thumbTop, Right: trackX + win.Scale(3, o.dpi), Bottom: thumbTop + thumbH}
		win.FillSolidRound(dc, thumb, 2, t.SubText)
	}

	// footer hint
	fh := win.Scale(18, o.dpi)
	fr := win.RECT{Left: o.listRect.Left + win.Scale(4, o.dpi), Top: client.Bottom - pad - fh,
		Right: o.listRect.Right, Bottom: client.Bottom - pad}
	hint := "Enter to switch · Esc to close"
	if o.app.cfg.AltTabMode {
		hint = "Hold Alt · Tab / Shift+Tab · release to switch"
	}
	win.DrawText(dc, hint, &fr, win.DT_LEFT|win.DT_VCENTER|win.DT_SINGLELINE|win.DT_NOPREFIX, t.SubText, o.fontSmall)

	// outer border, drawn last so it sits on top of the row fills
	edge := win.RECT{Left: client.Left + win.Scale(1, o.dpi), Top: client.Top + win.Scale(1, o.dpi),
		Right: client.Right - win.Scale(1, o.dpi), Bottom: client.Bottom - win.Scale(1, o.dpi)}
	win.StrokeRound(dc, edge, win.Scale(radius, o.dpi), win.Scale(1, o.dpi), t.Border)
}

// drawRow draws one window entry: icon, title (with match highlight), process.
func (o *Overlay) drawRow(dc win.HDC, row win.RECT, r Row, sel bool) {
	t := o.theme
	pad := win.Scale(padding, o.dpi)
	iconR := win.RECT{Left: row.Left + pad, Top: row.Top + (row.H()-win.Scale(iconSize, o.dpi))/2,
		Right: row.Left + pad + win.Scale(iconSize, o.dpi), Bottom: row.Top + (row.H()+win.Scale(iconSize, o.dpi))/2}
	icon := o.iconFor(r.Win)
	if icon == 0 {
		// placeholder: rounded square with the first letter
		win.FillSolidRound(dc, iconR, win.Scale(6, o.dpi), t.BgAlt)
	}
	win.DrawIcon(dc, iconR, icon)

	textX := iconR.Right + win.Scale(12, o.dpi)
	title := r.Win.Title
	process := r.Win.Process
	if process != "" {
		process = " · " + process
	}
	fg := t.Text
	sub := t.SubText
	if sel {
		fg = t.SelText
		sub = t.SelSub
	}
	// title line
	titleR := win.RECT{Left: textX, Top: row.Top + win.Scale(7, o.dpi),
		Right: row.Right - pad, Bottom: row.Top + row.H()/2 + win.Scale(2, o.dpi)}
	o.drawHighlighted(dc, title, r.Score, titleR, fg, t.Highlight, sel)
	// process line
	procR := win.RECT{Left: textX, Top: row.Top + row.H()/2 - win.Scale(2, o.dpi),
		Right: row.Right - pad, Bottom: row.Bottom - win.Scale(7, o.dpi)}
	win.DrawText(dc, process, &procR, win.DT_LEFT|win.DT_VCENTER|win.DT_SINGLELINE|win.DT_END_ELLIPSIS|win.DT_NOPREFIX, sub, o.fontSub)
}

// drawHighlighted draws title with the matched byte ranges in the accent colour.
// This is approximate (byte ranges vs UTF-16 glyphs) but good enough for titles.
func (o *Overlay) drawHighlighted(dc win.HDC, title string, m MatchResult, r win.RECT, fg, hl uint32, sel bool) {
	if len(m.Ranges) == 0 {
		win.DrawText(dc, title, &r, win.DT_LEFT|win.DT_VCENTER|win.DT_SINGLELINE|win.DT_END_ELLIPSIS|win.DT_NOPREFIX, fg, o.fontRow)
		return
	}
	// Draw in segments using TextOut. Baseline maths for Segoe UI: the font's
	// height is pixel size; text top = vcenter - height*0.62.
	x := r.Left
	maxX := r.Right
	fh := win.Scale(rowFontH, o.dpi)
	top := r.Top + (r.H()-fh)/2 - win.Scale(2, o.dpi)
	low := strings.ToLower(title)
	last := 0
	for _, rng := range m.Ranges {
		if rng[0] > len(low) || rng[1] > len(low) || rng[0] < 0 {
			continue
		}
		if rng[0] > last {
			x = o.drawSegment(dc, title[last:rng[0]], x, top, maxX, fg)
		}
		x = o.drawSegment(dc, title[rng[0]:rng[1]], x, top, maxX, hl)
		last = rng[1]
	}
	if last < len(title) {
		o.drawSegment(dc, title[last:], x, top, maxX, fg)
	}
}

// drawSegment draws s at (x, top) with the row font and returns the new x.
func (o *Overlay) drawSegment(dc win.HDC, s string, x, top, maxX int32, rgb uint32) int32 {
	if s == "" {
		return x
	}
	if maxX-x < 4 {
		return maxX
	}
	u := toUTF16(s)
	if len(u) == 0 {
		return x
	}
	old := win.SelectFont(dc, o.fontRow)
	win.SetTextColor(dc, rgb)
	win.SetBkMode(dc, win.TRANSPARENT)
	w := win.TextWidth(dc, s, o.fontRow)
	if x+w > maxX {
		w = maxX - x
	}
	win.TextOut(dc, x, top, u, len(u))
	_ = old
	return x + w
}

// iconFor returns the icon for a window, caching by process path.
func (o *Overlay) iconFor(w win.WindowInfo) win.HICON {
	key := w.Path
	if key == "" {
		key = w.Process
	}
	if key == "" {
		return 0
	}
	if ic, ok := o.icons[key]; ok {
		return ic
	}
	ic := win.ExeIcon(w.Path)
	if ic == 0 {
		ic = win.ShellIcon(w.Path)
	}
	o.icons[key] = ic
	return ic
}

// ---------------------------------------------------------------------------
// window procedure
// ---------------------------------------------------------------------------

func (o *Overlay) wndProc(h win.HWND, msg uint32, wp, lp uintptr) uintptr {
	switch msg {
	case win.WM_APP + 40:
		win.SetForegroundWindow(o.hwnd)
		return 0
	case win.WM_PAINT:
		o.paint(h)
		return 0
	case win.WM_CHAR:
		ch := uint32(wp)
		if ch >= 32 || ch == '\t' {
			if ch == '\t' {
				return 0
			}
			o.query += string(rune(ch))
			o.filter()
			o.layout(win.MonitorFromWindow(h))
			o.hwnd.Invalidate(true)
			return 0
		}
		return 0
	case win.WM_SYSKEYDOWN:
		// Alt is held and a key was pressed: Alt+Space must cycle the list, and
		// Alt+Tab steps it. Handling this here (the overlay has focus) avoids
		// depending on the global hook staying alive while the panel is open.
		if wp == win.VK_SPACE {
			o.app.cycleStep()
			return 0
		}
		if wp == win.VK_TAB {
			if o.app.altShiftHeld() {
				o.moveSel(-1)
			} else {
				o.moveSel(1)
			}
			return 0
		}
		return win.DefWindowProc(h, msg, wp, lp)
	case win.WM_SYSKEYUP:
		// Alt released while the overlay is open: commit if we were cycling.
		if wp == win.VK_MENU || wp == win.VK_LMENU || wp == win.VK_RMENU {
			o.app.holdAltReleased()
			return 0
		}
		// Space released: clear the press latch so the next press cycles again.
		if wp == win.VK_SPACE {
			o.app.releaseHotkeyKey()
			return 0
		}
		return win.DefWindowProc(h, msg, wp, lp)
	case win.WM_KEYUP:
		// Alt's own release can arrive as a plain WM_KEYUP for VK_MENU.
		if wp == win.VK_MENU || wp == win.VK_LMENU || wp == win.VK_RMENU {
			o.app.holdAltReleased()
			return 0
		}
		if wp == win.VK_SPACE {
			o.app.releaseHotkeyKey()
			return 0
		}
		return win.DefWindowProc(h, msg, wp, lp)
	case win.WM_KEYDOWN:
		switch wp {
		case win.VK_ESCAPE:
			o.app.cancelOverlay()
			return 0
		case win.VK_BACK:
			if len(o.query) > 0 {
				r := []rune(o.query)
				o.query = string(r[:len(r)-1])
				o.filter()
				o.layout(win.MonitorFromWindow(h))
				o.hwnd.Invalidate(true)
			}
			return 0
		case win.VK_UP:
			o.moveSel(-1)
			return 0
		case win.VK_DOWN:
			o.moveSel(1)
			return 0
		case win.VK_RETURN:
			o.activateSelected()
			return 0
		}
		return 0
	case win.WM_MOUSEMOVE:
		pt := win.PointFromLParam(lp)
		if i := o.hitTest(pt); i >= 0 && i != o.sel {
			o.sel = i
			o.hwnd.Invalidate(true)
		}
		return 0
	case win.WM_LBUTTONDOWN:
		pt := win.PointFromLParam(lp)
		if i := o.hitTest(pt); i >= 0 {
			o.sel = i
			o.activateSelected()
		}
		return 0
	case win.WM_MOUSEWHEEL:
		// high word of wParam is the signed wheel delta; one notch is 120.
		delta := int16((wp >> 16) & 0xFFFF)
		if delta > 0 {
			o.moveSel(-1)
		} else if delta < 0 {
			o.moveSel(1)
		}
		return 0
	case win.WM_KILLFOCUS:
		// Keep the overlay alive while the user holds Alt and cycles; only a
		// real focus loss (click elsewhere, no gesture) dismisses it.
		if !o.app.inHoldGesture() {
			o.app.cancelOverlay()
		}
		return 0
	case win.WM_CLOSE:
		o.app.cancelOverlay()
		return 0
	case win.WM_DESTROY:
		o.app.quit()
		return 0
	}
	return win.DefWindowProc(h, msg, wp, lp)
}

func (o *Overlay) moveSel(delta int) {
	if len(o.rows) == 0 {
		return
	}
	o.sel += delta
	if o.sel < 0 {
		o.sel = 0
	}
	if o.sel >= len(o.rows) {
		o.sel = len(o.rows) - 1
	}
	if o.sel < o.scroll {
		o.scroll = o.sel
	}
	if o.sel >= o.scroll+o.visibleRows {
		o.scroll = o.sel - o.visibleRows + 1
	}
	o.hwnd.Invalidate(true)
}

func (o *Overlay) hitTest(pt win.POINT) int {
	// translate to client coordinates; the list rect is in client space already
	if pt.X < o.listRect.Left || pt.X >= o.listRect.Right || pt.Y < o.listRect.Top || pt.Y >= o.listRect.Bottom {
		return -1
	}
	i := o.scroll + int((pt.Y-o.listRect.Top)/o.rowHeight)
	if i >= 0 && i < len(o.rows) {
		return i
	}
	return -1
}

func (o *Overlay) activateSelected() {
	if o.sel < 0 || o.sel >= len(o.rows) {
		o.app.cancelOverlay()
		return
	}
	w := o.rows[o.sel].Win
	o.app.switchTo(w)
}

// helpers ------------------------------------------------------------------

func toUTF16(s string) []uint16 {
	if s == "" {
		return nil
	}
	u, err := syscall.UTF16FromString(s)
	if err != nil {
		return nil
	}
	return u[:len(u)-1]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [24]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// setSel moves the selection to an absolute index (clamped) and keeps it in view.
func (o *Overlay) setSel(i int) {
	if len(o.rows) == 0 {
		return
	}
	if i < 0 {
		i = 0
	}
	if i >= len(o.rows) {
		i = len(o.rows) - 1
	}
	o.sel = i
	if o.sel < o.scroll {
		o.scroll = o.sel
	}
	if o.sel >= o.scroll+o.visibleRows {
		o.scroll = o.sel - o.visibleRows + 1
	}
	o.hwnd.Invalidate(true)
}

// RowCount returns how many rows are currently listed.
func (o *Overlay) RowCount() int { return len(o.rows) }
