// SPDX-License-Identifier: BSD-3-Clause
//
// ViewModel unit tests. The VM imports no widget package, so these run
// with no painter or canvas: pure state assertions on the observables, the
// derived list and the command.

package main

import "testing"

// names returns the projected Name of every row currently in Visible.
func names(vm *viewModel) []string {
	out := make([]string, 0, vm.Visible.Len())
	for i := 0; i < vm.Visible.Len(); i++ {
		out = append(out, vm.Visible.At(i).Name)
	}
	return out
}

func TestNewViewModelInitialState(t *testing.T) {
	vm := newViewModel(sampleItems())

	// categories = "All" + sorted distinct of the sample set.
	want := []string{"All", "render", "state", "text", "widgets"}
	if len(vm.categories) != len(want) {
		t.Fatalf("categories = %v, want %v", vm.categories, want)
	}
	for i, c := range want {
		if vm.categories[i] != c {
			t.Fatalf("categories[%d] = %q, want %q", i, vm.categories[i], c)
		}
	}

	// No filter yet: every row is visible.
	if got := vm.Visible.Len(); got != len(sampleItems()) {
		t.Fatalf("Visible.Len() = %d, want %d", got, len(sampleItems()))
	}
	if vm.Status.Get() != "24 of 24 shown" {
		t.Fatalf("Status = %q, want %q", vm.Status.Get(), "24 of 24 shown")
	}
	// The Clear command is disabled while no filter is active.
	if vm.Clear.CanExecute() {
		t.Fatal("Clear should not be executable with no active filter")
	}
}

func TestQueryFilter(t *testing.T) {
	vm := newViewModel(sampleItems())

	vm.Query.Set("t") // case-insensitive substring on Name
	got := names(vm)
	// Every visible row must contain "t"; at least one row must have been
	// filtered out (the nomatch branch of recompute).
	if len(got) == 0 || len(got) == len(sampleItems()) {
		t.Fatalf("query %q narrowed to %v (expected a strict subset)", "t", got)
	}
	for _, n := range got {
		if !contains(n, "t") {
			t.Fatalf("row %q does not contain the query", n)
		}
	}
	if vm.Status.Get() == "24 of 24 shown" {
		t.Fatalf("Status should reflect the narrowed set, got %q", vm.Status.Get())
	}
	// A filter is now active, so Clear is executable.
	if !vm.Clear.CanExecute() {
		t.Fatal("Clear should be executable once a query is set")
	}

	// A query matching nothing empties the list.
	vm.Query.Set("zzz-nomatch")
	if vm.Visible.Len() != 0 {
		t.Fatalf("no-match query left %d rows", vm.Visible.Len())
	}
}

func TestCategoryFilter(t *testing.T) {
	vm := newViewModel(sampleItems())

	vm.Category.Set(4) // "widgets"
	if vm.selectedCategory() != "widgets" {
		t.Fatalf("selectedCategory = %q, want %q", vm.selectedCategory(), "widgets")
	}
	for i := 0; i < vm.Visible.Len(); i++ {
		if vm.Visible.At(i).Category != "widgets" {
			t.Fatalf("row %q is not in the widgets category", vm.Visible.At(i).Name)
		}
	}
	if vm.Visible.Len() != 3 { // toolkit, tui, tray
		t.Fatalf("widgets category has %d rows, want 3", vm.Visible.Len())
	}

	// An out-of-range index means "no constraint" (defensive branch).
	vm.Category.Set(len(vm.categories))
	if vm.selectedCategory() != "" {
		t.Fatalf("out-of-range category should yield no constraint, got %q", vm.selectedCategory())
	}
	if vm.Visible.Len() != len(sampleItems()) {
		t.Fatalf("out-of-range category should show all rows, got %d", vm.Visible.Len())
	}
}

func TestCombinedFilterAndClear(t *testing.T) {
	vm := newViewModel(sampleItems())

	vm.Query.Set("t")
	vm.Category.Set(4) // widgets AND name contains "t"
	for i := 0; i < vm.Visible.Len(); i++ {
		it := vm.Visible.At(i)
		if it.Category != "widgets" || !contains(it.Name, "t") {
			t.Fatalf("row %q fails the combined AND filter", it.Name)
		}
	}

	// Clear resets both filters and restores the full list.
	vm.Clear.Execute()
	if vm.Query.Get() != "" || vm.Category.Get() != 0 {
		t.Fatalf("Clear left Query=%q Category=%d", vm.Query.Get(), vm.Category.Get())
	}
	if vm.Visible.Len() != len(sampleItems()) {
		t.Fatalf("Clear left %d rows, want all", vm.Visible.Len())
	}
	if vm.Clear.CanExecute() {
		t.Fatal("Clear should be disabled again after clearing")
	}
}

// contains is a tiny substring helper (case-sensitive; test data is lower).
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
