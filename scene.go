// SPDX-License-Identifier: BSD-3-Clause
//
// The View layer. It builds the widgets from go-widgets/toolkit, lays
// them out with box layouts, and binds every one of them to the ViewModel
// (viewmodel.go) through go-widgets/mvvmtk. It NEVER mutates a widget's
// state field directly — the binders own that — so the whole file passes
// go-widgets/mvvmlint. This is the pattern the template exists to seed.
//
// scene.go carries NO js/wasm build tag (main.go does), so a native
// `go test` exercises the whole View — construct, bind, event dispatch,
// layout and draw — against a plain RGBA buffer via painter. See
// scene_test.go.

package main

import (
	"github.com/go-widgets/mvvmtk"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// Surface dimensions. Fixed (not content-derived): the ListBox scrolls
// internally, so the canvas stays a constant size.
const (
	surfaceW = 480
	surfaceH = 420
)

// Layout constants: one outer margin, a uniform inter-row gap, the fixed
// heights of the search box and the control strip, and the fixed width of
// the Clear button. The ListBox flexes to fill the rest.
const (
	margin  = 10
	gap     = 8
	searchH = 26
	ctrlH   = 28
	clearW  = 96

	// dropDownRowH is the pixel height of one row in the category
	// DropDown's popover, used to map a popover click back to an option
	// index (mirrors the toolkit's PopoverBounds row step).
	dropDownRowH = 18
)

// state is the View: the widgets, their box layout, the bound ViewModel,
// and the tiny bit of host bookkeeping (hit-test order, keyboard focus, a
// repaint-needed flag the bindings raise through invalidate).
type state struct {
	w, h  int
	theme *toolkit.Theme
	vm    *viewModel

	// Widgets. A SearchEntry filters by name; a DropDown filters by
	// category; a Button clears both; a ListBox shows the derived rows; a
	// Label shows the count.
	search *toolkit.SearchEntry
	cat    *toolkit.DropDown
	clear  *toolkit.Button
	list   *toolkit.ListBox
	status *toolkit.Label

	// Box layout: root stacks search / controls / list / status vertically;
	// controls packs the category combo (flex) and the Clear button (fixed).
	root     *toolkit.VBox
	controls *toolkit.HBox

	// Live hit-test list (draw order) + the keyboard-focused widget (the
	// SearchEntry once clicked).
	clickables []toolkit.Widget
	keyTarget  toolkit.Widget

	// Pointer-drag capture: the widget grabbed on press, with its surface
	// bounds, so a following EventMouseDrag / EventMouseUp routes back to it in
	// its OWN local coordinates even as the pointer leaves its bounds. This is
	// what lets a scrollbar thumb (or a Paned divider, a Scale handle, ...) be
	// dragged: the browser driver forwards mousemove/mouseup here, and without
	// this capture those events would be thrown away. Generic — no widget-type
	// special-casing — so a copied app inherits working drags for free.
	dragTarget toolkit.Widget
	dragBounds toolkit.Rect

	// dirty is raised by the binding invalidate hook whenever a
	// ViewModel-originated change reaches a widget; the driver repaints when
	// it is set. Bindings are retained in unbind for a clean teardown.
	dirty  bool
	unbind []func()
}

// newState builds the ViewModel, constructs every widget, binds them to
// the VM through mvvmtk (the ONLY place widget state is touched), lays
// them out and returns the ready scene. The second parameter mirrors the
// gallery/registry-viewer newState(w, h) shape; the height is fixed.
func newState(w, _ int) *state {
	// Anti-aliased, shaped go-opentype text so labels render crisply. Done
	// before any layout measures text; on the impossible error the toolkit
	// keeps its bitmap default and the scene still renders.
	_ = toolkit.UseOpenTypeText()

	vm := newViewModel(sampleItems())
	s := &state{
		w:     w,
		h:     surfaceH,
		theme: toolkit.DefaultLight(),
		vm:    vm,
	}
	invalidate := func() { s.dirty = true }

	// --- widgets (state seeded + kept in sync ONLY via the binders) -------
	s.search = toolkit.NewSearchEntry("")
	s.cat = toolkit.NewDropDown(vm.categories, 0)
	s.clear = toolkit.NewButton("Clear", nil)
	s.list = toolkit.NewListBox(nil)
	s.status = toolkit.NewLabel("")

	s.unbind = append(s.unbind,
		mvvmtk.BindText(s.search, vm.Query, invalidate),
		mvvmtk.BindSelectedIndex(s.cat, vm.Category, invalidate),
		mvvmtk.BindCommand(s.clear, vm.Clear, invalidate),
		mvvmtk.BindListItems(s.list, vm.Visible, func(it item) string { return it.Name }, invalidate),
		mvvmtk.BindLabel(s.status, vm.Status, invalidate),
	)

	// --- box layout -------------------------------------------------------
	s.controls = toolkit.NewHBox()
	s.controls.Spacing = gap
	s.controls.AddFlex(s.cat, 1)
	s.controls.AddFixed(s.clear, clearW)

	s.root = toolkit.NewVBox()
	s.root.Spacing = gap
	s.root.AddFixed(s.search, searchH)
	s.root.AddFixed(s.controls, ctrlH)
	s.root.AddFlex(s.list, 1)
	s.root.AddFixed(s.status, toolkit.StatusbarH)
	s.root.SetBounds(toolkit.Rect{X: margin, Y: margin, W: w - 2*margin, H: surfaceH - 2*margin})

	// Hit-test order = visual/z order: controls first, then the list.
	s.clickables = []toolkit.Widget{s.search, s.cat, s.clear, s.list}
	return s
}

// draw paints the whole scene onto buf (an RGBA row-major slice) via a
// PixelPainter: background, then the box layout, then the category
// popover (which floats above the list when open).
func (s *state) draw(buf []byte) {
	fillBG(buf, s.theme.Background)
	p := painter.NewPixelPainter(buf, s.w, s.h)
	s.root.Draw(p, s.theme)
	if s.cat.Open {
		lb := toolkit.NewListBox(s.cat.Options)
		lb.SetBounds(s.cat.PopoverBounds())
		lb.Draw(p, s.theme)
	}
}

// handleClick dispatches a click at surface (x, y). An open popover floats
// above everything: a click inside selects that option, a click outside
// dismisses it. Otherwise the click routes to whichever widget contains it
// (focusing the SearchEntry, clearing focus elsewhere). Selecting a
// popover row goes through DropDown.Select, whose composed OnSelect (wired
// by BindSelectedIndex) updates the ViewModel — the app never writes the
// widget's Selected field itself.
func (s *state) handleClick(x, y int) bool {
	if s.cat.Open {
		pb := s.cat.PopoverBounds()
		if inside(x, y, pb) {
			s.cat.Select((y - pb.Y) / dropDownRowH)
		} else {
			s.cat.Open = false
		}
		return true
	}

	for _, w := range s.clickables {
		r := w.Bounds()
		if inside(x, y, r) {
			// The hit widget also becomes the drag-capture target, so a
			// following EventMouseDrag / EventMouseUp routes back to it (its
			// scrollbar thumb, a Paned divider, ...). See handleDrag.
			s.dragTarget, s.dragBounds = w, r
			s.keyTarget = w
			s.search.Focused = (w == toolkit.Widget(s.search))
			w.OnEvent(local(ev(toolkit.EventClick, x, y), r))
			return true
		}
	}
	s.dragTarget = nil
	s.keyTarget = nil
	s.search.Focused = false
	return true
}

// handleScroll routes a wheel scroll (Delta in ROWS, positive = down) to
// whichever widget contains (x, y) via the root's coordinate routing. The
// ListBox consumes the EventScroll and scrolls itself, clamping at both
// ends — the app does NO scroll math. Always repaints.
func (s *state) handleScroll(x, y, delta int) bool {
	e := ev(toolkit.EventScroll, x, y)
	e.Delta = delta
	s.root.OnEvent(local(e, s.root.Bounds()))
	return true
}

// handleChar routes a printable character to the focused widget (the
// SearchEntry) as an EventChar. The widget's OnChange — composed by
// BindText — flows the edit into the ViewModel's Query observable, which
// re-derives the visible list. Reports whether a target consumed it.
func (s *state) handleChar(ch string) bool {
	if s.keyTarget == nil {
		return false
	}
	s.keyTarget.OnEvent(toolkit.Event{Kind: toolkit.EventChar, Code: ch})
	return true
}

// handleKeyDown routes a named key (Backspace, …) to the focused widget as
// an EventKeyDown. Reports whether a target consumed it.
func (s *state) handleKeyDown(code string) bool {
	if s.keyTarget == nil {
		return false
	}
	s.keyTarget.OnEvent(toolkit.Event{Kind: toolkit.EventKeyDown, Code: code})
	return true
}

// handleMove drives pointer motion: a captured drag first (dragging a
// scrollbar thumb, a Paned divider, ...), else a no-op hover. The browser
// driver forwards every mousemove here; returning true asks for a repaint.
func (s *state) handleMove(x, y int) bool {
	return s.handleDrag(x, y)
}

// handleDrag routes a pointer move (button held) to the widget grabbed on
// press, as an EventMouseDrag in that widget's LOCAL coordinates (surface
// minus the captured bounds) — which is where the toolkit hit-tests its
// scrollbar thumb / divider. It reports whether a captured widget consumed
// it. No-op (false) when nothing is captured, so a plain hover changes
// nothing. Generic: it never inspects the widget's concrete type.
func (s *state) handleDrag(x, y int) bool {
	if s.dragTarget == nil {
		return false
	}
	s.dragTarget.OnEvent(toolkit.Event{Kind: toolkit.EventMouseDrag, X: x - s.dragBounds.X, Y: y - s.dragBounds.Y})
	return true
}

// handleRelease routes a button release to the captured widget as an
// EventMouseUp (its local coordinates), then drops the capture. Reports
// whether a captured widget consumed it.
func (s *state) handleRelease(x, y int) bool {
	if s.dragTarget == nil {
		return false
	}
	s.dragTarget.OnEvent(toolkit.Event{Kind: toolkit.EventMouseUp, X: x - s.dragBounds.X, Y: y - s.dragBounds.Y})
	s.dragTarget = nil
	return true
}

// --- helpers ---------------------------------------------------------------

// ev builds an EventClick/EventScroll-shaped Event at surface (x, y).
func ev(kind toolkit.EventKind, x, y int) toolkit.Event {
	return toolkit.Event{Kind: kind, X: x, Y: y}
}

// fillBG paints the whole RGBA buffer with c (opaque background).
func fillBG(buf []byte, c toolkit.RGBA) {
	for i := 0; i+3 < len(buf); i += 4 {
		buf[i], buf[i+1], buf[i+2], buf[i+3] = c.R, c.G, c.B, c.A
	}
}

// inside reports whether (x, y) falls in r (half-open on the far edges).
func inside(x, y int, r toolkit.Rect) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// local re-bases a surface-space event into r's widget-local coordinates.
func local(e toolkit.Event, r toolkit.Rect) toolkit.Event {
	e.X -= r.X
	e.Y -= r.Y
	return e
}
