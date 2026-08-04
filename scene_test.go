// SPDX-License-Identifier: BSD-3-Clause
//
// View (scene) tests. main.go carries the js && wasm tag and drops out on
// native, so these exercise the whole View natively: construction +
// binding, event dispatch (click / char / key / scroll), the popover and
// draw — against a plain RGBA buffer. Together with viewmodel_test.go they
// cover 100% of the tagless statements.

package main

import (
	"testing"

	"github.com/go-widgets/toolkit"
)

// newBuf allocates an RGBA surface buffer of the scene's fixed size.
func newBuf() []byte { return make([]byte, 4*surfaceW*surfaceH) }

// center returns the mid-point of a widget's bounds.
func center(w toolkit.Widget) (int, int) {
	r := w.Bounds()
	return r.X + r.W/2, r.Y + r.H/2
}

func TestNewStateBindsAndDraws(t *testing.T) {
	s := newState(surfaceW, surfaceH)

	// The ListBox starts populated from the ViewModel via BindListItems.
	if len(s.list.Items) != len(sampleItems()) {
		t.Fatalf("list has %d items, want %d", len(s.list.Items), len(sampleItems()))
	}
	// The status Label starts populated from the ViewModel via BindLabel.
	if s.status.Text != "12 of 12 shown" {
		t.Fatalf("status = %q, want %q", s.status.Text, "12 of 12 shown")
	}
	// draw must not panic and must paint the background (top-left pixel).
	buf := newBuf()
	s.draw(buf)
	bg := s.theme.Background
	if buf[0] != bg.R || buf[1] != bg.G || buf[2] != bg.B || buf[3] != bg.A {
		t.Fatalf("top-left pixel = %v, want background %v", buf[:4], bg)
	}
}

func TestSearchTypingFlowsThroughViewModel(t *testing.T) {
	s := newState(surfaceW, surfaceH)

	// Click the search box: it gains focus and becomes the key target.
	if !s.handleClick(center(s.search)) {
		t.Fatal("handleClick(search) should report a repaint")
	}
	if !s.search.Focused || s.keyTarget != toolkit.Widget(s.search) {
		t.Fatal("clicking the search box should focus it")
	}

	// Typing a character routes to the SearchEntry, whose composed OnChange
	// (from BindText) sets the Query observable, which re-derives the list.
	s.dirty = false
	if !s.handleChar("t") {
		t.Fatal("handleChar should be consumed by the focused entry")
	}
	if s.vm.Query.Get() != "t" {
		t.Fatalf("Query = %q, want %q", s.vm.Query.Get(), "t")
	}
	if !s.dirty {
		t.Fatal("a ViewModel-driven list change should raise the repaint flag")
	}
	if len(s.list.Items) == 0 || len(s.list.Items) == len(sampleItems()) {
		t.Fatalf("the list should have narrowed, got %d items", len(s.list.Items))
	}

	// Backspace deletes the character, restoring the full list.
	if !s.handleKeyDown("Backspace") {
		t.Fatal("handleKeyDown should be consumed by the focused entry")
	}
	if s.vm.Query.Get() != "" {
		t.Fatalf("Query after Backspace = %q, want empty", s.vm.Query.Get())
	}
	if len(s.list.Items) != len(sampleItems()) {
		t.Fatalf("list should be full again, got %d", len(s.list.Items))
	}
}

func TestKeyEventsWithoutFocusAreIgnored(t *testing.T) {
	s := newState(surfaceW, surfaceH)

	// A click on dead space (outside every widget) clears the key target.
	if !s.handleClick(0, 0) {
		t.Fatal("handleClick on dead space still repaints")
	}
	if s.keyTarget != nil || s.search.Focused {
		t.Fatal("clicking dead space should clear focus")
	}
	// With no key target, char + key events are not consumed.
	if s.handleChar("x") {
		t.Fatal("handleChar with no focus should return false")
	}
	if s.handleKeyDown("Enter") {
		t.Fatal("handleKeyDown with no focus should return false")
	}
}

func TestDropdownSelectViaPopover(t *testing.T) {
	s := newState(surfaceW, surfaceH)

	// Clicking the dropdown opens its popover (and does NOT focus the search
	// box — the false branch of the focus assignment).
	s.handleClick(center(s.cat))
	if !s.cat.Open {
		t.Fatal("clicking the dropdown should open the popover")
	}
	if s.search.Focused {
		t.Fatal("clicking the dropdown must not focus the search box")
	}

	// draw with the popover open exercises the overlay branch.
	s.draw(newBuf())

	// Click option index 2 ("state") inside the popover: DropDown.Select
	// fires the composed OnSelect (from BindSelectedIndex), updating the VM.
	pb := s.cat.PopoverBounds()
	s.handleClick(pb.X+4, pb.Y+2*dropDownRowH+3)
	if s.cat.Open {
		t.Fatal("selecting an option should close the popover")
	}
	if s.vm.Category.Get() != 2 || s.vm.selectedCategory() != "state" {
		t.Fatalf("Category = %d (%q), want 2 (state)", s.vm.Category.Get(), s.vm.selectedCategory())
	}
	for _, it := range s.list.Items {
		if it != "mvvm" && it != "mvvmtk" && it != "mvvmlint" {
			t.Fatalf("state filter leaked non-state row %q", it)
		}
	}
}

func TestDropdownClickOutsideDismisses(t *testing.T) {
	s := newState(surfaceW, surfaceH)
	s.handleClick(center(s.cat)) // open
	if !s.cat.Open {
		t.Fatal("popover should be open")
	}
	// A click outside the popover bounds dismisses it without selecting.
	s.handleClick(0, 0)
	if s.cat.Open {
		t.Fatal("clicking outside should close the popover")
	}
	if s.vm.Category.Get() != 0 {
		t.Fatalf("dismissing must not change the selection, got %d", s.vm.Category.Get())
	}
}

func TestClearButtonResetsFilters(t *testing.T) {
	s := newState(surfaceW, surfaceH)

	// Establish an active filter via the VM (as a user edit would).
	s.vm.Query.Set("t")
	if !s.vm.Clear.CanExecute() {
		t.Fatal("Clear should be enabled with an active filter")
	}
	// Click the Clear button: its composed OnClick (from BindCommand) runs
	// the Command, which resets both filters.
	s.handleClick(center(s.clear))
	if s.vm.Query.Get() != "" || s.vm.Category.Get() != 0 {
		t.Fatalf("Clear left Query=%q Category=%d", s.vm.Query.Get(), s.vm.Category.Get())
	}
	if len(s.list.Items) != len(sampleItems()) {
		t.Fatalf("list should be full after Clear, got %d", len(s.list.Items))
	}
}

func TestScrollAndNoopHandlers(t *testing.T) {
	s := newState(surfaceW, surfaceH)
	// A wheel scroll over the list is routed to the toolkit, which does the
	// scroll math itself; the handler always requests a repaint.
	x, y := center(s.list)
	if !s.handleScroll(x, y, 3) {
		t.Fatal("handleScroll should always repaint")
	}
	// Hover + release are no-ops in this scene.
	if s.handleMove(x, y) {
		t.Fatal("handleMove should be a no-op")
	}
	if s.handleRelease(x, y) {
		t.Fatal("handleRelease should be a no-op")
	}
}

func TestNativeBanner(t *testing.T) {
	if banner() == "" {
		t.Fatal("banner should be non-empty")
	}
	main() // native stub: prints the banner, must not panic
}
